#!/usr/bin/env bash
# Build and push an AEnvironment agent environment

# Enhanced error handling
set -euo pipefail

# Error handler function
error_handler() {
    local line_no=$1
    local command=$2
    local exit_code=${3:-$?}
    
    echo ""
    echo "❌ Error occurred at line $line_no"
    echo "   Command: $command"
    echo "   Exit code: $exit_code"
    echo ""
    echo "Debug information:"
    echo "   PWD: $(pwd)"
    echo "   ENV_DIR: ${ENV_DIR:-not set}"
    echo "   ENV_NAME: ${ENV_NAME:-not set}"
    echo ""
    
    # Show last few lines of output if available
    if [ -n "${BASH_COMMAND:-}" ]; then
        echo "   Failed command: ${BASH_COMMAND}"
    fi
    
    exit $exit_code
}

# Set trap to catch errors
trap 'error_handler ${LINENO} "${BASH_COMMAND}" $?' ERR

# Optional: Enable debug mode with DEBUG=1
if [ "${DEBUG:-0}" = "1" ]; then
    set -x
fi

if [ -z "$1" ]; then
    echo "Usage: $0 <environment-directory>"
    echo "Example: $0 temp/html-agent-66392481"
    exit 1
fi

ENV_DIR="$1"
ENV_NAME=$(basename "$ENV_DIR")
VERSION="1.0.0"

# Validate environment directory
if [ ! -d "$ENV_DIR" ]; then
    echo "❌ Error: Environment directory does not exist: $ENV_DIR"
    exit 1
fi

if [ ! -f "$ENV_DIR/config.json" ]; then
    echo "❌ Error: config.json not found in $ENV_DIR"
    exit 1
fi

if [ ! -f "$ENV_DIR/Dockerfile" ]; then
    echo "❌ Error: Dockerfile not found in $ENV_DIR"
    exit 1
fi

echo "📁 Environment directory: $ENV_DIR"
cd "$ENV_DIR" || {
    echo "❌ Failed to change directory to $ENV_DIR"
    exit 1
}

echo "🔨 Building environment: $ENV_NAME"

# Build and push
echo "Step 1: Building Docker image..."
echo "   Command: aenv build --push -n $ENV_NAME"
if ! aenv build --push -n "$ENV_NAME" 2>&1; then
    echo "❌ Build failed!"
    echo "   Please check:"
    echo "   1. Docker is running"
    echo "   2. You have permission to build Docker images"
    echo "   3. Network connectivity for pushing to registry"
    exit 1
fi

echo "Step 2: Pushing to registry..."
echo "   Command: aenv push"
if ! aenv push 2>&1; then
    echo "❌ Push failed!"
    echo "   Please check:"
    echo "   1. Network connectivity"
    echo "   2. Registry authentication"
    exit 1
fi

# Verify Docker image exists locally
echo "Step 3: Verifying local Docker image..."
IMAGE_NAME="reg.antgroup-inc.cn/aenv/${ENV_NAME}:${VERSION}"
echo "   Looking for image: ${IMAGE_NAME}"

if ! command -v docker &> /dev/null; then
    echo "❌ Error: docker command not found"
    exit 1
fi

if docker images --format "{{.Repository}}:{{.Tag}}" | grep -q "^${IMAGE_NAME}$"; then
    echo "✅ Local Docker image verified: ${IMAGE_NAME}"
else
    echo "⚠️  Warning: Local Docker image not found: ${IMAGE_NAME}"
    echo "   Attempting to pull..."
    if ! docker pull "${IMAGE_NAME}" 2>&1; then
        echo "❌ Failed to pull Docker image!"
        echo "   Image: ${IMAGE_NAME}"
        echo "   Please check:"
        echo "   1. Image was pushed successfully"
        echo "   2. Network connectivity"
        echo "   3. Registry authentication"
        exit 1
    fi
    echo "✅ Successfully pulled Docker image"
fi

# Verify environment is available in envhub
echo "Step 4: Verifying environment in envhub..."
echo "   Command: aenv get $ENV_NAME -v $VERSION"
if ! aenv get "$ENV_NAME" -v "$VERSION" 2>&1; then
    echo "❌ Failed to get environment from envhub!"
    echo "   Environment: ${ENV_NAME}@${VERSION}"
    echo "   Please check:"
    echo "   1. Environment was pushed successfully"
    echo "   2. Network connectivity"
    echo "   3. Environment name and version are correct"
    exit 1
fi
echo "✅ Environment verified in envhub"

# Final push to ensure sync
echo "Step 5: Final sync..."
if ! aenv push; then
    echo "⚠️  Warning: Final push failed, but environment may already be available"
fi

echo ""
echo "✅ Build and deployment complete: ${ENV_NAME}@${VERSION}"
echo ""
echo "Verification summary:"
echo "  ✅ Docker image: ${IMAGE_NAME}"
echo "  ✅ Environment: ${ENV_NAME}@${VERSION}"
echo ""
echo "Next steps:"
echo "1. Create a client script (copy scripts/client_template.py)"
echo "2. Update the client script with your environment name: ${ENV_NAME}@${VERSION}"
echo "3. Run: python3 client.py"

