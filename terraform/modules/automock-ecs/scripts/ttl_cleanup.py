#!/usr/bin/env python3
"""
TTL Cleanup Lambda - Triggers ECS Terraform Destroy Task
"""

import json
import os
import boto3
from datetime import datetime

ecs = boto3.client('ecs')
s3 = boto3.client('s3')

# Environment variables
PROJECT_NAME = os.environ['PROJECT_NAME']
CLUSTER_NAME = os.environ['CLUSTER_NAME']
DESTROY_TASK_DEFINITION = os.environ['DESTROY_TASK_DEFINITION']
SUBNETS = os.environ['SUBNETS'].split(',')
SECURITY_GROUP = os.environ['SECURITY_GROUP']
CONFIG_BUCKET = os.environ['CONFIG_BUCKET']


def lambda_handler(event, context):
    """Check TTL and trigger ECS destroy task if expired"""
    print(f"Checking TTL for project: {PROJECT_NAME}")
    
    try:
        # Read metadata
        metadata = get_metadata_from_s3()
        if not metadata:
            print("No metadata found")
            return {'statusCode': 200, 'body': 'No metadata'}
        
        # Check TTL
        ttl_expiry = metadata.get('ttl_expiry')
        if not ttl_expiry:
            print("No TTL set")
            return {'statusCode': 200, 'body': 'No TTL set'}
        
        # Parse TTL expiry time
        ttl_datetime = datetime.fromisoformat(ttl_expiry.replace('Z', '+00:00'))
        now = datetime.utcnow().replace(tzinfo=ttl_datetime.tzinfo)

        if event.get('force_destroy'):
            print("Force destroy flag detected. Skipping TTL check.")
        else:
            if now < ttl_datetime:
                remaining = (ttl_datetime - now).total_seconds() / 3600
                print(f"TTL not expired. {remaining:.1f} hours remaining")
                return {
                    'statusCode': 200,
                    'body': json.dumps(f'{remaining:.1f} hours remaining')
                }
        
        # TTL expired - trigger destroy task
        print(f"⚠️  TTL expired! Starting Terraform destroy task...")
        
        response = ecs.run_task(
            cluster=CLUSTER_NAME,
            taskDefinition=DESTROY_TASK_DEFINITION,
            launchType='FARGATE',
            networkConfiguration={
                'awsvpcConfiguration': {
                    'subnets': SUBNETS,
                    'securityGroups': [SECURITY_GROUP],
                    'assignPublicIp': 'ENABLED'  # Needs internet for Terraform providers
                }
            },
            tags=[
                {
                    'key': 'Purpose',
                    'value': 'TTL-Cleanup'
                },
                {
                    'key': 'Project',
                    'value': PROJECT_NAME
                }
            ]
        )
        
        if response.get('failures'):
            error_msg = f"Failed to start task: {response['failures']}"
            print(error_msg)
            return {
                'statusCode': 500,
                'body': json.dumps({'error': error_msg})
            }
        
        task_arn = response['tasks'][0]['taskArn']
        task_id = task_arn.split('/')[-1]
        
        print(f"✅ Started destroy task: {task_id}")
        print(f"📋 Task ARN: {task_arn}")
        print(f"📊 View logs: /ecs/automock/{PROJECT_NAME}/terraform-destroy")
        
        return {
            'statusCode': 200,
            'body': json.dumps({
                'message': 'Terraform destroy task started',
                'task_arn': task_arn,
                'task_id': task_id
            })
        }
        
    except Exception as e:
        error_msg = f"Error: {str(e)}"
        print(error_msg)
        return {
            'statusCode': 500,
            'body': json.dumps({'error': error_msg})
        }


def get_metadata_from_s3():
    """Read metadata from S3"""
    try:
        response = s3.get_object(
            Bucket=CONFIG_BUCKET, 
            Key='deployment-metadata.json'
        )
        return json.loads(response['Body'].read())
    except s3.exceptions.NoSuchKey:
        print(f"Metadata file not found in bucket: {CONFIG_BUCKET}")
        return {}
    except Exception as e:
        print(f"Error reading metadata: {e}")
        return {}