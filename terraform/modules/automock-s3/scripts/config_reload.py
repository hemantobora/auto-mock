#!/usr/bin/env python3
"""
AutoMock Config Reload Lambda Function
Triggers ECS service update when expectations.json changes in S3
"""

import json
import boto3
import logging
from typing import Dict, Any

# Configure logging
logger = logging.getLogger()
logger.setLevel(logging.INFO)

# Initialize AWS clients
ecs = boto3.client('ecs')
s3 = boto3.client('s3')

def handler(event: Dict[str, Any], context: Any) -> Dict[str, Any]:
    """
    Lambda handler triggered by S3 object changes
    """
    try:
        logger.info("Config reload triggered by S3 event")
        
        # Parse S3 event
        for record in event.get('Records', []):
            bucket_name = record['s3']['bucket']['name']
            object_key = record['s3']['object']['key']
            event_name = record['eventName']
            
            logger.info(f"S3 Event: {event_name} for {bucket_name}/{object_key}")
            
            # Only process expectations.json changes
            if object_key == 'expectations.json':
                if event_name.startswith('ObjectCreated'):
                    result = handle_config_update(bucket_name, object_key)
                elif event_name.startswith('ObjectRemoved'):
                    result = handle_config_removal(bucket_name, object_key)
                else:
                    logger.info(f"Ignoring event type: {event_name}")
                    continue
                    
                logger.info(f"Config reload result: {result}")
        
        return {
            'statusCode': 200,
            'body': json.dumps({'message': 'Config reload processed successfully'})
        }
        
    except Exception as e:
        logger.error(f"Config reload failed: {str(e)}")
        return {
            'statusCode': 500,
            'body': json.dumps({'error': str(e)})
        }

def handle_config_update(bucket_name: str, object_key: str) -> Dict[str, Any]:
    """
    Handle expectations.json creation/update
    """
    import os
    
    cluster_arn = os.environ.get('ECS_CLUSTER_ARN')
    service_name = os.environ.get('ECS_SERVICE_NAME')
    project_name = os.environ.get('PROJECT_NAME', 'unknown')
    environment = os.environ.get('ENVIRONMENT', 'unknown')
    
    if not cluster_arn or not service_name:
        logger.warning("ECS cluster or service not configured, skipping reload")
        return {'action': 'skipped', 'reason': 'ECS not configured'}
    
    try:
        # Validate the new expectations file
        logger.info(f"Validating new expectations from s3://{bucket_name}/{object_key}")
        
        response = s3.get_object(Bucket=bucket_name, Key=object_key)
        expectations_content = response['Body'].read().decode('utf-8')
        expectations = json.loads(expectations_content)
        
        # Basic validation
        if not isinstance(expectations, list):
            raise ValueError("Expectations must be a JSON array")
        
        expectation_count = len(expectations)
        logger.info(f"Validated {expectation_count} expectations")
        
        # Create a new version backup
        create_version_backup(bucket_name, expectations_content)
        
        # Trigger ECS service update to reload configuration
        logger.info(f"Triggering ECS service reload: {service_name}")
        
        ecs_response = ecs.update_service(
            cluster=cluster_arn,
            service=service_name,
            forceNewDeployment=True
        )
        
        logger.info(f"ECS service update initiated for {service_name}")
        
        # Update project metadata with new info
        update_project_metadata(bucket_name, {
            'last_config_update': context.aws_request_id if context else 'unknown',
            'expectations_count': expectation_count,
            'last_updated': json.dumps(context.get_remaining_time_in_millis() if context else 'unknown'),
            'reload_status': 'success'
        })
        
        return {
            'action': 'reloaded',
            'expectations_count': expectation_count,
            'ecs_deployment_arn': ecs_response['service']['deployments'][0]['taskDefinition'] if ecs_response else None
        }
        
    except json.JSONDecodeError as e:
        logger.error(f"Invalid JSON in expectations file: {str(e)}")
        return {'action': 'failed', 'reason': f'Invalid JSON: {str(e)}'}
        
    except Exception as e:
        logger.error(f"Failed to reload config: {str(e)}")
        return {'action': 'failed', 'reason': str(e)}

def handle_config_removal(bucket_name: str, object_key: str) -> Dict[str, Any]:
    """
    Handle expectations.json deletion
    """
    logger.warning(f"Expectations file deleted: s3://{bucket_name}/{object_key}")
    
    # Could implement fallback to previous version or default config
    # For now, just log the event
    
    return {
        'action': 'deleted',
        'message': 'Expectations file was deleted'
    }

def create_version_backup(bucket_name: str, expectations_content: str) -> None:
    """
    Create a versioned backup of the expectations
    """
    try:
        import datetime
        
        # Get current version number
        try:
            # List existing versions
            response = s3.list_objects_v2(
                Bucket=bucket_name,
                Prefix='versions/expectations-v'
            )
            
            version_numbers = []
            for obj in response.get('Contents', []):
                key = obj['Key']
                if key.startswith('versions/expectations-v') and key.endswith('.json'):
                    try:
                        version_num = int(key.split('-v')[1].split('.json')[0])
                        version_numbers.append(version_num)
                    except (IndexError, ValueError):
                        continue
            
            next_version = max(version_numbers, default=0) + 1
            
        except Exception as e:
            logger.warning(f"Could not determine version number: {e}")
            next_version = 1
        
        # Create versioned backup
        version_key = f"versions/expectations-v{next_version}.json"
        
        s3.put_object(
            Bucket=bucket_name,
            Key=version_key,
            Body=expectations_content,
            ContentType='application/json',
            Metadata={
                'created-by': 'automock-config-reload',
                'backup-time': datetime.datetime.utcnow().isoformat(),
                'version': str(next_version)
            }
        )
        
        logger.info(f"Created version backup: s3://{bucket_name}/{version_key}")
        
    except Exception as e:
        logger.error(f"Failed to create version backup: {e}")
        # Don't fail the main operation if backup fails

def update_project_metadata(bucket_name: str, updates: Dict[str, Any]) -> None:
    """
    Update project metadata with reload information
    """
    try:
        # Get current metadata
        try:
            response = s3.get_object(Bucket=bucket_name, Key='project-metadata.json')
            metadata = json.loads(response['Body'].read().decode('utf-8'))
        except s3.exceptions.NoSuchKey:
            metadata = {}
        
        # Add reload information
        if 'config_reloads' not in metadata:
            metadata['config_reloads'] = []
        
        reload_info = {
            'timestamp': context.aws_request_id if 'context' in globals() else 'unknown',
            **updates
        }
        
        metadata['config_reloads'].append(reload_info)
        
        # Keep only last 10 reload records
        metadata['config_reloads'] = metadata['config_reloads'][-10:]
        
        # Update last_updated
        metadata['last_updated'] = reload_info['timestamp']
        
        # Save updated metadata
        s3.put_object(
            Bucket=bucket_name,
            Key='project-metadata.json',
            Body=json.dumps(metadata, indent=2),
            ContentType='application/json'
        )
        
        logger.info("Updated project metadata with reload info")
        
    except Exception as e:
        logger.error(f"Failed to update project metadata: {e}")
        # Don't fail the main operation if metadata update fails