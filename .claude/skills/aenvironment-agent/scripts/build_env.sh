#!/usr/bin/env bash
# Build and deploy AEnvironment agent
# Usage: ./build_env.sh [env_idr] [version]

set -e

ENV_IDR="${1:-${ENV_IDR}}"
VERSION="${2:-1.0.0}"

if [ -z "$ENV_IDR" ]; then
    echo "❌ Error: ENV_IDR not set. Please provide environment ID or set ENV_IDR variable."
    exit 1
fi

echo "🔨 Building environment: $ENV_IDR"
echo "📦 Version: $VERSION"

# Build and push
aenv build --push -n "$ENV_IDR"
aenv push

# Verify
echo "✅ Verifying deployment..."
aenv get "$ENV_IDR" -v "$VERSION"
aenv push

echo "✅ Build and deployment completed successfully!"
echo "🚀 Environment available as: $ENV_IDR@$VERSION"

