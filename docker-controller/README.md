# Docker Controller

独立的 Docker Controller 模块，用于本地 Docker 部署模式。

## 概述

Docker Controller 是一个独立的 Go 模块，提供与 Kubernetes Controller 相同的 HTTP API 接口，但通过 Docker Socket 直接管理 Docker 容器，无需 Kubernetes 集群。

## 特性

- ✅ **独立模块**: 完全独立的 Go 模块，不依赖其他 controller 代码
- ✅ **Docker 原生**: 直接通过 Docker Socket 管理容器
- ✅ **API 兼容**: 与 Kubernetes Controller 相同的 HTTP API
- ✅ **容器缓存**: 内存缓存提升查询性能
- ✅ **TTL 支持**: 自动过期容器管理
- ✅ **资源限制**: 支持 CPU 和内存限制
- ✅ **环境变量**: 支持环境变量注入

## 目录结构

```
docker-controller/
├── cmd/
│   └── main.go                    # 主程序入口
├── pkg/
│   ├── constants/
│   │   └── common.go              # 常量定义
│   ├── docker_http_server/
│   │   ├── docker_container_handler.go  # HTTP 处理器
│   │   ├── docker_container_cache.go    # 容器缓存
│   │   └── util.go                       # 工具函数
│   └── model/
│       └── aenvhub_env.go         # 数据模型
├── go.mod                         # Go 模块定义
├── Dockerfile                     # Docker 镜像构建
├── Makefile                       # 构建脚本
└── README.md                      # 本文档
```

## 快速开始

### 构建

```bash
cd docker-controller
go mod tidy
go build -o docker-controller ./cmd/main.go
```

### 运行

```bash
# 使用默认端口 8080
./docker-controller --server-port=8080

# 或使用自定义 Docker socket
DOCKER_HOST=unix:///var/run/docker.sock ./docker-controller --server-port=8080
```

### Docker 镜像构建

```bash
docker build -t docker-controller:latest -f Dockerfile .
```

## API 端点

- `POST /pods` - 创建容器
- `GET /pods` - 列出所有容器
- `GET /pods/{id}` - 获取容器信息
- `DELETE /pods/{id}` - 删除容器
- `GET /healthz` - 健康检查
- `GET /readyz` - 就绪检查

## 配置

### 环境变量

- `DOCKER_HOST`: Docker daemon socket 地址（默认: `unix:///var/run/docker.sock`）
- `SERVER_PORT`: HTTP 服务器端口（默认: `8080`）

### 命令行参数

- `--server-port`: HTTP 服务器端口

## 使用示例

### 创建容器

```bash
curl -X POST http://localhost:8080/pods \
  -H "Content-Type: application/json" \
  -d '{
    "name": "test-env",
    "version": "1.0.0",
    "artifacts": [
      {
        "type": "image",
        "content": "nginx:alpine"
      }
    ],
    "deployConfig": {
      "cpu": "1C",
      "memory": "128M",
      "ttl": "10m",
      "environmentVariables": {
        "TEST_VAR": "test_value"
      }
    }
  }'
```

### 查询容器

```bash
curl http://localhost:8080/pods/test-env-xxxxxx
```

### 列出容器

```bash
curl http://localhost:8080/pods
```

### 删除容器

```bash
curl -X DELETE http://localhost:8080/pods/test-env-xxxxxx
```

## 与 API Service 集成

Docker Controller 实现了与 `api-service/service/env_instance.go` 中定义的接口相同的 HTTP API，可以直接替换 Kubernetes Controller：

```go
// 在 api-service 中配置
controllerURL := "http://docker-controller:8080"
envInstanceClient := service.NewEnvInstanceClient(controllerURL)
```

## 开发

### 运行测试

```bash
go test ./...
```

### 代码格式化

```bash
go fmt ./...
```

### 代码检查

```bash
go vet ./...
```

## 许可证

Copyright 2025. Licensed under the Apache License, Version 2.0.

