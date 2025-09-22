#!/usr/bin/env python3
"""
AutoMock TTL Cleanup Lambda Function
Automatically tears down ECS Fargate infrastructure when TTL expires
"""

import json
import boto3
import os
import logging
from datetime import datetime, timezone, timedelta
from typing import Dict, Any

# Configure logging
logger = logging.getLogger()
logger.setLevel(logging.INFO)

# Initialize AWS clients
ecs = boto3.client('ecs')
elbv2 = boto3.client('elbv2')
ec2 = boto3.client('ec2')
s3 = boto3.client('s3')
route53 = boto3.client('route53')
acm = boto3.client('acm')
sns = boto3.client('sns')

def handler(event: Dict[str, Any], context: Any) -> Dict[str, Any]:
    """
    Main Lambda handler for TTL cleanup
    """
    try:
        logger.info("TTL Cleanup triggered for ECS Fargate infrastructure")
        
        # Check if TTL has expired
        if not should_cleanup():
            logger.info("TTL not yet expired, skipping cleanup")
            return {'statusCode': 200, 'body': json.dumps({'message': 'TTL not expired'})}
        
        # Send notification before cleanup
        if os.environ.get('NOTIFICATION_EMAIL'):
            send_cleanup_notification()
        
        # Perform cleanup
        cleanup_results = perform_cleanup()
        
        # Send completion notification
        if os.environ.get('NOTIFICATION_EMAIL'):
            send_completion_notification(cleanup_results)
        
        logger.info("TTL cleanup completed successfully")
        return {
            'statusCode': 200,
            'body': json.dumps({
                'message': 'Cleanup completed',
                'results': cleanup_results
            })
        }
        
    except Exception as e:
        logger.error(f"TTL cleanup failed: {str(e)}")
        if os.environ.get('NOTIFICATION_EMAIL'):
            send_error_notification(str(e))
        return {'statusCode': 500, 'body': json.dumps({'error': str(e)})}

def should_cleanup() -> bool:
    """Check if infrastructure should be cleaned up based on TTL"""
    try:
        cluster_name = os.environ.get('CLUSTER_NAME')
        service_name = os.environ.get('SERVICE_NAME')
        ttl_hours = int(os.environ.get('TTL_HOURS', 0))
        
        if not cluster_name or not service_name or ttl_hours == 0:
            return False
            
        # Get ECS service creation time
        response = ecs.describe_services(cluster=cluster_name, services=[service_name])
        
        if not response['services']:
            logger.warning("Service not found, assuming cleanup needed")
            return True
        
        service = response['services'][0]
        created_at = service.get('createdAt')
        
        if not created_at:
            return check_ttl_from_tags()
        
        # Calculate if TTL has expired
        ttl_expiry = created_at + timedelta(hours=ttl_hours)
        current_time = datetime.now(timezone.utc)
        
        logger.info(f"Service created: {created_at}, TTL expiry: {ttl_expiry}, Current: {current_time}")
        return current_time >= ttl_expiry
        
    except Exception as e:
        logger.error(f"Error checking TTL: {str(e)}")
        return False

def check_ttl_from_tags() -> bool:
    """Fallback method to check TTL from ALB tags"""
    try:
        alb_arn = os.environ.get('ALB_ARN')
        if not alb_arn:
            return False
            
        response = elbv2.describe_tags(ResourceArns=[alb_arn])
        
        for tag_description in response['TagDescriptions']:
            for tag in tag_description['Tags']:
                if tag['Key'] == 'TTLExpiry':
                    ttl_expiry = datetime.fromisoformat(tag['Value'].replace('Z', '+00:00'))
                    current_time = datetime.now(timezone.utc)
                    return current_time >= ttl_expiry
        
        return False
    except Exception as e:
        logger.error(f"Error checking TTL from tags: {str(e)}")
        return False

