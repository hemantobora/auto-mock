#!/bin/bash

# Build script for AutoMock

echo "🔨 Building AutoMock..."

# Clean previous builds
rm -f automock
rm -f cmd/lambda/bootstrap

# Build main CLI
echo "📦 Building CLI..."
go build -o automock cmd/auto-mock/main.go

# Build Lambda function
echo "🚀 Building Lambda function..."
GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build -o cmd/lambda/bootstrap ./cmd/lambda

echo "✅ Build complete!"
echo ""
echo "To run AutoMock:"
echo "  export ANTHROPIC_API_KEY='your-key-here'"
echo "  ./automock init --project my-api"
