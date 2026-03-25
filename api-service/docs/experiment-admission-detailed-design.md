# Experiment-Level Resource Admission Control — 详细设计方案

> 基于方案二（隐式 Label 追踪）+ 二级限流增强

## 1. 问题与目标

### 1.1 问题

RL 训练实验在多个 step 中批量创建 env instance，呈锯齿波形。多个实验波峰重叠时，触发 huse-scheduler 全局水位限制，导致**所有实验同时失败**。

```text
实验A:  ████░░░░████░░░░████
实验B:       ████░░░░████░░░░
实验C:            ████░░░░████
              ↑ 波峰重叠 → 全局崩溃
```

### 1.2 目标

- **先来先得保障**：已有实验的资源不受新实验影响
- **二级限流**：集群利用率超过水位时，逐级收紧准入
- **元数据规范化**：强制要求携带可配置的标签字段
- **全功能可选**：feature gate 控制，默认关闭，零影响现有行为

---

## 2. 优先级模型

所有 `POST /env-instance` 请求根据标签分为三个优先级：

| 优先级 | 名称 | 条件 | 准入策略 |
|--------|------|------|----------|
| **P0** | 已知实验 | experiment label 存在，且该实验已有活跃 instance | **始终放行**（核心保障） |
| **P1** | 新实验 | experiment label 存在，但该实验无活跃 instance 记录 | 集群利用率 < watermark **且** 预留容量有余量时放行 |
| **P2** | 无标签 | 缺少必需 label（可配置，默认要求 `experiment`） | **直接拒绝** (429) |

### 决策流程图

```text
POST /env-instance 到达
    │
    ▼
[提取 labels]
    │
    ├── 缺少必需 label? ──YES──→ 429 "Missing required labels: experiment"  (P2)
    │
    ▼
[查找 experiment 状态]
    │
    ├── 已知实验? (有活跃 instance) ──YES──→ ✅ ALLOW  (P0)
    │
    ▼
[新实验准入检查]
    │
    ├── cluster_used / cluster_total >= watermark?
    │       ──YES──→ 429 "Cluster utilization above watermark"  (P1 rejected)
    │
    ├── cluster_total - reserved_capacity <= per_instance_cpu?
    │       ──YES──→ 429 "Insufficient capacity for new experiment"  (P1 rejected)
    │
    └── ✅ ALLOW  (P1 admitted)
```

### P1 双重门控说明

新实验（P1）必须同时通过两道检查：

1. **水位检查** `cluster_used / cluster_total < watermark`
   - 基于 scheduler 返回的**实际集群利用率**
   - 防止在集群已经繁忙时放入新实验
   - 捕获非沙箱负载（系统 pod、其他租户）导致的资源紧张

2. **预留容量检查** `cluster_total - reserved_capacity > per_instance_cpu`
   - 基于已追踪实验的**历史峰值预留**
   - 确保为每个已有实验保留足够的峰值资源
   - 即使实际利用率暂时较低（实验处于低谷），也不会过度承诺

两者互补：水位检查反映集群**当前真实状态**，预留容量检查反映**前瞻性承诺**。

---

## 3. 数据源

### 3.1 集群资源（来自 faas-api-service 统一接口）

#### 架构背景

huse-scheduler 是**分区级组件**，每个 scheduler 实例只管理一个分区的节点子集：

```text
faas-api-service ──gRPC──→ huse-coordinator ──路由──→ huse-scheduler-0 (分区 A)
                                              ├──→ huse-scheduler-1 (分区 B)
                                              └──→ huse-scheduler-2 (分区 C)
```

- 每个 scheduler 暴露 `:14457/clusterresource` 返回**该分区**的资源状态
- huse-coordinator 已经周期性轮询每个 scheduler 的 `/clusterresource` 并缓存在 `SchedulerCache.State.ClusterResource`
- **不能直接轮询单个 scheduler**，否则只看到一个分区的数据

#### 解决方案：faas-api-service 新增统一聚合接口

在 faas-api-service（faas-apiserver）中新增 HTTP 接口，聚合所有 scheduler 分区的资源数据：

