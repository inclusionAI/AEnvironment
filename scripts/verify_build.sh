#!/bin/bash
# Quick verification script for Docker Engine support build

set -e

echo "================================================"
echo "Docker Engine Support - Build Verification"
echo "================================================"
echo ""

# Colors
GREEN='\033[0;32m'
RED='\033[0;31m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# Function to print status
print_status() {
    if [ $? -eq 0 ]; then
        echo -e "${GREEN}✓${NC} $1"
    else
        echo -e "${RED}✗${NC} $1"
        exit 1
    fi
}

# Step 1: Check Go installation
echo "Step 1: Checking Go installation..."
if command -v go &> /dev/null; then
    GO_VERSION=$(go version | awk '{print $3}')
    echo -e "${GREEN}✓${NC} Go installed: $GO_VERSION"
else
    echo -e "${RED}✗${NC} Go not found. Please install Go 1.19+"
    exit 1
fi
echo ""

# Step 2: Build Controller
echo "Step 2: Building Controller..."
cd controller
go build -o bin/controller ./cmd/main.go
print_status "Controller built successfully"
CONTROLLER_SIZE=$(du -h bin/controller | awk '{print $1}')
echo "   Size: $CONTROLLER_SIZE"
echo ""

# Step 3: Build API Service
echo "Step 3: Building API Service..."
cd ../api-service
go build -o bin/api-service ./main.go
print_status "API Service built successfully"
API_SIZE=$(du -h bin/api-service | awk '{print $1}')
echo "   Size: $API_SIZE"
echo ""

# Step 4: Verify binaries
echo "Step 4: Verifying binaries..."
cd ..
if [ -f "controller/bin/controller" ] && [ -x "controller/bin/controller" ]; then
    echo -e "${GREEN}✓${NC} Controller binary is executable"
else
    echo -e "${RED}✗${NC} Controller binary is not executable"
    exit 1
fi

if [ -f "api-service/bin/api-service" ] && [ -x "api-service/bin/api-service" ]; then
    echo -e "${GREEN}✓${NC} API Service binary is executable"
else
    echo -e "${RED}✗${NC} API Service binary is not executable"
    exit 1
fi
echo ""

# Step 5: Check Docker availability (optional)
echo "Step 5: Checking Docker availability..."
if command -v docker &> /dev/null; then
    DOCKER_VERSION=$(docker version --format '{{.Server.Version}}' 2>/dev/null || echo "unavailable")
    if [ "$DOCKER_VERSION" != "unavailable" ]; then
        echo -e "${GREEN}✓${NC} Docker daemon is running (version: $DOCKER_VERSION)"
    else
        echo -e "${YELLOW}⚠${NC} Docker client installed but daemon not running"
    fi
else
    echo -e "${YELLOW}⚠${NC} Docker not installed (optional for build, required for runtime)"
fi
echo ""

# Step 6: Verify new files exist
echo "Step 6: Verifying implementation files..."
FILES=(
    "api-service/service/docker_client.go"
    "controller/pkg/aenvhub_http_server/aenv_docker_handler.go"
    "controller/pkg/aenvhub_http_server/aenv_docker_cache.go"
    "controller/pkg/aenvhub_http_server/aenv_docker_compose.go"
    "controller/pkg/aenvhub_http_server/aenv_http_types.go"
    "controller/pkg/model/docker_config.go"
)

for file in "${FILES[@]}"; do
    if [ -f "$file" ]; then
        echo -e "${GREEN}✓${NC} $file"
    else
        echo -e "${RED}✗${NC} $file (missing)"
        exit 1
    fi
done
echo ""

# Step 7: Quick syntax check for key features
echo "Step 7: Checking key implementation features..."

# Check if Docker handler is registered
if grep -q "dockerHandler" controller/cmd/main.go; then
    echo -e "${GREEN}✓${NC} Docker handler registered in main.go"
else
    echo -e "${RED}✗${NC} Docker handler not found in main.go"
fi

# Check if Docker client is in API service
if grep -q "DockerClient" api-service/main.go; then
    echo -e "${GREEN}✓${NC} Docker client integrated in API service"
else
    echo -e "${RED}✗${NC} Docker client not found in API service"
fi

# Check Helm charts updated
if grep -q "docker:" deploy/controller/values.yaml; then
    echo -e "${GREEN}✓${NC} Docker configuration in Helm values"
else
    echo -e "${YELLOW}⚠${NC} Docker configuration not found in Helm values"
fi
echo ""

# Summary
echo "================================================"
echo -e "${GREEN}✓ BUILD VERIFICATION COMPLETE${NC}"
echo "================================================"
echo ""
echo "Build Summary:"
echo "  - Controller: $CONTROLLER_SIZE"
echo "  - API Service: $API_SIZE"
echo "  - Docker support: ENABLED"
echo ""
echo "Next Steps:"
echo "  1. Run integration tests: see docs/DOCKER_ENGINE_TESTING.md"
echo "  2. Start Controller: cd controller && ENGINE_TYPE=docker ./bin/controller --leader-elect=false"
echo "  3. Start API Service: cd api-service && ./bin/api-service --schedule-type=docker"
echo ""
