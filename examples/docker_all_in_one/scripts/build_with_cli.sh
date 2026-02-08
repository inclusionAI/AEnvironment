#!/bin/bash
set -e

# Colors for output
GREEN='\033[0;32m'
RED='\033[0;31m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

echo "========================================="
echo "  AEnvironment - Build with aenv CLI"
echo "========================================="
echo ""

# Navigate to weather-demo directory
cd "$(dirname "$0")/../weather-demo"

# Check if aenv CLI is installed
if ! command -v aenv &> /dev/null; then
    echo -e "${RED}Error: aenv CLI is not installed${NC}"
    echo ""
    echo "Please install it first:"
    echo "  cd ../../aenv"
    echo "  pip install -e ."
    echo ""
    echo "Or use docker build directly:"
    echo "  docker build -t aenv/weather-demo:1.0.0-docker ."
    exit 1
fi

echo -e "${BLUE}Building weather-demo using aenv CLI...${NC}"
echo ""

# Build using aenv CLI
aenv build

echo ""
echo -e "${GREEN}=========================================${NC}"
echo -e "${GREEN}  Build completed successfully!${NC}"
echo -e "${GREEN}=========================================${NC}"
echo ""
echo "Image built: aenv/weather-demo:1.0.0-docker"
echo ""
echo "Next steps:"
echo "  1. Start services: cd .. && ./scripts/start.sh"
echo "  2. Run demo: python3 weather-demo/run_demo.py"
echo ""
