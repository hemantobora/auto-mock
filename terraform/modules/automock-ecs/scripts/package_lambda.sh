#!/bin/bash
# Script to package Lambda function for Terraform deployment

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
cd "$SCRIPT_DIR"

echo "Packaging TTL cleanup Lambda function..."

# Remove old zip if it exists
rm -f ttl_cleanup.zip

# Create ZIP archive directly
zip -q ttl_cleanup.zip ttl_cleanup.py

echo "✓ Lambda function packaged: ttl_cleanup.zip"
ls -lh ttl_cleanup.zip
