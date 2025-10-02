#!/bin/bash
# Script to package Lambda function for Terraform deployment

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
cd "$SCRIPT_DIR"

echo "Packaging TTL cleanup Lambda function..."

# Create temporary directory
TMP_DIR=$(mktemp -d)
trap "rm -rf $TMP_DIR" EXIT

# Copy Python script
cp ttl_cleanup.py "$TMP_DIR/"

# Create ZIP archive
cd "$TMP_DIR"
zip -q ttl_cleanup.zip ttl_cleanup.py

# Move to scripts directory
mv ttl_cleanup.zip "$SCRIPT_DIR/"

echo "✓ Lambda function packaged: ttl_cleanup.zip"
