#!/usr/bin/env python3
"""
TTL Cleanup Lambda Function for AutoMock Infrastructure
Automatically tears down infrastructure when TTL expires
"""

import json
import os
import time
import boto3
from datetime import datetime, timedelta
from typing import Dict, List, Any

# AWS clients
ecs = boto3.client('ecs')
elbv2 = boto3.client('elbv2')
ec2 = boto3.client('ec2')
s3 = boto3.client('s3')
logs = boto3.client('logs')
events = boto3.client('events')
lambda_client = boto3.client('lambda')
sns = boto3.client('sns')

# Environment variables
PROJECT_NAME = os.environ['PROJECT_NAME']
ENVIRONMENT = os.environ['ENVIRONMENT']
CLUSTER_NAME = os.environ['CLUSTER_NAME']
SERVICE_NAME = os.environ['SERVICE_NAME']
ALB_ARN = os.environ['ALB_ARN']
TARGET_GROUP_API_ARN = os.environ['TARGET_GROUP_API_ARN']
TARGET_GROUP_DASH_ARN = os.environ['TARGET_GROUP_DASH_ARN']
VPC_ID = os.environ['VPC_ID']
CONFIG_BUCKET = os.environ['CONFIG_BUCKET']
REGION = os.environ['REGION']
TTL_HOURS = int(os.environ['TTL_HOURS'])
NOTIFICATION_EMAIL = os.environ.get('NOTIFICATION_EMAIL', '')


def lambda_handler(event, context):
    """Main Lambda handler for TTL cleanup"""
    print(f"Checking TTL for project: {PROJECT_NAME} (environment: {ENVIRONMENT})")
    
    try:
        # Read metadata from S3 to get TTL expiry time
        metadata = get_metadata_from_s3()
        
        if not metadata:
            print("No metadata found, skipping cleanup")
            return {
                'statusCode': 200,
                'body': json.dumps('No metadata found')
            }
        
        ttl_timestamp = metadata.get('ttl_timestamp')
        if not ttl_timestamp:
            print("No TTL set, skipping cleanup")
            return {
                'statusCode': 200,
                'body': json.dumps('No TTL configured')
            }
        
        # Check if TTL expired
        ttl_datetime = datetime.fromisoformat(ttl_timestamp.replace('Z', '+00:00'))
        now = datetime.utcnow()
        
        if now < ttl_datetime:
            remaining_hours = (ttl_datetime - now).total_seconds() / 3600
            print(f"TTL not expired. {remaining_hours:.1f} hours remaining")
            
            # Send warning notification if less than 1 hour remaining
            if remaining_hours < 1 and NOTIFICATION_EMAIL:
                send_warning_notification(remaining_hours)
            
            return {
                'statusCode': 200,
                'body': json.dumps(f'TTL not expired: {remaining_hours:.1f} hours remaining')
            }
        
        print(f"TTL expired! Starting cleanup...")
        
        # Send notification before starting cleanup
        if NOTIFICATION_EMAIL:
            send_cleanup_notification()
        
        # Execute cleanup sequence
        cleanup_results = execute_cleanup()
        
        print(f"Cleanup complete for project {PROJECT_NAME}")
        
        return {
            'statusCode': 200,
            'body': json.dumps({
                'message': 'Infrastructure cleaned up successfully',
                'project': PROJECT_NAME,
                'results': cleanup_results
            })
        }
        
    except Exception as e:
        error_msg = f"Error during cleanup: {str(e)}"
        print(error_msg)
        
        if NOTIFICATION_EMAIL:
            send_error_notification(str(e))
        
        raise


def get_metadata_from_s3() -> Dict[str, Any]:
    """Retrieve metadata from S3 bucket"""
    try:
        response = s3.get_object(Bucket=CONFIG_BUCKET, Key='project-metadata.json')
        metadata = json.loads(response['Body'].read())
        return metadata
    except s3.exceptions.NoSuchKey:
        print(f"Metadata file not found in bucket: {CONFIG_BUCKET}")
        return {}
    except Exception as e:
        print(f"Error reading metadata: {str(e)}")
        return {}


