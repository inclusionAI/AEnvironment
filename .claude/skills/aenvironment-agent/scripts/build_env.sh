#!/usr/bin/env bash
# Build and deploy AEnvironment agent with comprehensive validation
# Usage: ./build_env.sh <env_dir> [version]
#
# This script:
# - Validates environment directory structure
# - Builds Docker image with 'aenv build --push'
# - Pushes to environment hub with 'aenv push'
# - Verifies deployment with 'aenv get'
# - Reports detailed success/failure status

set -e  # Exit on error
set -u  # Exit on undefined variable

# Color output helpers
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Print functions
print_success() { echo -e "${GREEN}✅ $1${NC}"; }
print_error() { echo -e "${RED}❌ $1${NC}"; }
print_warning() { echo -e "${YELLOW}⚠️  $1${NC}"; }
print_info() { echo -e "${BLUE}ℹ️  $1${NC}"; }
print_step() { echo -e "${BLUE}🔨 $1${NC}"; }

# Get script directory for prerequisite check
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

# Usage function
usage() {
    echo "Usage: $0 <env_dir> [version]"
    echo ""
    echo "Arguments:"
    echo "  env_dir   Path to environment directory (e.g., temp/agent-XXXX)"
    echo "  version   Version to deploy (default: 1.0.0)"
    echo ""
    echo "Example:"
    echo "  $0 temp/agent-1234 1.0.0"
    exit 1
}

# Check prerequisites before proceeding
print_info "Checking prerequisites..."
if ! bash "$SCRIPT_DIR/check_prerequisites.sh" --quiet; then
    print_error "Prerequisites check failed. Please install missing dependencies:"
    echo ""
    bash "$SCRIPT_DIR/check_prerequisites.sh"
    exit 1
fi
print_success "Prerequisites check passed"
echo ""

# Parse arguments
ENV_DIR="${1:-}"
VERSION="${2:-1.0.0}"

# Validate arguments
if [ -z "$ENV_DIR" ]; then
    print_error "Environment directory not specified"
    usage
fi

if [ ! -d "$ENV_DIR" ]; then
    print_error "Environment directory does not exist: $ENV_DIR"
    exit 1
fi

# Extract environment name from path
ENV_NAME=$(basename "$ENV_DIR")

print_info "Building and deploying AEnvironment agent"
print_info "Environment: $ENV_NAME"
print_info "Directory: $ENV_DIR"
print_info "Version: $VERSION"
echo ""

# Validate essential files
print_step "Validating environment structure..."
ESSENTIAL_FILES=("config.json" "Dockerfile" "requirements.txt" "src/custom_env.py")
VALIDATION_FAILED=0

for file in "${ESSENTIAL_FILES[@]}"; do
    if [ ! -f "$ENV_DIR/$file" ]; then
        print_error "Missing required file: $file"
        VALIDATION_FAILED=1
    else
        print_success "Found: $file"
    fi
done

if [ $VALIDATION_FAILED -eq 1 ]; then
    print_error "Environment validation failed"
    print_info "Make sure you initialized the environment with init_env.sh"
    exit 1
fi

echo ""

# Navigate to environment directory
cd "$ENV_DIR" || exit 1
print_success "Changed to environment directory"

# Build and push Docker image to registry
print_step "Building and pushing Docker image..."
print_info "This will build the image and push to Docker registry"
if ! aenv build --push; then
    print_error "Docker build or push failed"
    print_info "Check Dockerfile and requirements.txt for errors"
    print_info "Ensure Docker daemon is running: docker info"
    print_info "Verify registry credentials are configured"
    print_info "Review build logs above for specific error messages"
    exit 1
fi
print_success "Docker image built and pushed to registry"

echo ""

# Register environment in hub
print_step "Registering environment in AEnvironment hub..."
if ! aenv push; then
    print_error "Environment registration failed"
    print_info "Check network connectivity and hub access"
    exit 1
fi
print_success "Environment registered in hub"

echo ""

# Verify deployment
print_step "Verifying deployment..."
print_info "Waiting for registry sync (this may take 30-60 seconds)..."
sleep 5

MAX_RETRIES=5
RETRY_COUNT=0
VERIFY_SUCCESS=0

while [ $RETRY_COUNT -lt $MAX_RETRIES ]; do
    if aenv get "$ENV_NAME" -v "$VERSION" &> /dev/null; then
        VERIFY_SUCCESS=1
        break
    fi

    RETRY_COUNT=$((RETRY_COUNT + 1))
    if [ $RETRY_COUNT -lt $MAX_RETRIES ]; then
        print_warning "Verification attempt $RETRY_COUNT failed, retrying..."
        sleep 10
    fi
done

if [ $VERIFY_SUCCESS -eq 0 ]; then
    print_error "Deployment verification failed after $MAX_RETRIES attempts"
    print_info "Environment may still be syncing. Try manually:"
    print_info "  aenv get $ENV_NAME -v $VERSION"
    exit 1
fi

print_success "Deployment verified successfully"

# Final push (as per original script)
if ! aenv push; then
    print_warning "Final push failed, but deployment is verified"
fi

echo ""
echo "═══════════════════════════════════════════════════════"
print_success "Build and deployment completed successfully!"
echo "═══════════════════════════════════════════════════════"
print_info "Environment name: ${ENV_NAME}@${VERSION}"
print_info "Ready to use in client applications"
echo ""
print_info "📋 Next steps:"
echo "   1. Test deployment:"
echo "      aenv get $ENV_NAME -v $VERSION"
echo "   2. Create client application using:"
echo "      Environment(\"${ENV_NAME}@${VERSION}\")"
echo ""
print_success "Done!"