def perform_cleanup() -> Dict[str, Any]:
    """Perform the actual infrastructure cleanup"""
    results = {
        'ecs_service': False,
        'ecs_cluster': False,
        'load_balancer': False,
        'target_groups': False,
        's3_bucket': False,
        'vpc_resources': False,
        'route53_records': False,
        'ssl_certificate': False
    }
    
    try:
        # 1. Scale down and delete ECS service
        cluster_name = os.environ.get('CLUSTER_NAME')
        service_name = os.environ.get('SERVICE_NAME')
        
        if cluster_name and service_name:
            logger.info("Cleaning up ECS service...")
            ecs.update_service(cluster=cluster_name, service=service_name, desiredCount=0)
            
            # Wait for tasks to stop
            import time
            time.sleep(30)
            
            ecs.delete_service(cluster=cluster_name, service=service_name)
            results['ecs_service'] = True
            
            # Delete ECS cluster
            ecs.delete_cluster(cluster=cluster_name)
            results['ecs_cluster'] = True
            logger.info("ECS service and cluster deleted")
        
        # 2. Delete Load Balancer and Target Groups
        alb_arn = os.environ.get('ALB_ARN')
        if alb_arn:
            logger.info("Cleaning up Load Balancer...")
            
            # Get target groups
            tg_response = elbv2.describe_target_groups()
            alb_target_groups = [
                tg['TargetGroupArn'] for tg in tg_response['TargetGroups']
                if tg['LoadBalancerArns'] and alb_arn in tg['LoadBalancerArns']
            ]
            
            # Delete ALB
            elbv2.delete_load_balancer(LoadBalancerArn=alb_arn)
            results['load_balancer'] = True
            
            # Delete target groups
            for tg_arn in alb_target_groups:
                elbv2.delete_target_group(TargetGroupArn=tg_arn)
            results['target_groups'] = True
            logger.info("Load balancer and target groups deleted")
        
        # 3. Clean up S3 bucket
        config_bucket = os.environ.get('CONFIG_BUCKET')
        if config_bucket:
            logger.info("Cleaning up S3 bucket...")
            cleanup_s3_bucket(config_bucket)
            results['s3_bucket'] = True
        
        # 4. Clean up Route53 records (if custom domain)
        if os.environ.get('CUSTOM_DOMAIN'):
            logger.info("Cleaning up Route53 records...")
            cleanup_route53_records()
            results['route53_records'] = True
        
        # 5. Clean up SSL certificate (if custom domain)
        if os.environ.get('CERTIFICATE_ARN'):
            logger.info("Cleaning up SSL certificate...")
            cleanup_ssl_certificate()
            results['ssl_certificate'] = True
        
        # 6. Clean up VPC resources
        vpc_id = os.environ.get('VPC_ID')
        if vpc_id:
            logger.info("Cleaning up VPC resources...")
            cleanup_vpc_resources(vpc_id)
            results['vpc_resources'] = True
        
        logger.info("All cleanup operations completed successfully")
        
    except Exception as e:
        logger.error(f"Error during cleanup: {str(e)}")
        results['error'] = str(e)
    
    return results

def cleanup_s3_bucket(bucket_name: str) -> None:
    """Empty and delete the S3 configuration bucket"""
    try:
        # List and delete all objects
        response = s3.list_objects_v2(Bucket=bucket_name)
        if 'Contents' in response:
            objects_to_delete = [{'Key': obj['Key']} for obj in response['Contents']]
            s3.delete_objects(Bucket=bucket_name, Delete={'Objects': objects_to_delete})
        
        # Delete the bucket
        s3.delete_bucket(Bucket=bucket_name)
        logger.info(f"S3 bucket {bucket_name} deleted")
        
    except Exception as e:
        logger.error(f"Error cleaning up S3 bucket: {str(e)}")
        raise

def cleanup_route53_records() -> None:
    """Clean up Route53 DNS records for custom domain"""
    try:
        hosted_zone_id = os.environ.get('HOSTED_ZONE_ID')
        custom_domain = os.environ.get('CUSTOM_DOMAIN')
        
        if not hosted_zone_id or not custom_domain:
            return
        
        response = route53.list_resource_record_sets(HostedZoneId=hosted_zone_id)
        
        changes = []
        for record in response['ResourceRecordSets']:
            if (record['Name'].rstrip('.') == custom_domain and 
                record['Type'] in ['A', 'AAAA'] and 
                'AliasTarget' in record):
                changes.append({'Action': 'DELETE', 'ResourceRecordSet': record})
        
        if changes:
            route53.change_resource_record_sets(
                HostedZoneId=hosted_zone_id,
                ChangeBatch={'Changes': changes}
            )
            logger.info(f"Route53 records deleted for {custom_domain}")
        
    except Exception as e:
        logger.error(f"Error cleaning up Route53 records: {str(e)}")
        raise