def execute_cleanup() -> Dict[str, Any]:
    """Execute the complete cleanup sequence"""
    results = {}
    
    # Step 1: Scale ECS service to 0
    print("Scaling ECS service to 0 tasks...")
    results['ecs_scale'] = scale_ecs_service_to_zero()
    
    # Step 2: Delete ECS service
    print("Deleting ECS service...")
    results['ecs_service'] = delete_ecs_service()
    
    # Step 3: Delete ECS cluster
    print("Deleting ECS cluster...")
    results['ecs_cluster'] = delete_ecs_cluster()
    
    # Step 4: Delete ALB
    print("Deleting Application Load Balancer...")
    results['alb'] = delete_alb()
    
    # Step 5: Delete target groups
    print("Deleting target groups...")
    results['target_groups'] = delete_target_groups()
    
    # Step 6: Delete VPC resources
    print("Deleting VPC resources...")
    results['vpc'] = delete_vpc_resources()
    
    # Step 7: Delete S3 bucket
    print("Deleting S3 bucket...")
    results['s3_bucket'] = delete_s3_bucket()
    
    # Step 8: Delete CloudWatch logs
    print("Deleting CloudWatch log groups...")
    results['logs'] = delete_cloudwatch_logs()
    
    # Step 9: Delete EventBridge rule and self-destruct Lambda
    print("Cleaning up TTL infrastructure...")
    results['ttl_cleanup'] = cleanup_ttl_resources()
    
    return results


def scale_ecs_service_to_zero() -> Dict[str, Any]:
    """Scale ECS service to 0 tasks"""
    try:
        ecs.update_service(
            cluster=CLUSTER_NAME,
            service=SERVICE_NAME,
            desiredCount=0
        )
        
        # Wait for tasks to drain
        print("Waiting for tasks to drain...")
        waiter = ecs.get_waiter('services_stable')
        waiter.wait(
            cluster=CLUSTER_NAME,
            services=[SERVICE_NAME],
            WaiterConfig={'Delay': 10, 'MaxAttempts': 30}
        )
        
        return {'success': True, 'message': f'Scaled {SERVICE_NAME} to 0'}
    except Exception as e:
        return {'success': False, 'error': str(e)}


def delete_ecs_service() -> Dict[str, Any]:
    """Delete ECS service"""
    try:
        ecs.delete_service(cluster=CLUSTER_NAME, service=SERVICE_NAME, force=True)
        time.sleep(5)  # Give AWS time to process
        return {'success': True, 'message': f'Deleted service {SERVICE_NAME}'}
    except Exception as e:
        return {'success': False, 'error': str(e)}


def delete_ecs_cluster() -> Dict[str, Any]:
    """Delete ECS cluster"""
    try:
        ecs.delete_cluster(cluster=CLUSTER_NAME)
        return {'success': True, 'message': f'Deleted cluster {CLUSTER_NAME}'}
    except Exception as e:
        return {'success': False, 'error': str(e)}


def delete_alb() -> Dict[str, Any]:
    """Delete Application Load Balancer"""
    try:
        # Delete all listeners first
        listeners = elbv2.describe_listeners(LoadBalancerArn=ALB_ARN)
        for listener in listeners['Listeners']:
            elbv2.delete_listener(ListenerArn=listener['ListenerArn'])
            print(f"  Deleted listener: {listener['ListenerArn']}")
        
        # Delete ALB
        elbv2.delete_load_balancer(LoadBalancerArn=ALB_ARN)
        
        # Wait for ALB to be deleted
        time.sleep(10)
        
        return {'success': True, 'message': 'Deleted ALB and listeners'}
    except Exception as e:
        return {'success': False, 'error': str(e)}


def delete_target_groups() -> Dict[str, Any]:
    """Delete target groups"""
    deleted = []
    errors = []
    
    for tg_arn in [TARGET_GROUP_API_ARN, TARGET_GROUP_DASH_ARN]:
        try:
            elbv2.delete_target_group(TargetGroupArn=tg_arn)
            deleted.append(tg_arn)
            print(f"  Deleted target group: {tg_arn}")
        except Exception as e:
            errors.append({'arn': tg_arn, 'error': str(e)})
    
    return {
        'success': len(errors) == 0,
        'deleted': len(deleted),
        'errors': errors
    }


