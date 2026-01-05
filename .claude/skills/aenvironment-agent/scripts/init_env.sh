#!/usr/bin/env bash
# Initialize AEnvironment agent project with proper error handling
# Usage: ./init_env.sh [workspace_path] [env_name_prefix]
#
# This script:
# - Creates a new environment project with unique ID
# - Generates complete project structure using 'aenv init'
# - Provides clear feedback and next steps

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

# Get script directory for prerequisite check
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

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

# Parse arguments with defaults
WORKSPACE_PATH="${1:-$(pwd)}"
ENV_NAME_PREFIX="${2:-agent}"

# Validate workspace path
if [ ! -d "$WORKSPACE_PATH" ]; then
    print_error "Workspace path does not exist: $WORKSPACE_PATH"
    exit 1
fi

# Check if aenv is installed
if ! command -v aenv &> /dev/null; then
    print_error "aenv command not found"
    print_info "Install it with: pip install aenvironment"
    exit 1
fi

print_info "Initializing AEnvironment agent project..."
print_info "Workspace: $WORKSPACE_PATH"
print_info "Name prefix: $ENV_NAME_PREFIX"

# Navigate to workspace
cd "$WORKSPACE_PATH" || exit 1

# Create temp directory if it doesn't exist
mkdir -p temp
cd temp || exit 1

# Generate unique environment ID
if ! ENV_IDR="${ENV_NAME_PREFIX}-$(openssl rand -hex 4)"; then
    print_error "Failed to generate environment ID"
    exit 1
fi

print_info "Environment ID: $ENV_IDR"

# Initialize environment with aenv
print_info "Running: aenv init $ENV_IDR"
if ! aenv init "$ENV_IDR"; then
    print_error "aenv init failed"
    print_info "Make sure aenv is properly installed and configured"
    exit 1
fi

# Verify directory was created
if [ ! -d "$ENV_IDR" ]; then
    print_error "Environment directory was not created: $ENV_IDR"
    exit 1
fi

# Verify essential files exist
ESSENTIAL_FILES=("config.json" "Dockerfile" "requirements.txt" "src/custom_env.py")
for file in "${ESSENTIAL_FILES[@]}"; do
    if [ ! -f "$ENV_IDR/$file" ]; then
        print_warning "Expected file not found: $file"
    fi
done

# Success message
print_success "Environment initialized successfully: $ENV_IDR"
print_info "Project directory: $(pwd)/$ENV_IDR"
echo ""
print_info "📋 Next steps:"
echo "   1. cd temp/$ENV_IDR"
echo "   2. Edit src/custom_env.py to implement your agent tools"
echo "   3. Update requirements.txt with dependencies (e.g., openai>=1.0.0)"
echo "   4. Optionally update Dockerfile base image"
echo "   5. Build and deploy:"
echo "      bash .claude/skills/aenvironment-agent/scripts/build_env.sh temp/$ENV_IDR"
echo ""
print_success "Ready to develop!"