```json
// GET http://faas-api-service:8233/hapis/faas.hcs.io/v1/clusterinfo
→ {
    "success": true,
    "data": {
      "totalCPU": 600000,     // 所有分区 TotalCPU 之和
      "usedCPU": 420000,      // 所有分区 UsedCPU 之和
      "freeCPU": 180000,      // 所有分区 FreeCPU 之和
      "totalMemory": 1200000, // 所有分区 TotalMemory 之和
      "usedMemory": 840000,
      "freeMemory": 360000,
      "partitions": [         // 可选：每分区明细
        {"name": "scheduler-0", "totalCPU": 200000, "usedCPU": 140000, "healthy": true},
        {"name": "scheduler-1", "totalCPU": 200000, "usedCPU": 140000, "healthy": true},
        {"name": "scheduler-2", "totalCPU": 200000, "usedCPU": 140000, "healthy": true}
      ],
      "healthyPartitions": 3,
      "totalPartitions": 3
    }
  }
```

**实现方式**：faas-api-service 通过 gRPC 连接 huse-coordinator，coordinator 已持有所有 scheduler 的 `SchedulerCache`。新增接口从 coordinator 获取（或 faas-api-service 自身缓存）所有分区的聚合资源。

#### api-service 侧

- api-service 的 `ExperimentAdmission` 轮询 faas-api-service 的 `/clusterinfo` 聚合接口（而非直接连 scheduler）
- 轮询间隔：10s
- 用途：计算 `utilization = UsedCPU / TotalCPU`（全局聚合值）
- 容错：faas-api-service 不可达时保留最后已知值，无数据时 fail-open
- 仅使用健康分区的聚合数据（`healthyPartitions > 0`）

### 3.2 实验实例数（来自 ListEnvInstances）

- 数据源：现有 `startUnifiedPeriodicTask` 每 5m 调用 `ListEnvInstances("")`
- 按 `labels["experiment"]` 分组统计活跃 instance 数
- 排除 `Terminated` / `Failed` 状态的 instance
- 滑动窗口（默认 15m）内追踪每个实验的峰值 instance 数

---

## 4. 核心数据结构

### 4.1 ExperimentAdmission（增强后）

```go
type ExperimentAdmission struct {
    mu             sync.RWMutex
    experiments    map[string]*ExperimentState

    // Cluster resource (from scheduler polling)
    clusterTotal   int64    // TotalCPU (milli)
    clusterUsed    int64    // UsedCPU (milli)
    hasClusterData bool

    // Configuration
    perInstanceCPU    int64           // per-instance CPU (milli), default 2000
    peakWindow        time.Duration   // sliding window, default 15m
    watermark         float64         // utilization threshold, default 0.7
    requiredLabels    []string        // required labels, default ["experiment"]
    clusterInfoEndpoint string          // faas-api-service /clusterinfo URL
    pollInterval        time.Duration   // default 10s

    httpClient *http.Client
}
```

### 4.2 ExperimentState（不变）

```go
type ExperimentState struct {
    FirstSeen    time.Time
    CurrentCount int
    PeakCount    int
    PeakSamples  []PeakSample
}
```

### 4.3 准入结果

```go
type AdmissionResult struct {
    Allowed  bool
    Reason   string
    Tier     string  // "p0_known", "p1_new", "p2_unlabeled"
}
```

---

## 5. 核心公式

```text
utilization    = cluster_used / cluster_total          (实际利用率)
reserved       = Σ (experiment.PeakCount × perInstanceCPU)  (已承诺容量)
available      = cluster_total - reserved              (可分配余量)

P1 准入条件:  utilization < watermark  AND  available > perInstanceCPU
P0 准入条件:  always true
P2 准入条件:  always false (缺少必需 label)
```

---

## 6. 配置参数

| Flag | 类型 | 默认值 | 说明 |
|------|------|--------|------|
| `--admission-enabled` | bool | `false` | 功能总开关。关闭时所有请求直接放行，零额外开销 |
| `--scheduler-addr` | string | `""` | 集群资源 API 地址。FaaS 模式下指向 faas-api-service（如 `http://faas-api-service:8233`），复用 `--schedule-addr` 即可 |
| `--per-instance-cpu` | int64 | `2000` | 单实例 CPU（milli-cores），匹配 RL 沙箱规格 |
| `--peak-window` | duration | `15m` | 峰值滑动窗口，覆盖 3-5 个 RL 推理-执行周期 |
| `--admission-watermark` | float64 | `0.7` | 集群利用率阈值，超过则拒绝新实验 |
| `--admission-required-labels` | string | `"experiment"` | 逗号分隔的必需 label 列表，缺少任一则拒绝 |

