# Contributing to AEnvironment

Thank you for your interest in contributing to AEnvironment! We welcome contributions from
everyone, whether you're fixing bugs, improving documentation, adding new features, or
helping with code reviews. This guide will help you get started.

## Table of Contents

- [Quick Start](#quick-start)
- [Project Structure](#project-structure)
- [Development Setup](#development-setup)
- [Ways to Contribute](#ways-to-contribute)
- [Coding Standards](#coding-standards)
- [Testing](#testing)
- [CI/CD](#cicd)
- [Pull Request Guidelines](#pull-request-guidelines)

## Quick Start

1. **Fork and Clone:**

   ```bash
   # Fork the repository on GitHub, then:
   git clone https://github.com/YOUR-USERNAME/AEnvironment
   cd AEnvironment
   ```

2. **Set Up Development Environment:**

   See [Development Setup](#development-setup) for detailed instructions.

3. **Find an Issue:**

   - Browse [good first issues](https://github.com/inclusionAI/AEnvironment/labels/good%20first%20issue)
   - Check [help wanted](https://github.com/inclusionAI/AEnvironment/labels/help%20wanted) issues
   - Or create a new issue using our [issue templates](https://github.com/inclusionAI/AEnvironment/issues/new/choose)

4. **Make Your Changes:**

   - Create a branch: `git checkout -b feature/your-feature-name`
   - Make your changes following our [coding standards](#coding-standards)
   - Test your changes following the [testing guidelines](#testing)

5. **Submit a Pull Request**

   See [Pull Request Guidelines](#pull-request-guidelines) for details.

## Project Structure

AEnvironment is a multi-language project with Python SDK and Go services:

```bash
AEnvironment/
├── aenv/                 # Python SDK and CLI
│   ├── src/
│   │   ├── aenv/        # Core SDK (MCP server, tools, environment)
│   │   └── cli/          # CLI tool (build, deploy, manage environments)
│   └── examples/         # Example environments
├── api-service/         # API gateway service (Go)
├── controller/          # Environment controller (Go)
├── envhub/              # Environment registry service (Go)
├── deploy/              # Helm charts for Kubernetes deployment
└── docs/                # Documentation
```

## Development Setup

### Prerequisites

- **Python 3.12+** (for Python SDK)
- **Go 1.21+** (for Go services)
- **Docker** (for building and testing environments)
- **Kubernetes** (optional, for full platform testing)

### Python SDK Setup

```bash
cd aenv

# Install development dependencies
pip install -e ".[dev]"

# Or using uv (recommended)
uv pip install -e ".[dev]"
```

### Go Services Setup

```bash
# Controller
cd controller
go mod download

# API Service
cd api-service
go mod download

# EnvHub
cd envhub
go mod download
```

### Code Formatting Setup

We use [pre-commit](https://pre-commit.com/) hooks for automatic code formatting and linting. The hooks cover:

- **Python** (`aenv/`): Black, isort, Ruff
- **Go** (`api-service/`, `controller/`, `envhub/`): gofmt, goimports, go-vet, golangci-lint
- **Markdown**: markdownlint
- **YAML**: yamllint
- **Dockerfile**: hadolint
- **General**: trailing whitespace, end-of-file fixer, merge conflict detection, etc.

#### Installation

```bash
# Install pre-commit (if not already installed)
pip install pre-commit
# Or using uv
uv pip install pre-commit

# Install pre-commit hooks (run from repository root)
pre-commit install

# Subsequent commits will automatically format and lint your files:
git commit -a -m 'my change'
```

#### Manual Usage

```bash
# Run all hooks on all files
pre-commit run --all-files

# Run all hooks on staged files only
pre-commit run

# Run a specific hook
pre-commit run black --all-files
pre-commit run go-fmt --all-files
pre-commit run markdownlint --all-files

# Update hooks to latest versions
pre-commit autoupdate
```

#### Skipping Hooks (Not Recommended)

If you need to skip hooks temporarily:

```bash
# Skip all hooks
git commit --no-verify -m 'my change'

# Skip specific hooks
SKIP=black,isort git commit -m 'my change'
```

You can also manually format code:

```bash
# Format Python code
cd aenv
black src/
isort src/
ruff check --fix src/

# Format Go code
cd controller  # or api-service, envhub
go fmt ./...
goimports -w .
```

Or use the formatting script:

```bash
# Format all code
./check_format.sh

# Check formatting without modifying files
./check_format.sh --check

# Format specific language
./check_format.sh --lang python
./check_format.sh --lang go
```

## Ways to Contribute

### 🐛 Bug Reports

Found a bug? Please create a [bug report](https://github.com/inclusionAI/AEnvironment/issues/new?template=bug.md) with:

- A clear description of the issue
- Steps to reproduce
- Expected vs. actual behavior
- Environment details (OS, Python/Go versions, commit ID)
- Full logs when possible

### ✨ Feature Requests

Have an idea? Submit a [feature request](https://github.com/inclusionAI/AEnvironment/issues/new?template=feature.md) with:

- Background and use case
- Proposed solution or implementation approach
- Expected benefits to the community

### 📚 Documentation

Documentation improvements are always welcome:

- Fix typos or clarify existing docs
- Add examples or tutorials
- Improve API documentation
- Update architecture diagrams

See `docs/` directory for documentation source files.

### 💻 Code Contributions

We accept various types of code contributions:

- Bug fixes
- New features
- Performance improvements
- Test coverage improvements
- Code refactoring

**IMPORTANT**: For new features and significant code changes, please submit an issue or open a draft PR to discuss with the core developers before making extensive changes.

## Coding Standards

### Python

We follow PEP 8 with some modifications:

- **Line length**: 88 characters (Black default)
- **Type hints**: Required for all function signatures
- **Docstrings**: Google-style docstrings for all public functions/classes
- **Imports**: Use `isort` with Black profile

Example:

```python
"""Module docstring describing the module."""

from typing import Optional, List

from aenv.core.exceptions import ToolError


def my_function(
    param1: str,
    param2: int = 42,
    param3: Optional[List[str]] = None
) -> dict:
    """Short description of function.

    Longer description if needed.

    Args:
        param1: Description of param1
        param2: Description of param2
        param3: Description of param3

    Returns:
        Description of return value

    Raises:
        ToolError: When something goes wrong

    Example:
        >>> result = my_function("test")
        >>> print(result)
        {"status": "ok"}
    """
    if param3 is None:
        param3 = []

    return {"status": "ok"}
```

**Formatting Tools:**

- `black`: Code formatting
- `isort`: Import sorting
- `ruff`: Linting and additional checks
- `mypy`: Type checking

### Go

We follow standard Go conventions:

- **Formatting**: Use `gofmt` or `goimports`
- **Linting**: Use `golangci-lint`
- **Comments**: Follow Go documentation conventions

Example:

```go
// Package controller provides environment lifecycle management.
package controller

import (
    "context"
    "fmt"
)

// MyFunction does something useful.
//
// It takes a context and configuration, returning a result or error.
func MyFunction(ctx context.Context, config *Config) (*Result, error) {
    if config == nil {
        return nil, fmt.Errorf("config is required")
    }

    // Implementation
    return &Result{}, nil
}
```

## Testing

### Python Tests

```bash
cd aenv

# Run all tests
pytest tests/ -v

# Run specific test file
pytest tests/test_environment.py -v

# Run with coverage
pytest tests/ --cov=aenv --cov-report=html

# Run async tests
pytest tests/ -v -m asyncio
```

**Test Structure:**

- Unit tests: `src/aenv/tests/test_*.py`
- CLI tests: `src/cli/tests/test_*.py`
- Example tests: `examples/*/test/`

### Go Tests

```bash
# Controller tests
cd controller
go test ./... -v

# API Service tests
cd api-service
go test ./... -v

# EnvHub tests
cd envhub
go test ./... -v
```

### Writing Tests

**Python:**

```python
import pytest
from aenv import Environment

@pytest.mark.asyncio
async def test_environment_creation():
    env = Environment("test-env")
    assert env.name == "test-env"

@pytest.mark.asyncio
async def test_tool_execution():
    async with Environment("test-env") as env:
        result = await env.call_tool("echo", {"message": "hello"})
        assert not result.is_error
```

**Go:**

```go
package service

import (
    "testing"
)

func TestMyFunction(t *testing.T) {
    config := &Config{...}
    result, err := MyFunction(context.Background(), config)
    if err != nil {
        t.Fatalf("unexpected error: %v", err)
    }
    if result == nil {
        t.Fatal("result is nil")
    }
}
```

## CI/CD

### Format Check

The format check runs automatically whenever a PR is opened. Your PR will pass the
format check as long as you have properly run the formatting tools:

```bash
# Python
cd aenv
black src/
isort src/
ruff check src/

# Go
cd controller  # or api-service, envhub
go fmt ./...
goimports -w .
```

Or use the formatting script:

```bash
./check_format.sh --check
```

### Tests

Tests for PRs are triggered automatically. The test suite includes:

- Python SDK unit tests
- CLI command tests
- Go service unit tests
- Example environment tests

**Writing Tests for New Features:**

If you have implemented a new feature, we highly recommend writing tests:

- Place Python test files under `aenv/src/*/tests/test_*.py`
- Place Go test files alongside source files as `*_test.go`
- Mark slow tests with `@pytest.mark.slow` (Python) or use `-short` flag (Go)

### Docker Images

Docker images are built automatically for Go services. To build locally:

```bash
# Build controller image
cd controller
docker build -t aenv/controller:latest .

# Build API service image
cd api-service
docker build -t aenv/api-service:latest .

# Build EnvHub image
cd envhub
docker build -t aenv/envhub:latest .
```

## Pull Request Guidelines

### PR Title

Use conventional commit format:

- `feat: Add new feature`
- `fix: Fix bug in X`
- `docs: Update documentation`
- `refactor: Refactor code`
- `test: Add tests`
- `chore: Update dependencies`

### PR Description

Include:

1. **What**: Brief description of changes
2. **Why**: Motivation for the change
3. **How**: Implementation approach (if significant)
4. **Testing**: How it was tested
5. **Related Issues**: Link to related issues using `Fixes #123` or `Closes #456`

### PR Checklist

Before submitting, ensure:

- [ ] Tests pass locally
- [ ] Code is formatted (run `./check_format.sh --check`)
- [ ] Type checks pass (Python: `mypy`, Go: `go vet`)
- [ ] Documentation updated (if applicable)
- [ ] No breaking changes (or documented in PR description)
- [ ] Commit messages follow conventional commit format

### Review Process

1. **Automated checks**: CI runs tests, linters, and format checks
2. **Code review**: At least one maintainer review required
3. **Approval**: Maintainer approves
4. **Merge**: Squash and merge (preferred) or merge commit

## Additional Resources

- **Detailed Contributing Guide**: See `docs/development/contributing.md`
- **Building Guide**: See `docs/development/building.md`
- **Architecture Documentation**: See `docs/architecture/`
- **API Documentation**: See `docs/guide/`

## Getting Help

- **GitHub Issues**: Bug reports and feature requests
- **GitHub Discussions**: Questions and ideas
- **Documentation**: Check `docs/` directory

## Recognition

Contributors are recognized in:

- CONTRIBUTORS.md (if exists)
- Release notes
- Project documentation

Thank you for contributing to AEnvironment! 🙏
