#!/bin/bash
# AutoMock Project Cleanup Script
# Removes redundant files and keeps only essential ECS Fargate infrastructure

echo "🧹 Cleaning up AutoMock project..."

# Remove old documentation files (keep main README.md and GETTING_STARTED.md)
rm -f BRANCH_2_FIXES.md
rm -f BRANCH_2_IMPLEMENTATION.md  
rm -f INTERACTIVE_MOCK_FIXES.md

# Remove test files
rm -f test_core.go
rm -f test_new_approach.go

# Remove old example configuration files (keep imei-expectations.json as example)
rm -f commerce-expectations.json
rm -f commerce-mock-config.json

# Remove old Lambda deployment code (replaced with ECS)
rm -rf internal/deployer/
rm -rf cmd/lambda/

# Remove empty generator directory if it exists
rm -rf internal/generator/

# Remove .DS_Store files
find . -name ".DS_Store" -delete

echo "✅ Cleanup completed!"
echo ""
echo "📁 Remaining structure:"
echo "   ├── README.md"
echo "   ├── GETTING_STARTED.md" 
echo "   ├── cmd/auto-mock/           # Main CLI"
echo "   ├── internal/"
echo "   │   ├── cloud/               # Multi-cloud abstraction"
echo "   │   ├── mcp/                 # AI integration"
echo "   │   ├── provider/            # Provider interface"
echo "   │   ├── repl/                # Interactive CLI"
echo "   │   ├── state/               # State management"
echo "   │   └── utils/               # Utilities"
echo "   ├── terraform/               # ECS Fargate infrastructure"
echo "   │   ├── modules/automock-ecs/"
echo "   │   └── main.tf"
echo "   ├── docker-compose.yml       # Local development"
echo "   ├── run-mockserver.sh        # Local setup script"
echo "   └── imei-expectations.json   # Example config"