def cleanup_ssl_certificate() -> None:
    """Clean up ACM SSL certificate"""
    try:
        certificate_arn = os.environ.get('CERTIFICATE_ARN')
        if not certificate_arn:
            return
        
        try:
            acm.describe_certificate(CertificateArn=certificate_arn)
            acm.delete_certificate(CertificateArn=certificate_arn)
            logger.info(f"SSL certificate deleted: {certificate_arn}")
        except acm.exceptions.ResourceNotFoundException:
            logger.info("Certificate already deleted")
        
    except Exception as e:
        logger.error(f"Error cleaning up SSL certificate: {str(e)}")
        raise

def cleanup_vpc_resources(vpc_id: str) -> None:
    """Clean up VPC and all associated resources"""
    try:
        # Check if VPC exists
        vpc_response = ec2.describe_vpcs(VpcIds=[vpc_id])
        if not vpc_response['Vpcs']:
            logger.info("VPC already deleted")
            return
        
        # 1. Delete NAT Gateways
        nat_gateways = ec2.describe_nat_gateways(
            Filters=[{'Name': 'vpc-id', 'Values': [vpc_id]}]
        )['NatGateways']
        
        for nat_gw in nat_gateways:
            if nat_gw['State'] not in ['deleted', 'deleting']:
                ec2.delete_nat_gateway(NatGatewayId=nat_gw['NatGatewayId'])
                logger.info(f"NAT Gateway {nat_gw['NatGatewayId']} deletion initiated")
        
        # Wait for NAT gateways to be deleted
        import time
        time.sleep(60)
        
        # 2. Release Elastic IPs
        addresses = ec2.describe_addresses(
            Filters=[{'Name': 'domain', 'Values': ['vpc']}]
        )['Addresses']
        
        for addr in addresses:
            if 'AssociationId' not in addr:
                try:
                    ec2.release_address(AllocationId=addr['AllocationId'])
                    logger.info(f"Released EIP: {addr['AllocationId']}")
                except Exception as e:
                    logger.warning(f"Could not release EIP {addr['AllocationId']}: {str(e)}")
        
        # 3. Delete subnets
        subnets = ec2.describe_subnets(
            Filters=[{'Name': 'vpc-id', 'Values': [vpc_id]}]
        )['Subnets']
        
        for subnet in subnets:
            ec2.delete_subnet(SubnetId=subnet['SubnetId'])
            logger.info(f"Subnet {subnet['SubnetId']} deleted")
        
        # 4. Delete route tables (except main)
        route_tables = ec2.describe_route_tables(
            Filters=[{'Name': 'vpc-id', 'Values': [vpc_id]}]
        )['RouteTables']
        
        for rt in route_tables:
            if not any(assoc.get('Main', False) for assoc in rt.get('Associations', [])):
                ec2.delete_route_table(RouteTableId=rt['RouteTableId'])
                logger.info(f"Route table {rt['RouteTableId']} deleted")
        
        # 5. Detach and delete Internet Gateway
        igws = ec2.describe_internet_gateways(
            Filters=[{'Name': 'attachment.vpc-id', 'Values': [vpc_id]}]
        )['InternetGateways']
        
        for igw in igws:
            ec2.detach_internet_gateway(
                InternetGatewayId=igw['InternetGatewayId'],
                VpcId=vpc_id
            )
            ec2.delete_internet_gateway(InternetGatewayId=igw['InternetGatewayId'])
            logger.info(f"Internet Gateway {igw['InternetGatewayId']} deleted")
        
        # 6. Delete security groups (except default)
        security_groups = ec2.describe_security_groups(
            Filters=[{'Name': 'vpc-id', 'Values': [vpc_id]}]
        )['SecurityGroups']
        
        for sg in security_groups:
            if sg['GroupName'] != 'default':
                ec2.delete_security_group(GroupId=sg['GroupId'])
                logger.info(f"Security Group {sg['GroupId']} deleted")
        
        # 7. Finally, delete the VPC
        ec2.delete_vpc(VpcId=vpc_id)
        logger.info(f"VPC {vpc_id} deleted")
        
    except Exception as e:
        logger.error(f"Error cleaning up VPC resources: {str(e)}")
        raise

