#!/usr/bin/env bash
# Initialize a new AEnvironment agent environment

# Enhanced error handling
set -eo pipefail

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
    echo "   USER: ${USER:-unknown}"
    echo ""
    
    # Show last few lines of output if available
    if [ -n "${BASH_COMMAND:-}" ]; then
        echo "   Failed command: ${BASH_COMMAND}"
    fi
    
    exit $exit_code
}

# Set trap to catch errors
# Ensure variables have defaults to avoid issues with set -u
trap 'error_handler "${LINENO:-0}" "${BASH_COMMAND:-unknown}" "${?}"' ERR

# Optional: Enable debug mode with DEBUG=1
if [ "${DEBUG:-0}" = "1" ]; then
    set -x
fi

# Create temp directory if it doesn't exist (needed for venv creation)
mkdir -p temp
cd temp

# Create Python virtual environment
echo "🐍 Creating Python virtual environment..."
if [ -d "venv" ]; then
    echo "⚠️  Virtual environment already exists, reusing it"
else
    if ! python3 -m venv venv 2>&1; then
        echo "❌ Failed to create Python virtual environment"
        echo "   Please ensure python3 is installed and accessible"
        exit 1
    fi
    echo "✅ Python virtual environment created"
fi

# Activate virtual environment
echo "🔌 Activating virtual environment..."
if [ ! -f "venv/bin/activate" ]; then
    echo "❌ Virtual environment activation script not found: venv/bin/activate"
    exit 1
fi
if ! source venv/bin/activate 2>&1; then
    echo "❌ Failed to activate virtual environment"
    exit 1
fi
echo "✅ Virtual environment activated"

# Check if aenv can be imported in virtual environment
if ! python -c "import aenv" 2>/dev/null; then
    echo "⚠️  aenvironment not found in virtual environment. Installing aenvironment..."
    if ! pip install aenvironment 2>&1; then
        echo "❌ Failed to install aenvironment. Please install it manually:"
        echo "   source venv/bin/activate"
        echo "   pip install aenvironment"
        exit 1
    fi
    echo "✅ aenvironment installed successfully in virtual environment"
else
    echo "✅ aenvironment is already installed in virtual environment"
fi

# Generate unique environment ID
echo "🔑 Generating unique environment ID..."
if ! command -v openssl &> /dev/null; then
    echo "❌ openssl command not found. Cannot generate unique ID."
    echo "   Please install openssl or set ENV_IDR manually"
    exit 1
fi
export ENV_IDR="agent-$(openssl rand -hex 4)"
if [ -z "$ENV_IDR" ]; then
    echo "❌ Failed to generate environment ID"
    exit 1
fi
echo "   Generated ID: $ENV_IDR"

# Initialize environment
echo "🚀 Initializing environment: $ENV_IDR"
# Try aenv command first, fallback to python -m aenv (using venv python)
if command -v aenv &> /dev/null; then
    if ! aenv init "$ENV_IDR" 2>&1; then
        echo "❌ Failed to initialize environment with 'aenv init'"
        exit 1
    fi
elif python -m aenv --help &> /dev/null 2>&1; then
    if ! python -m aenv init "$ENV_IDR" 2>&1; then
        echo "❌ Failed to initialize environment with 'python -m aenv init'"
        exit 1
    fi
else
    echo "❌ Error: aenv command not found. Please ensure aenvironment is properly installed."
    echo "   Try: source venv/bin/activate && pip install aenvironment"
    exit 1
fi

echo "✅ Environment initialized: $ENV_IDR"
echo "📁 Location: $(pwd)/$ENV_IDR"
echo ""
echo "Next steps:"
echo "1. Activate virtual environment: source venv/bin/activate"
echo "2. Edit config.json to configure your environment"
echo "3. Edit src/custom_env.py to register your tools"
echo "4. Run: ../scripts/build_env.sh $(pwd)/$ENV_IDR"
