#!/usr/bin/env bash
# Prerequisites checker for AEnvironment agent development
# Verifies all required dependencies are installed and configured
#
# Usage:
#   ./check_prerequisites.sh           # Full check with output
#   ./check_prerequisites.sh --quiet   # Silent mode, exit code only
#
# Exit codes:
#   0 - All prerequisites met
#   1 - One or more prerequisites missing

set -u  # Exit on undefined variable

# Color output helpers
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Quiet mode flag
QUIET_MODE=false
if [ "${1:-}" = "--quiet" ]; then
    QUIET_MODE=true
fi

# Print functions (respect quiet mode)
print_success() {
    if [ "$QUIET_MODE" = false ]; then
        echo -e "${GREEN}✅ $1${NC}"
    fi
}

print_error() {
    if [ "$QUIET_MODE" = false ]; then
        echo -e "${RED}❌ $1${NC}"
    fi
}

print_warning() {
    if [ "$QUIET_MODE" = false ]; then
        echo -e "${YELLOW}⚠️  $1${NC}"
    fi
}

print_info() {
    if [ "$QUIET_MODE" = false ]; then
        echo -e "${BLUE}ℹ️  $1${NC}"
    fi
}

print_header() {
    if [ "$QUIET_MODE" = false ]; then
        echo ""
        echo -e "${BLUE}═══════════════════════════════════════════════════════${NC}"
        echo -e "${BLUE}$1${NC}"
        echo -e "${BLUE}═══════════════════════════════════════════════════════${NC}"
        echo ""
    fi
}

# Track overall status
ALL_CHECKS_PASSED=true

# Start checks
if [ "$QUIET_MODE" = false ]; then
    print_header "🔍 AEnvironment Prerequisites Check"
fi

# Check 1: Python version
if [ "$QUIET_MODE" = false ]; then
    echo "Checking Python installation..."
fi

if command -v python3 &> /dev/null; then
    PYTHON_VERSION=$(python3 --version 2>&1 | awk '{print $2}')
    PYTHON_MAJOR=$(echo "$PYTHON_VERSION" | cut -d. -f1)
    PYTHON_MINOR=$(echo "$PYTHON_VERSION" | cut -d. -f2)

    if [ "$PYTHON_MAJOR" -ge 3 ] && [ "$PYTHON_MINOR" -ge 10 ]; then
        print_success "Python $PYTHON_VERSION (>= 3.10 required)"
    else
        print_error "Python $PYTHON_VERSION found, but 3.10+ required"
        print_info "Install Python 3.10 or later: https://www.python.org/downloads/"
        ALL_CHECKS_PASSED=false
    fi
else
    print_error "Python 3 not found"
    print_info "Install Python 3.10+: https://www.python.org/downloads/"
    ALL_CHECKS_PASSED=false
fi

# Check 2: pip
if [ "$QUIET_MODE" = false ]; then
    echo ""
    echo "Checking pip installation..."
fi

if command -v pip &> /dev/null || command -v pip3 &> /dev/null; then
    PIP_CMD=$(command -v pip3 &> /dev/null && echo "pip3" || echo "pip")
    PIP_VERSION=$($PIP_CMD --version 2>&1 | awk '{print $2}')
    print_success "pip $PIP_VERSION installed"
else
    print_error "pip not found"
    print_info "Install pip: python3 -m ensurepip --upgrade"
    ALL_CHECKS_PASSED=false
fi

# Check 3: AEnvironment (aenv command)
if [ "$QUIET_MODE" = false ]; then
    echo ""
    echo "Checking aenv CLI..."
fi

if command -v aenv &> /dev/null; then
    AENV_LOCATION=$(which aenv 2>/dev/null || echo "unknown")
    print_success "aenv command found at: $AENV_LOCATION"

    # Try to get version
    if aenv --version &> /dev/null; then
        AENV_VERSION=$(aenv --version 2>&1)
        print_info "Version: $AENV_VERSION"
    fi
else
    print_error "aenv command not found"
    print_info ""
    print_info "Install AEnvironment SDK:"
    print_info ""
    print_info "  Option A - From source (recommended):"
    print_info "    git clone https://github.com/inclusionAI/AEnvironment"
    print_info "    cd AEnvironment/aenv"
    print_info "    pip install -e ."
    print_info ""
    print_info "  Option B - From PyPI (when available):"
    print_info "    pip install aenvironment"
    print_info ""
    ALL_CHECKS_PASSED=false
fi

# Check 4: Docker
if [ "$QUIET_MODE" = false ]; then
    echo ""
    echo "Checking Docker..."
fi

if command -v docker &> /dev/null; then
    DOCKER_VERSION=$(docker --version 2>&1)
    print_success "Docker installed: $DOCKER_VERSION"

    # Check if Docker daemon is running
    if docker info &> /dev/null; then
        print_success "Docker daemon is running"
    else
        print_error "Docker daemon is not running"
        print_info "Start Docker Desktop or run: sudo systemctl start docker"
        ALL_CHECKS_PASSED=false
    fi
else
    print_error "Docker not found"
    print_info "Install Docker:"
    print_info "  - macOS: https://docs.docker.com/desktop/install/mac-install/"
    print_info "  - Linux: https://docs.docker.com/engine/install/"
    print_info "  - Windows: https://docs.docker.com/desktop/install/windows-install/"
    ALL_CHECKS_PASSED=false
fi

# Check 5: Git (optional but recommended)
if [ "$QUIET_MODE" = false ]; then
    echo ""
    echo "Checking Git (optional)..."
fi

if command -v git &> /dev/null; then
    GIT_VERSION=$(git --version 2>&1)
    print_success "Git installed: $GIT_VERSION"
else
    print_warning "Git not found (optional, but recommended)"
    print_info "Install Git: https://git-scm.com/downloads"
fi

# Summary
if [ "$QUIET_MODE" = false ]; then
    echo ""
    print_header "📊 Prerequisites Check Summary"
fi

if [ "$ALL_CHECKS_PASSED" = true ]; then
    if [ "$QUIET_MODE" = false ]; then
        print_success "All required prerequisites are met! ✨"
        echo ""
        print_info "You're ready to use the AEnvironment agent skill!"
        print_info ""
        print_info "Next steps:"
        echo "  1. Initialize a new agent:"
        echo "     bash .claude/skills/aenvironment-agent/scripts/init_env.sh"
        echo ""
        echo "  2. Follow the workflow in SKILL.md"
        echo ""
    fi
    exit 0
else
    if [ "$QUIET_MODE" = false ]; then
        print_error "Some prerequisites are missing"
        echo ""
        print_info "Please install the missing dependencies listed above"
        print_info "Then run this check again to verify"
        echo ""
    fi
    exit 1
fi
