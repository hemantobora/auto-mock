# terraform/modules/iam/outputs.tf

output "ecs_task_execution_role_arn" {
  description = "ARN of the ECS task execution role"
  value       = aws_iam_role.ecs_task_execution.arn
}

output "ecs_task_execution_role_name" {
  description = "Name of the ECS task execution role"
  value       = aws_iam_role.ecs_task_execution.name
}

output "ecs_task_role_arn" {
  description = "ARN of the ECS task role"
  value       = aws_iam_role.ecs_task.arn
}

output "ecs_task_role_name" {
  description = "Name of the ECS task role"
  value       = aws_iam_role.ecs_task.name
}

output "lambda_teardown_role_arn" {
  description = "ARN of the Lambda teardown role"
  value       = aws_iam_role.lambda_teardown.arn
}

output "lambda_teardown_role_name" {
  description = "Name of the Lambda teardown role"
  value       = aws_iam_role.lambda_teardown.name
}

output "ecs_autoscaling_role_arn" {
  description = "ARN of the ECS autoscaling role"
  value       = aws_iam_role.ecs_autoscaling.arn
}

output "ecs_autoscaling_role_name" {
  description = "Name of the ECS autoscaling role"
  value       = aws_iam_role.ecs_autoscaling.name
}
