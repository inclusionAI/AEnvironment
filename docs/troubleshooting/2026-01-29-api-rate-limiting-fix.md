# AEnvironment Controller API Rate Limiting Fix

## 问题描述

**时间**: 2026-01-29
**集群**: eu126-sqa
**问题**: `aenv service list` 命令失败，返回 500 错误

### 错误信息

```
Failed to list services: list services: request failed with status 500:
failed to list deployments failed: err is the server has received too many
requests and has asked us to try again later (get deployments.apps)
```

### 根本原因

Controller 组件遇到 Kubernetes API server 的速率限制（rate limiting），导致：

1. Pod reflector 无法成功 list/watch pods
2. Service handler 无法 list deployments
3. 两者共享同一个速率限制器，相互竞争

## 已实施的修复

### 第一轮修复 (Commit: ed2cf86)

**部署的镜像**: `reg.antgroup-inc.cn/aenv/controller:ed2cf86-202601291452-1`

#### 主要改动

1. **降低 QPS 和 Burst** (从 1000/1000 → 5/10)
   - [main.go:127-128](../controller/cmd/main.go#L127-L128)
   - [aenv_service_handler.go:63-64](../controller/pkg/aenvhub_http_server/aenv_service_handler.go#L63-L64)
   - [aenv_pod_handler.go:67-68](../controller/pkg/aenvhub_http_server/aenv_pod_handler.go#L67-L68)

2. **实现 Lazy REST Mapper**
   - [main.go:172-176](../controller/cmd/main.go#L172-L176)
   - 避免启动时发现所有 300+ CRD

3. **使用共享 Clientset**
   - [main.go:71-80](../controller/cmd/main.go#L71-L80)
   - 所有 handler 共享同一个 clientset 和速率限制器

4. **优化 Pod Cache**
   - [aenv_pod_cache.go:43-93](../controller/pkg/aenvhub_http_server/aenv_pod_cache.go#L43-L93)
   - 从 SharedInformerFactory 改为直接使用 ListWatchFromClient
   - 缓存同步改为异步执行

5. **增强日志**
   - 添加 emoji 标记便于识别新版本
   - 🔧 API Rate Limiting configured
   - 🚀 Creating lazy REST mapper
   - ✅ Successful initialization
   - 🔗 Creating shared clientset
   - 🎯 Using optimized ListWatcher

#### 验证结果

✅ 新版本日志确认已部署
❌ `aenv service list` 仍然失败 (QPS=5 过低)

### 第二轮修复 (Commit: fa9cba6)

**部署的镜像**: `reg.antgroup-inc.cn/aenv/controller:fa9cba6-202601291500-1`

#### 主要改动

**提高 QPS 到 20, Burst 到 40** (从 5/10 → 20/40)

- [main.go:127-128](../controller/cmd/main.go#L127-L128)
- [aenv_service_handler.go:63-64](../controller/pkg/aenvhub_http_server/aenv_service_handler.go#L63-L64)
- [aenv_pod_handler.go:67-68](../controller/pkg/aenvhub_http_server/aenv_pod_handler.go#L67-L68)

**原因**: QPS=5 过于保守，导致 Pod reflector 和 Service handler 争抢速率配额

#### 验证结果

❌ `aenv service list` **仍然失败** (集群 API server 负载过高)

## 当前状态

### 部署信息

- **分支**: `fix/controller`
- **最新 Commit**: `fa9cba6`
- **镜像**: `reg.antgroup-inc.cn/aenv/controller:fa9cba6-202601291500-1`
- **命名空间**: `aenv`
- **集群**: `eu126-sqa`

### 问题分析

1. ✅ 代码修改已生效（日志确认）
2. ✅ 优化措施已实施（lazy mapper, shared clientset, async cache）
3. ❌ **eu126-sqa 集群的 API server 负载极其严重**
4. ❌ 即使使用 QPS=20，Pod reflector 仍然无法成功同步
5. ❌ Deployments list 操作继续被限流

### 日志证据

```
W0129 06:55:01.534283 reflector.go:424 failed to list *v1.Pod:
  the server has received too many requests and has asked us to try again later
```

**直接使用 kubectl 却可以成功**:

```bash
$ kubectl -n aenv-sandbox get deployments
No resources found in aenv-sandbox namespace.
```

这说明问题在于 controller 的多个并发请求（Pod reflector + API handler）。

## 下一步方案

### 方案 A: 进一步提高 QPS (推荐)

将 QPS 提升到 50-100，Burst 提升到 100-200

**优点**:

- 简单直接
- 允许 Pod reflector 和 Service handler 并行工作

**缺点**:

- 可能对集群 API server 造成更大压力
- 如果集群整体负载过高，可能仍然失败

### 方案 B: 完全禁用 Pod Cache 自动同步

修改 `aenv_pod_cache.go`，不启动后台 reflector

**优点**:

- 彻底消除后台 API 请求
- 释放所有 QPS 配额给用户请求

**缺点**:

- Pod list/get 操作将直接请求 API server（无缓存）
- 可能影响 pod 相关功能的性能

### 方案 C: 使用 API Priority and Fairness

配置 Kubernetes API server 的 PriorityLevelConfiguration

**优点**:

- 从源头解决问题
- 可以为 controller 保留专用的 QPS 配额

**缺点**:

- 需要集群管理员权限
- 需要修改集群配置

### 方案 D: 延迟 Pod Cache 启动

延迟 30-60 秒后再启动 Pod reflector，让用户请求先完成

**优点**:

- 避免启动时的 QPS 争抢
- 代码改动较小

**缺点**:

- 启动后 30-60 秒内 pod 功能不可用
- 治标不治本

## Git 历史

```bash
fa9cba6 (HEAD -> fix/controller) fix(controller): increase QPS to 20 for highly loaded clusters
ed2cf86 fix(controller): resolve API rate limiting with enhanced logging
c714edf (origin/main, main) fix kubeconfig issue
```

## 相关文件

### 核心文件

- [controller/cmd/main.go](../controller/cmd/main.go) - 主入口，速率限制配置
- [controller/pkg/aenvhub_http_server/aenv_service_handler.go](../controller/pkg/aenvhub_http_server/aenv_service_handler.go) - Service API handler
- [controller/pkg/aenvhub_http_server/aenv_pod_handler.go](../controller/pkg/aenvhub_http_server/aenv_pod_handler.go) - Pod API handler
- [controller/pkg/aenvhub_http_server/aenv_pod_cache.go](../controller/pkg/aenvhub_http_server/aenv_pod_cache.go) - Pod cache 实现

### 构建和部署

- [controller/Dockerfile](../controller/Dockerfile)
- [controller/Makefile](../controller/Makefile)

## 测试命令

### 验证部署

```bash
export KUBECONFIG=/Users/jun/.kube/eu126-sqa-config

# 检查镜像版本
kubectl -n aenv get deployment controller -o jsonpath='{.spec.template.spec.containers[0].image}'

# 查看日志（寻找 emoji 标记）
kubectl -n aenv logs -l app.kubernetes.io/name=controller --tail=50 | grep -E "(🔧|🚀|✅|🔗|🎯)"

# 检查速率限制配置
kubectl -n aenv logs -l app.kubernetes.io/name=controller --tail=200 | grep "QPS"
```

### 测试功能

```bash
# 测试 service list
aenv service list

# 查看实时错误
kubectl -n aenv logs -l app.kubernetes.io/name=controller -f
```

### 构建新镜像

```bash
cd AEnvironment

# 提交修改
git add controller/
git commit -m "fix: your message"
git push origin fix/controller

# 构建镜像
COMMIT=$(git rev-parse --short HEAD)
TIMESTAMP=$(date +%Y%m%d%H%M)
NEW_IMAGE="reg.antgroup-inc.cn/aenv/controller:${COMMIT}-${TIMESTAMP}-1"

docker build -t "${NEW_IMAGE}" -f controller/Dockerfile .
docker push "${NEW_IMAGE}"

# 更新部署
kubectl -n aenv set image deployment/controller "controller=${NEW_IMAGE}"
kubectl -n aenv rollout status deployment/controller
```

## 建议

**立即行动**: 实施方案 A + B 组合

1. 将 QPS 提升到 50, Burst 100
2. 暂时禁用 Pod Cache 的后台同步（只在需要时按需加载）
3. 观察效果

**长期解决**:

1. 与集群管理员沟通，调查 API server 高负载的根本原因
2. 考虑启用 API Priority and Fairness
3. 如果是 CRD 过多导致，考虑清理不必要的 CRD

## 联系方式

如有问题，请查看：

- GitHub Issues: <https://github.com/inclusionAI/AEnvironment/issues>
- 提交日期: 2026-01-29
- 调试人员: Claude (claude-sonnet-4-5)
