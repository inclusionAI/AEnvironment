#!/bin/bash
# Quick validation script for docker_all_in_one example

echo "Validating docker_all_in_one example structure..."
echo ""

ERRORS=0

# Check required files
echo "Checking required files..."
FILES=(
    "README.md"
    "docker-compose.yml"
    "env.example"
    ".gitignore"
    "scripts/start.sh"
    "scripts/stop.sh"
    "scripts/demo.sh"
    "weather-demo/Dockerfile"
    "weather-demo/requirements.txt"
    "weather-demo/config.json"
    "weather-demo/run_demo.py"
    "weather-demo/src/__init__.py"
    "weather-demo/src/custom_env.py"
    "docs/architecture.md"
)

for file in "${FILES[@]}"; do
    if [ -f "$file" ]; then
        echo "  ✓ $file"
    else
        echo "  ✗ $file (missing)"
        ERRORS=$((ERRORS + 1))
    fi
done

echo ""

# Check executable permissions
echo "Checking executable permissions..."
SCRIPTS=(
    "scripts/start.sh"
    "scripts/stop.sh"
    "scripts/demo.sh"
    "weather-demo/run_demo.py"
)

for script in "${SCRIPTS[@]}"; do
    if [ -x "$script" ]; then
        echo "  ✓ $script"
    else
        echo "  ✗ $script (not executable)"
        ERRORS=$((ERRORS + 1))
    fi
done

echo ""

# Validate docker-compose.yml syntax
echo "Validating docker-compose.yml syntax..."
if command -v docker-compose &> /dev/null; then
    if docker-compose config > /dev/null 2>&1; then
        echo "  ✓ docker-compose.yml syntax is valid"
    else
        echo "  ✗ docker-compose.yml has syntax errors"
        ERRORS=$((ERRORS + 1))
    fi
else
    echo "  ⚠ docker-compose not found, skipping validation"
fi

echo ""

# Check Python syntax
echo "Validating Python files..."
PYTHON_FILES=(
    "weather-demo/run_demo.py"
    "weather-demo/src/custom_env.py"
)

for pyfile in "${PYTHON_FILES[@]}"; do
    if python3 -m py_compile "$pyfile" 2>/dev/null; then
        echo "  ✓ $pyfile"
    else
        echo "  ✗ $pyfile (syntax error)"
        ERRORS=$((ERRORS + 1))
    fi
done

echo ""

# Summary
if [ $ERRORS -eq 0 ]; then
    echo "========================================="
    echo "  ✓ All validations passed!"
    echo "========================================="
    echo ""
    echo "Quick start:"
    echo "  cd examples/docker_all_in_one"
    echo "  ./scripts/start.sh"
    exit 0
else
    echo "========================================="
    echo "  ✗ Found $ERRORS error(s)"
    echo "========================================="
    exit 1
fi