def send_cleanup_notification() -> None:
    """Send notification before starting cleanup"""
    sns_topic_arn = "${sns_topic_arn}"
    if not sns_topic_arn:
        return
    
    try:
        project_name = os.environ.get('PROJECT_NAME', 'Unknown')
        environment = os.environ.get('ENVIRONMENT', 'Unknown')
        ttl_hours = os.environ.get('TTL_HOURS', '0')
        
        message = f"""AutoMock ECS Fargate Infrastructure Cleanup Starting

Project: {project_name}
Environment: {environment}
TTL Hours: {ttl_hours}

The following AWS resources will be deleted:
- ECS Cluster and Fargate Service
- Application Load Balancer
- VPC and networking resources
- S3 configuration bucket
- SSL certificates (if applicable)
- Route53 DNS records (if applicable)

This action cannot be undone.

To extend the TTL or cancel cleanup, please access your AutoMock CLI immediately."""
        
        sns.publish(
            TopicArn=sns_topic_arn,
            Subject=f"AutoMock ECS Cleanup Starting - {project_name}",
            Message=message
        )
        logger.info("Cleanup notification sent")
        
    except Exception as e:
        logger.error(f"Error sending cleanup notification: {str(e)}")

def send_completion_notification(results: Dict[str, Any]) -> None:
    """Send notification after cleanup completion"""
    sns_topic_arn = "${sns_topic_arn}"
    if not sns_topic_arn:
        return
    
    try:
        project_name = os.environ.get('PROJECT_NAME', 'Unknown')
        environment = os.environ.get('ENVIRONMENT', 'Unknown')
        
        success_count = sum(1 for v in results.values() if v is True)
        total_operations = len([k for k in results.keys() if k != 'error'])
        
        status_symbols = {
            'ecs_service': '✓' if results.get('ecs_service') else '✗',
            'ecs_cluster': '✓' if results.get('ecs_cluster') else '✗',
            'load_balancer': '✓' if results.get('load_balancer') else '✗',
            'target_groups': '✓' if results.get('target_groups') else '✗',
            's3_bucket': '✓' if results.get('s3_bucket') else '✗',
            'vpc_resources': '✓' if results.get('vpc_resources') else '✗',
            'route53_records': '✓' if results.get('route53_records') else '✗',
            'ssl_certificate': '✓' if results.get('ssl_certificate') else '✗'
        }
        
        message = f"""AutoMock ECS Fargate Infrastructure Cleanup Completed

Project: {project_name}
Environment: {environment}

Cleanup Results ({success_count}/{total_operations} operations completed):
- ECS Service: {status_symbols['ecs_service']}
- ECS Cluster: {status_symbols['ecs_cluster']}
- Load Balancer: {status_symbols['load_balancer']}
- Target Groups: {status_symbols['target_groups']}
- S3 Bucket: {status_symbols['s3_bucket']}
- VPC Resources: {status_symbols['vpc_resources']}
- Route53 Records: {status_symbols['route53_records']}
- SSL Certificate: {status_symbols['ssl_certificate']}

{'Error: ' + results.get('error', '') if 'error' in results else 'All ECS Fargate resources have been successfully deleted.'}

Your AutoMock infrastructure has been automatically cleaned up to prevent unnecessary AWS costs."""
        
        sns.publish(
            TopicArn=sns_topic_arn,
            Subject=f"AutoMock ECS Cleanup Complete - {project_name}",
            Message=message
        )
        logger.info("Completion notification sent")
        
    except Exception as e:
        logger.error(f"Error sending completion notification: {str(e)}")

def send_error_notification(error: str) -> None:
    """Send notification if cleanup fails"""
    sns_topic_arn = "${sns_topic_arn}"
    if not sns_topic_arn:
        return
    
    try:
        project_name = os.environ.get('PROJECT_NAME', 'Unknown')
        environment = os.environ.get('ENVIRONMENT', 'Unknown')
        
        message = f"""AutoMock ECS Fargate Infrastructure Cleanup FAILED

Project: {project_name}
Environment: {environment}

Error: {error}

Manual cleanup may be required to prevent ongoing AWS costs.
Please check the AWS console and remove any remaining resources:

- ECS Services and Clusters
- Application Load Balancers and Target Groups
- VPC and networking resources (subnets, NAT gateways, etc.)
- S3 buckets
- Route53 DNS records
- SSL certificates

Contact your system administrator if you need assistance with manual cleanup."""
        
        sns.publish(
            TopicArn=sns_topic_arn,
            Subject=f"AutoMock ECS Cleanup FAILED - {project_name}",
            Message=message
        )
        logger.info("Error notification sent")
        
    except Exception as e:
        logger.error(f"Error sending error notification: {str(e)}")
