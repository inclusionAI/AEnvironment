# Go 1.21 Rollback - Complete Documentation

## Decision

**Rolled back from Go 1.23 to Go 1.21** to maintain compatibility with more environments and avoid forcing users to upgrade their Go installation.

## Why Rollback?

### 1. Broader Compatibility

- Go 1.21 is more widely deployed
- Many CI/CD systems still use Go 1.21
- Avoid forcing users to upgrade

### 2. Dependency Management

- Instead of upgrading Go, we downgraded dependency packages
- This is more maintainable long-term
- Gives us control over dependency versions

### 3. Stable Ecosystem

- Go 1.21 is a mature, stable release
- Well-tested with Kubernetes ecosystem
- Fewer breaking changes

## Changes Made

### 1. Rolled Back Go Versions

| File | Old Version | New Version |
| ------ |-------------|-------------|
| `controller/Dockerfile` | golang:1.23-alpine | golang:1.21-alpine |
| `api-service/Dockerfile` | golang:1.23-alpine | golang:1.21-alpine |
| `go.work` | go 1.23 | go 1.21 |
| `controller/go.mod` | go 1.23 | go 1.21 |
| `api-service/go.mod` | go 1.23 | go 1.21 |

### 2. Downgraded Dependencies

**golang.org/x packages:**

```
golang.org/x/crypto  v0.44.0 → v0.17.0  (Go 1.24 → Go 1.21)
golang.org/x/net     v0.47.0 → v0.19.0  (Go 1.24 → Go 1.21)
golang.org/x/sys     v0.38.0 → v0.15.0  (Go 1.24 → Go 1.21)
golang.org/x/text    v0.31.0 → v0.14.0  (Go 1.24 → Go 1.21)
```

**Kubernetes packages:**

```
k8s.io/apimachinery  v0.35.0 → v0.28.4  (Go 1.25 → Go 1.21)
k8s.io/client-go     v0.35.0 → v0.28.4  (Go 1.25 → Go 1.21)
k8s.io/api           v0.35.0 → v0.28.4  (Go 1.25 → Go 1.21)
```

**Other packages:**

```
google.golang.org/protobuf  v1.36.8 → v1.31.0  (Go 1.23 → Go 1.21)
```

### 3. Fixed Version Conflicts

Added replace directive in `controller/go.mod`:

```go
// Fix genproto version conflict for Go 1.21 compatibility
replace google.golang.org/genproto => google.golang.org/genproto v0.0.0-20231106174013-bbf56f31fb17
```

This resolves the ambiguous import error with `google.golang.org/genproto/googleapis/rpc/status`.

## Scripts Updated

### 1. `scripts/fix_go_version.sh`

- Changed target version from 1.23 → 1.21
- Updates all go.mod files and go.work to Go 1.21

### 2. `scripts/downgrade_deps_go121.sh` (NEW)

- Automates dependency downgrades
- Pins specific versions compatible with Go 1.21
- Handles both controller and api-service

## Verification Steps

### 1. Check Go Versions

```bash
# All should show go 1.21
grep '^go ' go.work controller/go.mod api-service/go.mod
```

Expected output:

```
go.work:go 1.21
controller/go.mod:go 1.21
api-service/go.mod:go 1.21
```

### 2. Verify Dependency Versions

```bash
cd controller
go list -m all | grep -E "(golang.org/x/(crypto|net|sys|text)|k8s.io/apimachinery|google.golang.org/protobuf)"
```

Expected versions:

```
golang.org/x/crypto v0.17.0
golang.org/x/net v0.19.0
golang.org/x/sys v0.15.0
golang.org/x/text v0.14.0
k8s.io/apimachinery v0.28.4
google.golang.org/protobuf v1.31.0
```

### 3. Test Docker Build

```bash
# Build Controller
docker build -f controller/Dockerfile -t aenv-controller:latest .

# Build API Service  
docker build -f api-service/Dockerfile -t aenv-api-service:latest .
```

Expected: Builds complete without "requires go >= 1.2X" errors.

### 4. Run Demo

```bash
cd examples/docker_all_in_one
./scripts/demo.sh
```

