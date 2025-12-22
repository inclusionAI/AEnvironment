#!/usr/bin/env bash
# Initialize AEnvironment agent project
# Usage: ./init_env.sh [workspace_path] [env_name_prefix]

set -e

WORKSPACE_PATH="${1:-/tmp}"
ENV_NAME_PREFIX="${2:-agent}"

cd "$WORKSPACE_PATH"
mkdir -p temp
cd temp

export ENV_IDR="${ENV_NAME_PREFIX}-$(openssl rand -hex 4)"
aenv init "$ENV_IDR"

echo "✅ Environment initialized: $ENV_IDR"
echo "📁 Project directory: $(pwd)/$ENV_IDR"
echo "🚀 Next steps:"
echo "   1. cd $ENV_IDR"
echo "   2. Edit src/custom_env.py to implement your tools"
echo "   3. Update requirements.txt with dependencies"
echo "   4. Run build_env.sh to build and deploy"