### 配置示例

```bash
# 基本启用（FaaS 模式，复用 --schedule-addr 指向 faas-api-service）
api-service --admission-enabled --schedule-addr=http://faas-api-service:8233

# 严格模式：要求 experiment+owner，60% 水位即收紧
api-service \
  --admission-enabled \
  --schedule-addr=http://faas-api-service:8233 \
  --admission-watermark=0.6 \
  --admission-required-labels=experiment,owner \
  --per-instance-cpu=2000 \
  --peak-window=15m

# 禁用（默认）：完全不影响现有行为
api-service  # admission-enabled 默认 false
```

---

## 7. 中间件位置

在现有中间件链中的位置不变：

```text
POST /env-instance
  → AuthTokenMiddleware        (身份认证)
  → InstanceLimitMiddleware    (Token 级别的实例配额)
  → ExperimentAdmissionMiddleware   ← 准入控制
  → RateLimit                  (全局 QPS 限流)
  → CreateEnvInstance          (业务逻辑)
```

### 中间件行为

```go
func ExperimentAdmissionMiddleware(admission *ExperimentAdmission) gin.HandlerFunc {
    return func(c *gin.Context) {
        // Feature gate
        if admission == nil {
            c.Next()
            return
        }

        // 1. Peek request body, extract labels
        labels := extractLabelsFromRequest(c)

        // 2. Check required labels (P2 gate)
        if missing := admission.CheckRequiredLabels(labels); len(missing) > 0 {
            // 429, reject, metric p2_rejected
            return
        }

        // 3. Admission decision (P0/P1)
        result := admission.ShouldAdmit(labels["experiment"])
        if !result.Allowed {
            // 429, reject, metric by tier
            return
        }

        // metric allowed by tier
        c.Next()
    }
}
```

---

## 8. 滑动窗口与自动去注册

### 8.1 峰值追踪

每次 `ListEnvInstances` 返回时（每 5m）：

```text
对每个 experiment:
  1. 统计活跃 instance 数 → currentCount
  2. 记录 PeakSample{now, currentCount}
  3. 淘汰窗口外的旧采样
  4. 重新计算 peakCount = max(窗口内所有采样)
```

### 8.2 自动去注册

```text
实验释放所有 instance 后:
  → 不再产生新的非零采样
  → 旧采样逐渐过期 (15m 窗口)
  → peakCount 衰减到 0
  → 实验记录自动删除
  → 预留资源释放，新实验可进入
```

时序示意：

```text
t=0    实验结束，所有 instance 释放
t=5m   UpdateExperimentCounts: currentCount=0, 窗口内仍有旧峰值采样
t=10m  UpdateExperimentCounts: currentCount=0, 旧采样继续过期
t=15m  UpdateExperimentCounts: 所有采样过期, peakCount=0 → 删除实验记录
```

---

## 9. 容错设计

| 故障场景 | 行为 | 原因 |
|----------|------|------|
| faas-api-service 不可达 | 保留最后已知的 cluster_used/cluster_total，继续使用 | stale data 优于无 data |
| 部分 scheduler 分区不健康 | faas-api-service 仅聚合健康分区数据，返回中包含健康分区数 | 局部故障不影响准入决策 |
| 首次启动无数据 | fail-open，所有请求放行 | 避免冷启动阻塞所有业务 |
| api-service 重启 | ~5m 内通过 ListEnvInstances 自动重建实验状态，期间 fail-open | 无持久化依赖 |
| ListEnvInstances 失败 | 保持上一次的实验状态不变 | 已有行为 |
| 多副本独立状态 | 每个副本独立追踪，是全局的下界估计 | scheduler 全局水位是最终安全网 |

---

## 10. 可观测性

### 10.1 Prometheus 指标