Expected: Services start successfully.

## Benefits of This Approach

### 1. No User Impact

- Users don't need to upgrade Go
- Works with existing CI/CD setups
- Compatible with older systems

### 2. Controlled Dependencies

- We explicitly choose package versions
- Prevents automatic upgrades breaking compatibility
- Easier to debug version-related issues

### 3. Stable Build

- Go 1.21 is well-tested
- Dependencies are proven stable
- Less risk of new bugs

## Dependency Version Strategy

### Philosophy

**Pin to LTS-like versions** rather than chasing latest:

1. **golang.org/x packages**: Use versions from ~1 year ago
   - Stable, well-tested
   - Security fixes backported
   - Compatible with Go 1.21

2. **k8s.io packages**: Use v0.28.x (Kubernetes 1.28)
   - LTS-supported version
   - Widely deployed
   - Good balance of features vs stability

3. **google.golang.org/protobuf**: Use v1.31.x
   - Mature, stable
   - Compatible with Go 1.21
   - Sufficient for our needs

### When to Upgrade

Upgrade dependencies when:

- Security vulnerability requires it
- New feature is critically needed
- Package maintainers drop support for old version

**Don't upgrade** just because:

- A new version exists
- `go mod tidy` suggests it
- Other projects use it

## Timeline

| Date | Go Version | Reason |
| ------ |------------|--------|
| Initial | 1.21 | Original Dockerfile |
| 2026-02-07 (Fix #1) | 1.22 | Match controller/go.mod |
| 2026-02-07 (Fix #2) | 1.23 | Satisfy dependency requirements |
| 2026-02-07 (Rollback) | **1.21** | **Downgrade deps instead** |

## Related Files

**Configuration:**

- `go.work` - Go workspace file (Go 1.21)
- `controller/go.mod` - Controller dependencies (Go 1.21)
- `api-service/go.mod` - API Service dependencies (Go 1.21)

**Dockerfiles:**

- `controller/Dockerfile` - Uses golang:1.21-alpine
- `api-service/Dockerfile` - Uses golang:1.21-alpine

**Documentation:**

- `docs/ROLLBACK_TO_GO_1_21.md` - This document
- `docs/DOCKER_ENGINE_IMPLEMENTATION_REVIEW.md` - Final implementation review

## Quick Command Reference

### Rollback to Go 1.21 (if needed)

```bash
# Update Dockerfiles
perl -pi -e 's/golang:1\.\d+-alpine/golang:1.21-alpine/' controller/Dockerfile api-service/Dockerfile

# Update go.mod files
perl -pi -e 's/^go 1\.\d+.*$/go 1.21/' go.work controller/go.mod api-service/go.mod

# Downgrade dependencies
./scripts/downgrade_deps_go121.sh

# Or manually
cd controller
go get golang.org/x/crypto@v0.17.0
go get golang.org/x/net@v0.19.0
go get golang.org/x/sys@v0.15.0
go get golang.org/x/text@v0.14.0
go get k8s.io/apimachinery@v0.28.4
go get k8s.io/client-go@v0.28.4
go get google.golang.org/protobuf@v1.31.0
go mod tidy
cd ..

cd api-service
go get golang.org/x/crypto@v0.17.0
go get golang.org/x/net@v0.19.0
go get golang.org/x/sys@v0.15.0
go get golang.org/x/text@v0.14.0
go get google.golang.org/protobuf@v1.31.0
go mod tidy
cd ..
```

### Verify Everything Works

```bash
# Check versions
grep '^go ' go.work */go.mod

# Test build
docker build -f controller/Dockerfile -t test-controller .
docker build -f api-service/Dockerfile -t test-api-service .

# Run demo
cd examples/docker_all_in_one && ./scripts/demo.sh
```

## Status

✅ **COMPLETED** - 2026-02-07

All components rolled back to Go 1.21 with compatible dependency versions.

## Recommendation

**Keep Go 1.21 for the foreseeable future** unless:

1. Critical security vulnerability in Go 1.21
2. New Go feature is absolutely required
3. All target environments support Go 1.23+

When in doubt, **downgrade dependencies** rather than upgrading Go version.