def delete_vpc_resources() -> Dict[str, Any]:
    """Delete VPC and associated resources"""
    results = {}
    
    try:
        # Delete NAT Gateways
        nat_gateways = ec2.describe_nat_gateways(
            Filters=[{'Name': 'vpc-id', 'Values': [VPC_ID]}]
        )
        for nat in nat_gateways['NatGateways']:
            if nat['State'] != 'deleted':
                ec2.delete_nat_gateway(NatGatewayId=nat['NatGatewayId'])
                print(f"  Deleted NAT Gateway: {nat['NatGatewayId']}")
        
        # Wait for NAT gateways to be deleted
        time.sleep(30)
        
        # Release Elastic IPs
        addresses = ec2.describe_addresses(
            Filters=[{'Name': 'domain', 'Values': ['vpc']}]
        )
        for addr in addresses['Addresses']:
            if 'NetworkInterfaceId' not in addr:
                try:
                    ec2.release_address(AllocationId=addr['AllocationId'])
                    print(f"  Released EIP: {addr['AllocationId']}")
                except Exception as e:
                    print(f"  Error releasing EIP: {str(e)}")
        
        # Delete security groups (except default)
        security_groups = ec2.describe_security_groups(
            Filters=[{'Name': 'vpc-id', 'Values': [VPC_ID]}]
        )
        for sg in security_groups['SecurityGroups']:
            if sg['GroupName'] != 'default':
                try:
                    ec2.delete_security_group(GroupId=sg['GroupId'])
                    print(f"  Deleted security group: {sg['GroupId']}")
                except Exception as e:
                    print(f"  Error deleting SG: {str(e)}")
        
        # Delete subnets
        subnets = ec2.describe_subnets(
            Filters=[{'Name': 'vpc-id', 'Values': [VPC_ID]}]
        )
        for subnet in subnets['Subnets']:
            ec2.delete_subnet(SubnetId=subnet['SubnetId'])
            print(f"  Deleted subnet: {subnet['SubnetId']}")
        
        # Detach and delete internet gateway
        igws = ec2.describe_internet_gateways(
            Filters=[{'Name': 'attachment.vpc-id', 'Values': [VPC_ID]}]
        )
        for igw in igws['InternetGateways']:
            ec2.detach_internet_gateway(
                InternetGatewayId=igw['InternetGatewayId'],
                VpcId=VPC_ID
            )
            ec2.delete_internet_gateway(InternetGatewayId=igw['InternetGatewayId'])
            print(f"  Deleted IGW: {igw['InternetGatewayId']}")
        
        # Delete route tables (except main)
        route_tables = ec2.describe_route_tables(
            Filters=[{'Name': 'vpc-id', 'Values': [VPC_ID]}]
        )
        for rt in route_tables['RouteTables']:
            is_main = any(assoc.get('Main', False) for assoc in rt.get('Associations', []))
            if not is_main:
                ec2.delete_route_table(RouteTableId=rt['RouteTableId'])
                print(f"  Deleted route table: {rt['RouteTableId']}")
        
        # Finally, delete VPC
        ec2.delete_vpc(VpcId=VPC_ID)
        print(f"  Deleted VPC: {VPC_ID}")
        
        return {'success': True, 'message': 'Deleted VPC resources'}
    except Exception as e:
        return {'success': False, 'error': str(e)}


def delete_s3_bucket() -> Dict[str, Any]:
    """Delete S3 bucket and all contents"""
    try:
        # Delete all objects
        paginator = s3.get_paginator('list_object_versions')
        delete_markers = []
        versions = []
        
        for page in paginator.paginate(Bucket=CONFIG_BUCKET):
            if 'DeleteMarkers' in page:
                delete_markers.extend(page['DeleteMarkers'])
            if 'Versions' in page:
                versions.extend(page['Versions'])
        
        # Delete all versions
        objects_to_delete = []
        for version in versions:
            objects_to_delete.append({'Key': version['Key'], 'VersionId': version['VersionId']})
        
        for marker in delete_markers:
            objects_to_delete.append({'Key': marker['Key'], 'VersionId': marker['VersionId']})
        
        if objects_to_delete:
            # Delete in batches of 1000 (AWS limit)
            for i in range(0, len(objects_to_delete), 1000):
                batch = objects_to_delete[i:i+1000]
                s3.delete_objects(
                    Bucket=CONFIG_BUCKET,
                    Delete={'Objects': batch}
                )
            print(f"  Deleted {len(objects_to_delete)} objects from {CONFIG_BUCKET}")
        
        # Delete bucket
        s3.delete_bucket(Bucket=CONFIG_BUCKET)
        print(f"  Deleted bucket: {CONFIG_BUCKET}")
        
        return {'success': True, 'message': f'Deleted bucket {CONFIG_BUCKET}'}
    except Exception as e:
        return {'success': False, 'error': str(e)}


def delete_cloudwatch_logs() -> Dict[str, Any]:
    """Delete CloudWatch log groups"""
    deleted = []
    errors = []
    
    log_groups = [
        f'/ecs/automock/{PROJECT_NAME}/mockserver',
        f'/ecs/automock/{PROJECT_NAME}/config-loader',
        f'/aws/lambda/automock-{PROJECT_NAME}-{ENVIRONMENT}-ttl-cleanup'
    ]
    
    for log_group in log_groups:
        try:
            logs.delete_log_group(logGroupName=log_group)
            deleted.append(log_group)
            print(f"  Deleted log group: {log_group}")
        except logs.exceptions.ResourceNotFoundException:
            pass  # Already deleted
        except Exception as e:
            errors.append({'log_group': log_group, 'error': str(e)})
    
    return {
        'success': len(errors) == 0,
        'deleted': len(deleted),
        'errors': errors
    }


