# 🚀 AEnvironment Docker Demo - 快速启动

## ⚡ 一键启动（推荐）

```bash
# 从项目根目录执行
docker build -f controller/Dockerfile -t aenv-controller:latest . && \
docker build -f api-service/Dockerfile -t aenv-api-service:latest . && \
cd examples/docker_all_in_one && \
./scripts/demo.sh
```

## 📋 手动步骤

### 1. 构建镜像

```bash
# Controller
docker build -f controller/Dockerfile -t aenv-controller:latest .

# API Service
docker build -f api-service/Dockerfile -t aenv-api-service:latest .
```

### 2. 启动服务

```bash
cd examples/docker_all_in_one
docker compose up -d
```

### 3. 验证服务

```bash
# 查看容器状态
docker ps --filter "name=aenv"

# 测试 Controller（容器内部健康检查）
docker exec aenv-controller wget -qO- http://localhost:8081/readyz

# 测试 Controller API（从宿主机）
curl http://localhost:9090/containers

# 测试 API Service
curl http://localhost:8080/health
```

### 4. 运行 Demo

```bash
# 方法 A: 使用脚本（推荐）
./scripts/demo.sh

# 方法 B: 手动运行
cd weather-demo
aenv build
python run_demo.py
```

## 🔍 常用命令

### 查看日志

```bash
# Controller
docker logs -f aenv-controller

# API Service
docker logs -f aenv-api-service

# 所有服务
docker compose logs -f
```

### 管理容器

```bash
# 列出所有 AEnv 实例
curl http://localhost:9090/containers

# 创建测试实例
curl -X POST http://localhost:8080/env-instance \
  -H "Content-Type: application/json" \
  -d '{
    "envName": "weather-demo@1.0.0-docker",
    "ttl": "30m"
  }'

# 删除实例
curl -X DELETE http://localhost:9090/containers/<CONTAINER_ID>
```

### 停止服务

```bash
# 停止所有服务
docker compose down

# 停止并删除卷
docker compose down -v

# 使用脚本
./scripts/stop.sh
```

## ⚠️ 重要说明

### 端口映射

| 服务 | 容器内部端口 | 宿主机端口 | 用途 |
|------|------------|-----------|------|
| Controller | 8080 | 9090 | 主服务 API |
| Controller | 8081 | *未映射* | 健康检查（仅容器内部） |
| API Service | 8080 | 8080 | API 服务 |

### 为什么 `curl http://localhost:8081/readyz` 会失败？

- ❌ **错误原因**：端口 `8081` 是 Controller 的健康检查端口，仅在**容器内部**使用，未映射到宿主机
- ✅ **正确方法**：
  - **容器内测试**：`docker exec aenv-controller wget -qO- http://localhost:8081/readyz`
  - **宿主机测试**：`curl http://localhost:9090/containers`（使用主服务端口）

### 服务间通信

- **容器间通信**：使用容器内部端口
  - API Service → Controller: `http://controller:8080`
  - Controller → Docker Daemon: `unix:///var/run/docker.sock`
  
- **宿主机访问**：使用映射端口
  - Controller: `http://localhost:9090`
  - API Service: `http://localhost:8080`

## 🐛 快速故障排除

### 容器不健康

```bash
# 检查日志
docker logs aenv-controller
docker logs aenv-api-service

# 重启服务
docker compose restart controller
docker compose restart api-service
```

### 端口被占用

```bash
# 检查端口占用
lsof -i :8080
lsof -i :9090

# 停止占用端口的进程或修改 docker-compose.yml
```

### weather-demo 构建失败

```bash
# 重新安装 aenv CLI
cd ../../aenv
pip install -e .
aenv version

# 手动构建镜像
cd ../examples/docker_all_in_one/weather-demo
docker build -t aenv/weather-demo:1.0.0-docker .
```

## 📚 完整文档

详细说明请查看 [README_COMPLETE_GUIDE.md](./README_COMPLETE_GUIDE.md)

## ✅ 验证清单

- [ ] Docker Desktop 正在运行
- [ ] 端口 8080, 9090 未被占用
- [ ] Controller 容器状态为 `healthy`
- [ ] API Service 容器状态为 `healthy`
- [ ] 可以访问 `http://localhost:9090/containers`
- [ ] 可以访问 `http://localhost:8080/health`
- [ ] weather-demo 镜像已构建
- [ ] Demo 脚本运行成功

---

**最后更新**: 2026-02-08  
**状态**: ✅ 完全可用
