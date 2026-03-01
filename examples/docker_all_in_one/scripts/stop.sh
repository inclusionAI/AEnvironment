#!/bin/bash
set -e

# Colors for output
GREEN='\033[0;32m'
RED='\033[0;31m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

echo "========================================="
echo "  AEnvironment Docker Mode - Shutdown"
echo "========================================="
echo ""

# Navigate to the script directory
cd "$(dirname "$0")/.."

# Stop services
echo "Stopping AEnvironment services..."
docker-compose down

echo -e "${GREEN}✓ Services stopped${NC}"

# Check if cleanup flag is provided
if [ "$1" == "--cleanup" ] || [ "$1" == "-c" ]; then
    echo ""
    echo "Cleaning up AEnv containers..."

    # Find and remove all containers with aenv.env_name label
    AENV_CONTAINERS=$(docker ps -aq --filter "label=aenv.env_name")

    if [ -n "$AENV_CONTAINERS" ]; then
        echo "Found $(echo $AENV_CONTAINERS | wc -w) AEnv container(s)"
        docker rm -f $AENV_CONTAINERS
        echo -e "${GREEN}✓ AEnv containers removed${NC}"
    else
        echo "No AEnv containers found"
    fi

    # Remove aenv-network if it exists and is not in use
    if docker network ls | grep -q aenv-network; then
        docker network rm aenv-network 2>/dev/null || echo -e "${YELLOW}Note: aenv-network is still in use or already removed${NC}"
    fi
fi

echo ""
echo -e "${GREEN}=========================================${NC}"
echo -e "${GREEN}  AEnvironment stopped successfully${NC}"
echo -e "${GREEN}=========================================${NC}"
echo ""

if [ "$1" != "--cleanup" ] && [ "$1" != "-c" ]; then
    echo "Tip: Use './scripts/stop.sh --cleanup' to also remove all AEnv containers"
    echo ""
fi
