#!/bin/bash
set -e

echo "🔧 Starting Terraform destroy process..."
echo "Project: ${PROJECT_NAME}"
echo "Region: ${AWS_REGION}"
echo "Bucket: ${S3_BUCKET}"

# Create backend configuration dynamically
cat > backend.tf << EOF
terraform {
  backend "s3" {
    bucket  = "${S3_BUCKET}"
    key     = "terraform/state/terraform.tfstate"
    region  = "${AWS_REGION}"
    encrypt = true
  }
}
EOF

echo "📥 Initializing Terraform..."
terraform init -input=false

echo "🗑️  Destroying infrastructure..."
terraform destroy -auto-approve -input=false

echo "✅ Infrastructure destroyed successfully!"

# Optional: Delete the state file
echo "🧹 Cleaning up state file..."
aws s3 rm "s3://${S3_BUCKET}/terraform/state/terraform.tfstate" 2>/dev/null || true

echo "✅ Cleanup complete!"