def cleanup_ttl_resources() -> Dict[str, Any]:
    """Clean up EventBridge rule and Lambda function (self-destruct)"""
    try:
        rule_name = f'automock-{PROJECT_NAME}-{ENVIRONMENT}-ttl-check'
        function_name = f'automock-{PROJECT_NAME}-{ENVIRONMENT}-ttl-cleanup'
        
        # Remove EventBridge targets
        try:
            events.remove_targets(Rule=rule_name, Ids=['TTLCleanupLambda'])
            print(f"  Removed EventBridge targets from rule: {rule_name}")
        except Exception as e:
            print(f"  Error removing targets: {str(e)}")
        
        # Delete EventBridge rule
        try:
            events.delete_rule(Name=rule_name)
            print(f"  Deleted EventBridge rule: {rule_name}")
        except Exception as e:
            print(f"  Error deleting rule: {str(e)}")
        
        # Delete Lambda function (self-destruct)
        try:
            lambda_client.delete_function(FunctionName=function_name)
            print(f"  Deleted Lambda function: {function_name}")
        except Exception as e:
            print(f"  Error deleting Lambda: {str(e)}")
        
        return {'success': True, 'message': 'Cleaned up TTL resources'}
    except Exception as e:
        return {'success': False, 'error': str(e)}


def send_warning_notification(remaining_hours: float):
    """Send warning notification when TTL is about to expire"""
    try:
        subject = f'[AutoMock] TTL Warning - {PROJECT_NAME}'
        message = f"""
AutoMock Infrastructure TTL Warning

Project: {PROJECT_NAME}
Environment: {ENVIRONMENT}
Region: {REGION}

Your infrastructure will be automatically deleted in {remaining_hours:.1f} hours.

If you need more time, you can extend the TTL using:
  automock extend-ttl --project {PROJECT_NAME} --hours <ADDITIONAL_HOURS>

Infrastructure Details:
- ECS Cluster: {CLUSTER_NAME}
- ECS Service: {SERVICE_NAME}
- Config Bucket: {CONFIG_BUCKET}

This is an automated message from AutoMock TTL Cleanup.
"""
        
        sns.publish(
            TopicArn=os.environ.get('SNS_TOPIC_ARN', ''),
            Subject=subject,
            Message=message
        )
    except Exception as e:
        print(f"Error sending warning notification: {str(e)}")


def send_cleanup_notification():
    """Send notification that cleanup is starting"""
    try:
        subject = f'[AutoMock] Infrastructure Cleanup Started - {PROJECT_NAME}'
        message = f"""
AutoMock Infrastructure Cleanup

Project: {PROJECT_NAME}
Environment: {ENVIRONMENT}
Region: {REGION}

Your infrastructure TTL has expired and cleanup has started.

Resources being deleted:
- ECS Cluster: {CLUSTER_NAME}
- ECS Service: {SERVICE_NAME}
- Application Load Balancer
- VPC and networking resources
- S3 Configuration Bucket: {CONFIG_BUCKET}
- CloudWatch Logs
- TTL Lambda Function

This is an automated message from AutoMock TTL Cleanup.
"""
        
        sns.publish(
            TopicArn=os.environ.get('SNS_TOPIC_ARN', ''),
            Subject=subject,
            Message=message
        )
    except Exception as e:
        print(f"Error sending cleanup notification: {str(e)}")


def send_error_notification(error: str):
    """Send notification when cleanup encounters an error"""
    try:
        subject = f'[AutoMock] Cleanup Error - {PROJECT_NAME}'
        message = f"""
AutoMock Infrastructure Cleanup Error

Project: {PROJECT_NAME}
Environment: {ENVIRONMENT}
Region: {REGION}

An error occurred during infrastructure cleanup:

{error}

Some resources may not have been deleted. Please check your AWS console
and manually delete any remaining resources to avoid unexpected charges.

This is an automated message from AutoMock TTL Cleanup.
"""
        
        sns.publish(
            TopicArn=os.environ.get('SNS_TOPIC_ARN', ''),
            Subject=subject,
            Message=message
        )
    except Exception as e:
        print(f"Error sending error notification: {str(e)}")
