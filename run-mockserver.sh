#!/bin/bash

# AutoMock Local MockServer Runner
# This script starts MockServer locally with Docker using your generated configuration

set -e

PROJECT_DIR="/Users/hemantobora/Desktop/Projects/auto-mock"
CONFIG_FILE="${PROJECT_DIR}/imei-expectations.json"
CONTAINER_NAME="automock-local-server"
MOCKSERVER_PORT="1080"

echo "🤖 AutoMock Local MockServer Runner"
echo "=================================="

# Function to check if Docker is running
check_docker() {
    if ! docker info >/dev/null 2>&1; then
        echo "❌ Docker is not running. Please start Docker and try again."
        exit 1
    fi
    echo "✅ Docker is running"
}

# Function to stop existing container
stop_existing() {
    if docker ps -q -f name=${CONTAINER_NAME} | grep -q .; then
        echo "🛑 Stopping existing MockServer container..."
        docker stop ${CONTAINER_NAME} >/dev/null 2>&1
        docker rm ${CONTAINER_NAME} >/dev/null 2>&1
        echo "✅ Existing container stopped"
    fi
}

# Function to start MockServer
start_mockserver() {
    echo "🚀 Starting MockServer with your IMEI configuration..."
    
    docker run -d \
        --name ${CONTAINER_NAME} \
        -p ${MOCKSERVER_PORT}:1080 \
        -v "${CONFIG_FILE}:/config/expectations.json:ro" \
        -e MOCKSERVER_INITIALIZATION_JSON_PATH=/config/expectations.json \
        -e MOCKSERVER_LOG_LEVEL=INFO \
        -e MOCKSERVER_WATCH_INITIALIZATION_JSON=true \
        mockserver/mockserver:5.15.0
    
    echo "⏳ Waiting for MockServer to start..."
    sleep 3
    
    # Health check
    for i in {1..10}; do
        if curl -s http://localhost:${MOCKSERVER_PORT}/mockserver/status >/dev/null 2>&1; then
            echo "✅ MockServer is ready!"
            return 0
        fi
        echo "⏳ Waiting... (attempt $i/10)"
        sleep 2
    done
    
    echo "❌ MockServer failed to start. Checking logs..."
    docker logs ${CONTAINER_NAME}
    return 1
}

# Function to test the IMEI endpoint
test_imei_endpoint() {
    echo ""
    echo "🧪 Testing your IMEI endpoint..."
    echo "================================"
    
    # Test the POST endpoint
    echo "📤 Testing: POST /api/v1/imei"
    response=$(curl -s -w "\nHTTP_STATUS:%{http_code}" \
        -X POST \
        -H "Content-Type: application/json" \
        -d '{"imei": "1234567890"}' \
        "http://localhost:${MOCKSERVER_PORT}/api/v1/imei")
    
    # Extract body and status
    body=$(echo "$response" | sed '$d')
    status=$(echo "$response" | tail -n1 | sed 's/.*HTTP_STATUS://')
    
    echo "📥 Response Status: $status"
    echo "📥 Response Body: $body"
    
    if [ "$status" = "200" ]; then
        echo "✅ IMEI endpoint working correctly!"
    else
        echo "❌ Unexpected status: $status"
    fi
    
    echo ""
    echo "🔄 Try different IMEI values:"
    echo "curl -X POST http://localhost:${MOCKSERVER_PORT}/api/v1/imei \\"
    echo "  -H \"Content-Type: application/json\" \\"
    echo "  -d '{\"imei\": \"9876543210\"}'"
}

# Main execution
main() {
    check_docker
    
    # Check if config file exists
    if [[ ! -f "$CONFIG_FILE" ]]; then
        echo "❌ Configuration file not found: $CONFIG_FILE"
        echo "💡 Make sure you've run your AutoMock CLI to generate the configuration"
        exit 1
    fi
    
    echo "📁 Using config: $CONFIG_FILE"
    
    stop_existing
    
    if start_mockserver; then
        echo ""
        echo "🌐 MockServer URLs:"
        echo "=================="
        echo "📊 Dashboard: http://localhost:${MOCKSERVER_PORT}/mockserver/dashboard"
        echo "🔗 API Base: http://localhost:${MOCKSERVER_PORT}"
        echo "📋 Status: http://localhost:${MOCKSERVER_PORT}/mockserver/status"
        echo "🎯 Your IMEI API: http://localhost:${MOCKSERVER_PORT}/api/v1/imei"
        
        # Test the endpoint
        test_imei_endpoint
        
        echo ""
        echo "📚 What you can do now:"
        echo "======================"
        echo "1. Test your API with curl or Postman"
        echo "2. View request logs in the dashboard"
        echo "3. Add more expectations by updating the JSON file and restarting"
        echo ""
        echo "🛑 To stop MockServer:"
        echo "   $0 stop"
        echo ""
        echo "📄 To view logs:"
        echo "   $0 logs"
        
    else
        exit 1
    fi
}

# Handle script arguments
case "${1:-}" in
    "stop")
        stop_existing
        echo "✅ MockServer stopped"
        ;;
    "logs")
        if docker ps -q -f name=${CONTAINER_NAME} | grep -q .; then
            echo "📄 MockServer logs:"
            docker logs -f ${CONTAINER_NAME}
        else
            echo "❌ MockServer is not running"
        fi
        ;;
    "restart")
        stop_existing
        main
        ;;
    "status")
        if docker ps -q -f name=${CONTAINER_NAME} | grep -q .; then
            echo "✅ MockServer is running"
            echo "📊 Status endpoint:"
            curl -s http://localhost:${MOCKSERVER_PORT}/mockserver/status | python3 -m json.tool 2>/dev/null || curl -s http://localhost:${MOCKSERVER_PORT}/mockserver/status
        else
            echo "❌ MockServer is not running"
            echo "💡 Run '$0' to start it"
        fi
        ;;
    "test")
        if docker ps -q -f name=${CONTAINER_NAME} | grep -q .; then
            test_imei_endpoint
        else
            echo "❌ MockServer is not running"
            echo "💡 Run '$0' to start it first"
        fi
        ;;
    *)
        main
        ;;
esac