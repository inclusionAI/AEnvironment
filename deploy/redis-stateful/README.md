# Redis StatefulSet Helm Chart

基于StatefulSet的Redis主从部署方案，提供稳定的有状态服务。

## 特性

- 使用StatefulSet确保稳定的网络标识和持久化存储
- 支持Redis主从复制架构
- 通过Headless Service实现Pod间直接通信
- 持久化存储支持
- 可配置的Redis密码和资源配置

## 快速开始

### 1. 安装Chart

```bash
# 安装到默认命名空间
helm install redis-stateful ./redis-stateful

# 安装到指定命名空间
helm install redis-stateful ./redis-stateful -n your-namespace
```

### 2. 验证部署

```bash
# 查看Pod状态
kubectl get pods -l app.kubernetes.io/name=redis-stateful

# 查看服务
kubectl get svc -l app.kubernetes.io/name=redis-stateful
```

### 3. 连接Redis

```bash
# 连接主节点
kubectl exec -it redis-stateful-master-0 -- redis-cli -a yourpassword

# 连接从节点
kubectl exec -it redis-stateful-slave-0 -- redis-cli -a yourpassword
```

## 配置参数

### Redis配置

| 参数 | 描述 | 默认值 |
|------|------|--------|
| `redis.password` | Redis密码 | `yourpassword` |
| `redis.master.replicas` | 主节点副本数 | `1` |
| `redis.slave.replicas` | 从节点副本数 | `2` |

### 资源限制

| 参数 | 描述 | 默认值 |
|------|------|--------|
| `redis.master.resources.limits.cpu` | 主节点CPU限制 | `1` |
| `redis.master.resources.limits.memory` | 主节点内存限制 | `1Gi` |
| `redis.slave.resources.limits.cpu` | 从节点CPU限制 | `1` |
| `redis.slave.resources.limits.memory` | 从节点内存限制 | `1Gi` |

### 持久化存储

| 参数 | 描述 | 默认值 |
|------|------|--------|
| `persistence.enabled` | 是否启用持久化 | `true` |
| `persistence.size` | 存储大小 | `5Gi` |
| `persistence.storageClassName` | 存储类名称 | `""` |

## 自定义配置

### 修改Redis密码

```bash
helm upgrade redis-stateful ./redis-stateful --set redis.password=newpassword
```

### 调整资源配置

创建自定义values.yaml文件：

```yaml
redis:
  password: "mypassword"
  master:
    resources:
      limits:
        cpu: "2"
        memory: "2Gi"
  slave:
    replicas: 3
    resources:
      limits:
        cpu: "1.5"
        memory: "1.5Gi"

persistence:
  size: 10Gi
  storageClassName: "fast-ssd"
```

然后安装：

```bash
helm install redis-stateful ./redis-stateful -f custom-values.yaml
```

## 验证主从复制

```bash
# 在主节点写入数据
kubectl exec -it redis-stateful-master-0 -- redis-cli -a yourpassword SET testkey "hello"

# 在从节点读取数据
kubectl exec -it redis-stateful-slave-0 -- redis-cli -a yourpassword GET testkey

# 查看复制状态
kubectl exec -it redis-stateful-slave-0 -- redis-cli -a yourpassword INFO replication
```

## 故障排查

### 查看Pod日志

```bash
kubectl logs redis-stateful-master-0
kubectl logs redis-stateful-slave-0
```

### 检查服务状态

```bash
kubectl describe svc redis-headless
kubectl describe statefulset redis-stateful-master
kubectl describe statefulset redis-stateful-slave
```

### 常见问题

1. **从节点无法连接主节点**：检查网络连通性和密码配置
2. **持久化存储问题**：确认StorageClass和权限配置
3. **资源不足**：调整resources配置或节点资源

## 卸载

```bash
helm uninstall redis-stateful
```

## 架构说明

- **Master StatefulSet**: 负责写操作，1个副本
- **Slave StatefulSet**: 负责读操作，2个副本（可配置）
- **Headless Service**: 提供稳定的网络标识，支持Pod间直接通信
- **Persistent Volume**: 确保数据持久化
- **ConfigMap**: 管理Redis配置文件
- **Secret**: 安全存储Redis密码
