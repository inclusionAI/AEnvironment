# 🚀 AEnvironment Docker All-in-One Demo - 完整运行指南

本指南提供了从零开始运行 AEnvironment Docker Engine Demo 的完整步骤。

## 📋 前置要求

- **Docker Desktop**: 已安装并运行（版本 >= 24.0）
- **Go**: 1.21+ （仅当需要本地构建时）
- **Python**: 3.8+ （用于 aenv CLI）
- **系统**: macOS / Linux / Windows with WSL2

## 🔧 步骤 1: 验证环境

```bash
# 检查 Docker 是否运行
docker info

# 检查 Docker API 版本（应该 >= 1.44）
docker version --format '{{.Server.APIVersion}}'

# 检查 Docker Compose
docker compose version
```

## 🏗️ 步骤 2: 构建服务镜像

从项目根目录执行：

```bash
# 构建 Controller 镜像（约 2-3 分钟）
docker build -f controller/Dockerfile -t aenv-controller:latest .

# 构建 API Service 镜像（约 2-3 分钟）
docker build -f api-service/Dockerfile -t aenv-api-service:latest .

# 验证镜像已创建
docker images | grep aenv
```

**预期输出**：

```text
aenv-controller    latest    <image-id>   ...   ...MB
aenv-api-service   latest    <image-id>   ...   ...MB
```

## 🐍 步骤 3: 安装 aenv CLI（可选但推荐）

```bash
# 进入 aenv 目录
cd aenv

# 安装 aenv CLI
pip install -e .

# 验证安装
aenv version

# 返回项目根目录
cd ..
```

## 🎯 步骤 4: 运行 Demo

### 方法 A: 使用自动化脚本（推荐）

```bash
cd examples/docker_all_in_one

# 运行完整 demo
./scripts/demo.sh
```

### 方法 B: 手动步骤

#### 4.1 启动 AEnvironment 服务

```bash
cd examples/docker_all_in_one

# 启动服务
./scripts/start.sh

# 或者直接使用 docker compose
docker compose up -d
```

#### 4.2 验证服务健康

```bash
# 检查容器状态（应该都是 healthy）
docker ps --filter "name=aenv"

# 方法 1: 通过容器内部测试 Controller 健康检查（推荐）
docker exec aenv-controller wget -O- http://localhost:8081/readyz

# 方法 2: 通过主服务端口测试 Controller（从宿主机）
curl http://localhost:9090/containers

# 测试 API Service 健康检查
curl http://localhost:8080/health
```

**预期输出**：

```text
CONTAINER ID   IMAGE                     STATUS                    PORTS
...            aenv-controller:latest    Up ... (healthy)          0.0.0.0:9090->8080/tcp
...            aenv-api-service:latest   Up ... (healthy)          0.0.0.0:8080->8080/tcp
```

**注意**：

- Controller 的健康检查端口 `8081` 仅在容器内部使用，未映射到宿主机
- 从宿主机访问 Controller 应使用主服务端口 `9090`（映射到容器内部的 `8080`）
- 健康检查端点 `/readyz` 在容器内部的 `8081` 端口
- 主服务 API 在容器内部的 `8080` 端口（映射到宿主机 `9090`）

#### 4.3 构建 weather-demo 镜像

```bash
# 使用 aenv CLI 构建（推荐）
cd weather-demo
aenv build

# 或者手动构建
docker build -t aenv/weather-demo:1.0.0-docker .
cd ..
```

#### 4.4 运行 Demo 客户端

```bash
# 运行 weather demo
cd weather-demo
python run_demo.py
cd ..
```

## 📊 步骤 5: 验证运行结果

### 查看运行中的容器

```bash
docker ps -a | grep -E "aenv|weather"
```

你应该看到：

- `aenv-controller` (healthy)
- `aenv-api-service` (healthy)  
- `docker-weather-demo-*` (运行中的 weather-demo 实例)

### 查看日志

```bash
# Controller 日志
docker logs aenv-controller

# API Service 日志
docker logs aenv-api-service

# Weather Demo 日志
docker logs $(docker ps -a | grep weather-demo | awk '{print $1}')
```

### 手动测试 API

```bash
# 创建测试环境实例
curl -X POST http://localhost:8080/env-instance \
  -H "Content-Type: application/json" \
  -d '{
    "envName": "weather-demo@1.0.0-docker",
    "ttl": "30m",
    "environment_variables": {
      "TEST": "true"
    }
  }'

# 列出所有容器实例
curl http://localhost:9090/containers

# 查看特定容器详情
curl http://localhost:9090/containers/<CONTAINER_ID>

# 删除容器
curl -X DELETE http://localhost:9090/containers/<CONTAINER_ID>
```

## 🛑 步骤 6: 停止和清理

```bash
cd examples/docker_all_in_one

# 使用脚本停止
./scripts/stop.sh

# 或者手动停止
docker compose down -v

# 清理所有 aenv 相关容器
docker ps -a | grep aenv | awk '{print $1}' | xargs -r docker rm -f

# 清理网络
docker network rm aenv-network 2>/dev/null || true
```

## 🔍 故障排除

### 问题 1: Controller 容器不健康

**症状**: `docker ps` 显示 `aenv-controller` 状态为 `unhealthy`

**解决方案**:

