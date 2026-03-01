#!/bin/bash
set -e

# Colors for output
GREEN='\033[0;32m'
RED='\033[0;31m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

echo "========================================="
echo "  AEnvironment Docker Mode - Startup"
echo "========================================="
echo ""

# Check if Docker daemon is running
echo -n "Checking Docker daemon... "
if ! docker info > /dev/null 2>&1; then
    echo -e "${RED}✗${NC}"
    echo "Error: Docker daemon is not running. Please start Docker first."
    exit 1
fi
echo -e "${GREEN}✓${NC}"

# Navigate to the script directory
cd "$(dirname "$0")/.."

# Copy env.example to .env if it doesn't exist
if [ ! -f ".env" ]; then
    echo -n "Creating .env file from env.example... "
    cp env.example .env
    echo -e "${GREEN}✓${NC}"
fi

# Build Controller and API Service images if they don't exist
echo ""
echo "Checking Docker images..."

# Note: Controller and API Service Dockerfiles require building from project root
# because they depend on go.work and multiple modules (controller, api-service, envhub)

if ! docker images | grep -q "aenv-controller"; then
    echo -e "${YELLOW}Warning: aenv-controller:latest image not found${NC}"
    echo "Building Controller image from project root..."
    cd ../..
    docker build -f controller/Dockerfile -t aenv-controller:latest .
    cd examples/docker_all_in_one
    echo -e "${GREEN}✓ Controller image built${NC}"
fi

if ! docker images | grep -q "aenv-api-service"; then
    echo -e "${YELLOW}Warning: aenv-api-service:latest image not found${NC}"
    echo "Building API Service image from project root..."
    cd ../..
    docker build -f api-service/Dockerfile -t aenv-api-service:latest .
    cd examples/docker_all_in_one
    echo -e "${GREEN}✓ API Service image built${NC}"
fi

# Start services
echo ""
echo "Starting AEnvironment services..."
docker-compose up -d

# Wait for services to be healthy
echo ""
echo -n "Waiting for Controller to be ready"
for i in {1..30}; do
    if curl -s http://localhost:9090/health > /dev/null 2>&1; then
        echo -e " ${GREEN}✓${NC}"
        break
    fi
    echo -n "."
    sleep 1
    if [ $i -eq 30 ]; then
        echo -e " ${RED}✗${NC}"
        echo "Error: Controller failed to start within 30 seconds"
        docker-compose logs controller
        exit 1
    fi
done

echo -n "Waiting for API Service to be ready"
for i in {1..30}; do
    if curl -s http://localhost:8080/health > /dev/null 2>&1; then
        echo -e " ${GREEN}✓${NC}"
        break
    fi
    echo -n "."
    sleep 1
    if [ $i -eq 30 ]; then
        echo -e " ${RED}✗${NC}"
        echo "Error: API Service failed to start within 30 seconds"
        docker-compose logs api-service
        exit 1
    fi
done

echo ""
echo -e "${GREEN}=========================================${NC}"
echo -e "${GREEN}  AEnvironment is ready!${NC}"
echo -e "${GREEN}=========================================${NC}"
echo ""
echo "Service URLs:"
echo "  - Controller:   http://localhost:9090"
echo "  - API Service:  http://localhost:8080"
echo ""
echo "Test the setup:"
echo "  curl http://localhost:9090/health"
echo "  curl http://localhost:8080/health"
echo ""
echo "View logs:"
echo "  docker-compose logs -f"
echo ""
echo "Stop services:"
echo "  ./scripts/stop.sh"
echo ""
