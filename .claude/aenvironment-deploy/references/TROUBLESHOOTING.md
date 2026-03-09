# Troubleshooting Guide

Common issues and solutions when using AEnvironment.

## Installation Issues

### aenv command not found

**Problem**: `aenv: command not found` after installation

**Solution**:

```bash
# Reinstall with --upgrade
pip install aenvironment --upgrade

# Verify installation
aenv version

# If still not found, check PATH
which aenv
python -m aenv version
```

### Permission denied

**Problem**: Permission errors when running aenv

**Solution**:

```bash
# Use --user flag
pip install --user aenvironment --upgrade

# Or use a virtual environment
python -m venv venv
source venv/bin/activate
pip install aenvironment
```

## Configuration Issues

### Invalid configuration

**Problem**: Configuration validation errors

**Solution**:

```bash
# Show current config
aenv config show

# Reset specific setting
aenv config set owner <correct-owner-name>

# Verify each setting one by one
```

### Registry authentication failed

**Problem**: Cannot push to Docker registry

**Solution**:

1. Verify registry credentials are correct
2. Check Docker is installed: `docker --version`
3. Test Docker login manually:

   ```bash
   docker login <registry-host> -u <username> -p <password>
   ```

4. Re-configure aenv registry settings

## Build Issues

### Docker build failed

**Problem**: `aenv build` fails

**Solutions**:

**Check Dockerfile syntax**:

```bash
cd <env-directory>
docker build -t test .
```

**Check platform compatibility**:

```bash
# Use correct platform
aenv build --platform linux/amd64
```

**Check Docker daemon**:

```bash
docker ps  # Should not error
```

### Build timeout

**Problem**: Build takes too long and times out

**Solution**:

- Optimize Dockerfile (use smaller base images, multi-stage builds)
- Increase timeout in aenv_operations.py
- Build without push first: `docker build .`

## Environment Registration Issues

### Push failed: config.json not found

**Problem**: `aenv push` fails with config.json error

**Solution**:

```bash
# Ensure you're in the right directory
ls config.json

# Reinitialize if needed
aenv init <env-name> --config-only
```

### Environment version conflict

**Problem**: Version already exists in EnvHub

**Solution**:

1. Update version in config.json
2. Re-run `aenv push`

```json
{
  "version": "1.0.1"  // Increment version
}
```

## Deployment Issues

### Instance creation failed

**Problem**: `aenv instance create` fails

**Common Causes**:

1. **Environment not registered**:

   ```bash
   # Check if environment exists
   aenv list
   aenv get <env-name>
   ```

2. **Invalid environment spec**:

   ```bash
   # Use correct format: name@version
   aenv instance create myenv@1.0.0
   ```

3. **Resource quota exceeded**:
   - Check cluster resource limits
   - Reduce CPU/memory in config.json

### Instance not accessible

**Problem**: Cannot access instance via IP

**Solutions**:

1. **Check instance status**:

   ```bash
   aenv instance list
   aenv instance get <instance-id>
   ```

2. **Verify network access**:

   ```bash
   ping <instance-ip>
   curl http://<instance-ip>:<port>
   ```

3. **Check application logs** (if accessible):
   - Application may not be listening on expected port
   - Application may have crashed

### Service creation failed with storage

**Problem**: Service with storage fails to create

**Solution**:

```bash
# Ensure replicas=1 when storage enabled
aenv service create myapp@1.0.0 \
  --replicas 1 \
  --enable-storage \
  --port 8080
```

**Check config.json**:

```json
{
  "deployConfig": {
    "service": {
      "replicas": 1,  // Must be 1
      "enableStorage": true
    }
  }
}
```

### Service URL not accessible

**Problem**: Cannot access service via domain

**Solutions**:

1. **Check service status**:

   ```bash
   aenv service list
   aenv service get <service-id>
   ```

2. **Verify URL format**:

   ```text
   http://<service-name>.aenv-sandbox.svc.tydd-staging.alipay.net:<port>
   ```

3. **Check network access**:
   - Ensure you're on the correct network (e.g., office network)
   - Try from different network if possible

## Common Error Messages

### "Environment not found"

**Cause**: Environment not registered in EnvHub

**Solution**:

```bash
# List all environments
aenv list

# Register environment
cd <env-directory>
aenv push
```

### "ReadWriteOnce volume conflict"

**Cause**: Multiple replicas with storage enabled

**Solution**:
Set `replicas: 1` in config.json or command line

### "Image pull failed"

**Cause**: Registry authentication or image not found

**Solutions**:

1. Verify image name in config.json artifacts
2. Check registry credentials
3. Ensure image was pushed successfully

### "Resource quota exceeded"

**Cause**: Cluster resource limits reached

**Solutions**:

1. Reduce CPU/memory in config.json
2. Delete unused instances/services
3. Contact cluster administrator

## Getting Help

If issues persist:

1. **Check configuration**:

   ```bash
   aenv config show
   ```

2. **Enable verbose output**:

   ```bash
   # Use --verbose flag in workflow scripts
   python scripts/deploy_with_local_build.py --verbose ...
   ```

3. **Review recent deployments**:

   ```bash
   aenv instance list
   aenv service list
   ```

4. **Check environment details**:

   ```bash
   aenv get <env-name>
   aenv instance get <instance-id>
   aenv service get <service-id>
   ```