| 指标名 | 类型 | 标签 | 说明 |
|--------|------|------|------|
| `aenv_api_experiment_admission_total` | Counter | `decision={allowed,rejected}`, `tier={p0_known,p1_new,p2_unlabeled}` | 各优先级的准入决策计数 |
| `aenv_api_experiment_admission_watermark_ratio` | Gauge | — | 当前集群利用率 (`used/total`) |
| `aenv_api_experiment_reserved_capacity` | Gauge | — | 总预留 CPU (milli) |
| `aenv_api_experiment_count` | Gauge | — | 活跃实验数 |
| `aenv_api_experiment_peak_instances` | GaugeVec | `experiment` | 每实验的滑动窗口峰值 |

### 10.2 日志

```text
# P2 拒绝
WARN Experiment admission: rejected request missing required labels [experiment] (tier=p2)

# P1 新实验被水位拒绝
WARN Experiment admission: rejected new experiment "rl-train-005"
     cluster_utilization=0.73 watermark=0.70 (tier=p1, reason=watermark)

# P1 新实验被容量拒绝
WARN Experiment admission: rejected new experiment "rl-train-005"
     reserved=180000 available=20000 required=2000 (tier=p1, reason=capacity)

# P0 已知实验放行
DEBUG Experiment admission: allowed known experiment "rl-train-001" (tier=p0)
```

### 10.3 Debug 端点

`GET /metrics` 中包含上述 Prometheus 指标。`ExperimentAdmission.GetMetrics()` 返回完整内存状态，可用于排查。

---

## 11. 边界情况

| 场景 | 处理 |
|------|------|
| experiment label 为空字符串 `""` | 视为缺少 label → P2 拒绝 |
| 同一 experiment ID 不同 owner | 合并为同一实验追踪（experiment 是分组 key） |
| 单实验独占整个集群 | P0 始终放行，不限制已有实验（这是设计意图） |
| watermark 设为 1.0 | 等效于禁用水位检查，仅保留预留容量检查 |
| watermark 设为 0.0 | 永远拒绝新实验（仅已知实验可运行） |
| required-labels 为空 | 不做 label 检查，所有请求至少为 P1 |
| api-service 多副本状态不一致 | 每个副本的峰值追踪是全局的下界，保守方向安全；scheduler 水位是共享的真实值 |
| 部分 scheduler 分区不健康 | faas-api-service 仅聚合健康分区数据，不健康分区的资源不计入 total/used |
| 新增/缩减 scheduler 分区 | faas-api-service 通过 coordinator 自动发现，无需 api-service 侧配置变更 |

---

## 12. 实现变更清单

### 12.1 修改文件

#### api-service 变更

| 文件 | 变更 |
|------|------|
| `service/experiment_admission.go` | 新增 `watermark`、`requiredLabels` 字段；`ShouldAdmit` 增加水位检查和 tier 返回；新增 `CheckRequiredLabels` 方法；`pollClusterResource` 改为调用 faas-api-service `/clusterinfo` 聚合接口 |
| `service/experiment_admission_test.go` | 新增水位门控测试、required labels 测试、tier 分级测试 |
| `middleware/experiment_admission.go` | 中间件增加 required labels 检查，metric 区分 tier |
| `metrics/admission_metrics.go` | `ExperimentAdmissionTotal` 增加 `tier` 标签；新增 `ExperimentAdmissionWatermarkRatio` gauge |
| `main.go` | 新增 `--admission-watermark`、`--admission-required-labels` flag；`schedulerEndpoint` 改为复用 `--schedule-addr`（指向 faas-api-service） |

#### faas-api-service (faas-apiserver) 侧

| 文件 | 变更 |
|------|------|
| `pkg/httpserver/server.go` | 注册新路由 `GET /hapis/faas.hcs.io/v1/clusterinfo` |
| `pkg/controller/clusterinfo.go` | 新增 controller，调用 service 层获取聚合资源数据 |
| `pkg/service/clusterinfo.go` | 新增 service，通过 coordinator gRPC 或本地 scheduler cache 聚合所有分区的 `ClusterResource` |

> faas-api-service 已通过 `--huse-scheduler-addr` 连接 huse-coordinator。coordinator 的 `SchedulerCache` 已持有所有分区的 `ClusterResource`。新接口仅需将这些数据 sum 聚合后返回。