```bash
# 查看日志
docker logs aenv-controller

# 检查健康检查端点
docker exec aenv-controller wget -O- http://localhost:8081/readyz

# 常见原因：
# 1. Docker API 版本不匹配 → 已通过设置 DOCKER_API_VERSION=1.44 解决
# 2. 健康检查端口错误 → 使用 /readyz 在 8081 端口
# 3. Docker socket 权限 → 确保 /var/run/docker.sock 可访问
```

### 问题 2: API Service 连接 Controller 失败

**症状**: 错误信息 `unsupported protocol scheme ""`

**解决方案**:

```bash
# 检查 docker-compose.yml 中的 command 配置
grep -A 5 "api-service:" examples/docker_all_in_one/docker-compose.yml

# 应该包含：
# command:
#   - "--schedule-addr=http://controller:8080"
#   - "--schedule-type=docker"
```

### 问题 3: weather-demo 容器启动失败

**症状**: 容器立即退出，错误 `No module named aenv.server.__main__`

**解决方案**:

```bash
# 检查 Dockerfile CMD
cat examples/docker_all_in_one/weather-demo/Dockerfile | grep CMD

# 正确的 CMD 应该是：
# CMD ["python", "-m", "aenv.main", "src"]

# 重新构建镜像
cd examples/docker_all_in_one/weather-demo
docker build -t aenv/weather-demo:1.0.0-docker .
```

### 问题 4: Docker 构建失败 - Go 版本不兼容

**症状**: 构建时报错 `requires go >= 1.23` 或依赖版本冲突

**解决方案**:
所有依赖已降级到 Go 1.21 兼容版本：

- `golang.org/x/crypto`: v0.17.0
- `k8s.io/apimachinery`: v0.28.4  
- `sigs.k8s.io/controller-runtime`: v0.16.3
- `github.com/docker/docker`: v24.0.7+incompatible

如果遇到问题，执行：

```bash
# 清理缓存重新构建
docker build --no-cache -f controller/Dockerfile -t aenv-controller:latest .
docker build --no-cache -f api-service/Dockerfile -t aenv-api-service:latest .
```

## 📝 关键配置说明

### docker-compose.yml 关键配置

#### Controller 配置

```yaml
controller:
  environment:
    - ENGINE_TYPE=docker
    - DOCKER_HOST=unix:///var/run/docker.sock
    - DOCKER_API_VERSION=1.44  # 设置在 Dockerfile 中
  volumes:
    - /var/run/docker.sock:/var/run/docker.sock  # Docker socket 挂载
  ports:
    - "9090:8080"  # 宿主机:容器 端口映射
  healthcheck:
    test: ["CMD", "wget", "--spider", "-q", "http://localhost:8081/readyz"]
```

#### API Service 配置

```yaml
api-service:
  command:
    - "--schedule-addr=http://controller:8080"  # 容器间通信使用内部端口
    - "--schedule-type=docker"                   # 指定 Docker 引擎模式
  ports:
    - "8080:8080"
  depends_on:
    controller:
      condition: service_healthy  # 等待 Controller 健康后启动
```

### weather-demo Dockerfile 配置

```dockerfile
FROM python:3.12-slim
WORKDIR /app
ENV PYTHONPATH=/app/src

COPY requirements.txt .
RUN python -m pip install --no-cache-dir -r requirements.txt

COPY . .

# 正确的启动命令
CMD ["python", "-m", "aenv.main", "src"]
```

## 🎯 成功标志

当 demo 成功运行时，你应该看到：

```text
============================================================
  AEnvironment Docker Engine Demo
============================================================
API Service: http://localhost:8080/

============================================================
  Creating Environment Instance
============================================================
Environment: weather-demo@1.0.0-docker
✓ Environment instance created

============================================================
  Listing Available Tools
============================================================
[list_tools()]
Response: {'tools': [...]}

============================================================
  Calling Tools
============================================================
[get_weather('Beijing')]
Response: {'city': 'Beijing', 'temperature': '20', ...}

============================================================
  Demo Completed
============================================================
✓ All operations completed successfully
```

## 📚 相关文档

- [Docker Engine Support](../../docs/DOCKER_ENGINE_SUPPORT.md)
- [Docker Engine Implementation Review](../../docs/DOCKER_ENGINE_IMPLEMENTATION_REVIEW.md)
- [Go 1.21 Rollback Details](../../docs/ROLLBACK_TO_GO_1_21.md)
- [Architecture Overview](../../docs/architecture/architecture.md)

## 🆘 获取帮助

如果遇到问题：

1. **查看日志**: `docker logs <container-name>`
2. **检查容器状态**: `docker ps -a`
3. **验证镜像**: `docker images | grep aenv`
4. **检查网络**: `docker network inspect aenv-network`
5. **清理重试**: `./scripts/stop.sh && ./scripts/start.sh`

## ✅ 快速启动命令（一键运行）

```bash
# 从项目根目录执行
docker build -f controller/Dockerfile -t aenv-controller:latest . && \
docker build -f api-service/Dockerfile -t aenv-api-service:latest . && \
cd examples/docker_all_in_one && \
./scripts/demo.sh
```

---

**提示**:

- 首次运行会下载依赖，需要 5-10 分钟
- 后续运行使用缓存，约 1-2 分钟
- 确保端口 8080, 9090, 8081 未被占用
- 建议使用 Go 1.21 以获得最佳兼容性

**状态**: ✅ **完全可用** - 所有功能已验证通过（2026-02-07）
