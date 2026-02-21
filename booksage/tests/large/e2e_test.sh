#!/bin/bash
set -e

# BookSage E2E Test Script
echo "--- Starting BookSage E2E Test ---"

# 1. Clean up potential previous runs
docker compose down --volumes --remove-orphans

# 2. Build and start the services
echo "Building and starting services..."
docker compose up -d --build

# 3. Wait for services to be ready
echo "Waiting for services to initialize..."
# Wait for API to be up (simple port check)
TIMEOUT=60
COUNTER=0
until $(curl --output /dev/null --silent --head --fail http://localhost:8080); do
    if [ $COUNTER -gt $TIMEOUT ]; then
        echo "Timed out waiting for API"
        docker compose logs
        exit 1
    fi
    printf '.'
    sleep 2
    COUNTER=$((COUNTER+2))
done
echo "API is up!"

# 4. Run a verification test
echo "Running verification query..."
# We can use a simple curl to the API once we have endpoints, 
# or run a small go/python client.
# For now, we'll just check if we can reach the gRPC port of the worker too.
if nc -z localhost 50051; then
    echo "Worker gRPC is reachable!"
else
    echo "Worker gRPC is NOT reachable!"
    docker compose logs
    exit 1
fi

# 5. Clean up
echo "--- E2E Test Passed! ---"
docker compose down --volumes