### 12.2 不修改

| 文件 | 原因 |
|------|------|
| api-service/controller/* | 业务逻辑不变，准入由中间件完成 |
| huse-scheduler | 每个分区已暴露 `/clusterresource`，无需改动 |
| huse-coordinator | 已周期性轮询 scheduler 资源，无需改动 |
| SDK (aenv) | 无任何改动需求 |
| SWEAgent | 无任何改动需求（已携带 experiment label） |

---

## 13. 实现步骤

```text
Phase A: faas-api-service 新增 /clusterinfo 聚合接口

Step A1: 新增 pkg/service/clusterinfo.go
         - 通过 coordinator 连接获取所有 SchedulerCache 的 ClusterResource
         - 聚合（sum）所有健康分区的 TotalCPU/UsedCPU/FreeCPU/Memory
         - 返回聚合结果 + 每分区明细 + 健康分区数

Step A2: 新增 pkg/controller/clusterinfo.go
         - GET handler，调用 service 层
         - 返回标准 JSON 响应

Step A3: 更新 pkg/httpserver/server.go
         - 注册路由 GET /hapis/faas.hcs.io/v1/clusterinfo

Step A4: 构建验证
         cd faas-apiserver && go build ./... && go test ./...

Phase B: api-service 准入控制增强

Step B1: 更新 service/experiment_admission.go
         - 添加 watermark, requiredLabels 配置
         - ShouldAdmit 增加水位检查和 tier 返回 (AdmissionResult)
         - 新增 CheckRequiredLabels 方法
         - pollClusterResource 改为调用 faas-api-service /clusterinfo

Step B2: 更新 service/experiment_admission_test.go
         - 新增 TestWatermarkBlocksNewExperiment
         - 新增 TestWatermarkAllowsKnownExperiment
         - 新增 TestRequiredLabelsMissing
         - 新增 TestAdmissionTierClassification

Step B3: 更新 middleware/experiment_admission.go
         - 提取完整 labels map（不仅是 experiment）
         - 先检查 required labels
         - 调用 ShouldAdmit，按 tier 记录 metrics

Step B4: 更新 metrics/admission_metrics.go
         - ExperimentAdmissionTotal 增加 tier 标签维度
         - 新增 ExperimentAdmissionWatermarkRatio gauge

Step B5: 更新 main.go
         - 新增 flag：--admission-watermark, --admission-required-labels
         - clusterinfo endpoint 复用 --schedule-addr

Step B6: 构建验证
         go build ./... && go vet ./... && go test ./service/ ./middleware/ -v
```

---

## 14. 部署与回滚

### 14.1 灰度上线

```bash
# Phase 0: 先部署 faas-api-service 新版（含 /clusterinfo 接口），验证接口可用
curl http://faas-api-service:8233/hapis/faas.hcs.io/v1/clusterinfo

# Phase 1: 仅开启，watermark=1.0（等效只检查 required labels + 预留容量）
--admission-enabled --admission-watermark=1.0 --admission-required-labels=experiment

# Phase 2: 收紧水位到 0.8，观察 metric
--admission-enabled --admission-watermark=0.8

# Phase 3: 目标水位 0.7
--admission-enabled --admission-watermark=0.7

# Phase 4: 如需要，增加 owner 要求
--admission-required-labels=experiment,owner
```

### 14.2 回滚

```bash
# 去掉 --admission-enabled 即可，或设为 false
# 效果：中间件检测到 admission==nil，直接 c.Next()，零额外开销
```

### 14.3 监控告警

```yaml
# 新实验被拒率过高（可能 watermark 过低）
alert: ExperimentAdmissionRejectRate
expr: rate(aenv_api_experiment_admission_total{decision="rejected",tier="p1_new"}[5m])
      / rate(aenv_api_experiment_admission_total{tier="p1_new"}[5m]) > 0.5
for: 10m

# 无标签请求仍然存在（上层未适配）
alert: UnlabeledExperimentRequests
expr: rate(aenv_api_experiment_admission_total{tier="p2_unlabeled"}[5m]) > 0
for: 30m
```
