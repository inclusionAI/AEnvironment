#!/bin/bash
set -e

# Colors for output
GREEN='\033[0;32m'
RED='\033[0;31m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

echo "========================================="
echo "  AEnvironment Docker Mode - Full Demo"
echo "========================================="
echo ""

# Navigate to the script directory
cd "$(dirname "$0")/.."

# Step 1: Start services
echo -e "${BLUE}[Step 1/5] Starting AEnvironment services...${NC}"
./scripts/start.sh
echo ""

# Step 2: Build weather-demo image
echo -e "${BLUE}[Step 2/5] Building weather-demo Docker image...${NC}"
cd weather-demo

# Check if aenv CLI is available
if command -v aenv &> /dev/null; then
    echo "Using aenv CLI to build image..."
    aenv build
    echo -e "${GREEN}✓ Image built using aenv CLI${NC}"
else
    echo "aenv CLI not found, using docker build..."
    docker build -t aenv/weather-demo:1.0.0-docker .
    echo -e "${GREEN}✓ Image built: aenv/weather-demo:1.0.0-docker${NC}"
fi

cd ..
echo ""

# Step 3: Run demo client
echo -e "${BLUE}[Step 3/5] Running demo client...${NC}"
echo ""
python3 weather-demo/run_demo.py
echo ""

# Step 4: Show container status
echo -e "${BLUE}[Step 4/5] Checking AEnv containers...${NC}"
echo ""
AENV_CONTAINERS=$(docker ps -a --filter "label=aenv.env_name" --format "table {{.ID}}\t{{.Image}}\t{{.Status}}\t{{.Names}}\t{{.Labels}}")

if [ -n "$AENV_CONTAINERS" ]; then
    echo "$AENV_CONTAINERS"
else
    echo "No AEnv containers found (they may have been cleaned up)"
fi
echo ""

# Step 5: Show logs
echo -e "${BLUE}[Step 5/5] Recent Controller logs:${NC}"
echo ""
docker-compose logs --tail=20 controller
echo ""

# Prompt for cleanup
echo -e "${YELLOW}=========================================${NC}"
echo -e "${YELLOW}  Demo completed!${NC}"
echo -e "${YELLOW}=========================================${NC}"
echo ""
echo "What would you like to do?"
echo "  1. View full logs (docker-compose logs -f)"
echo "  2. Stop services (./scripts/stop.sh)"
echo "  3. Stop and cleanup (./scripts/stop.sh --cleanup)"
echo "  4. Keep services running"
echo ""
read -p "Enter choice [1-4] (default: 4): " choice

case $choice in
    1)
        docker-compose logs -f
        ;;
    2)
        ./scripts/stop.sh
        ;;
    3)
        ./scripts/stop.sh --cleanup
        ;;
    *)
        echo ""
        echo "Services are still running."
        echo "  - Controller:  http://localhost:9090"
        echo "  - API Service: http://localhost:8080"
        echo ""
        echo "Stop them later with: ./scripts/stop.sh"
        ;;
esac

echo ""
echo "Thank you for trying AEnvironment Docker Engine mode!"
echo ""
