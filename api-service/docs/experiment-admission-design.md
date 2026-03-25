# Experiment-Level Resource Admission Control — 方案设计与比较

## 问题背景

当前 AEnvironment 沙箱集群提供的是 **env instance 级别的 API**。上层 RL 训练实验和评测实验在多个 step 中批量创建 env instance，用完后释放，呈现锯齿波形 (sawtooth pattern):

```text
实验A:  ████░░░░████░░░░████░░░░
实验B:       ████░░░░████░░░░████
实验C:            ████░░░░████░░░░
          ↑ 三个波峰重叠 → 资源耗尽 → 全部崩溃
```

当多个实验的波峰重叠时，触发 huse-scheduler 的**全局**水位限制 (`ClusterResourceUtil >= ClusterHealthyWaterMark`)，导致**所有实验同时失败**，爆炸半径过大。

**核心需求**: 实验间资源隔离 — 先到的实验保障资源，后到的实验在资源不足时被优雅拒绝，而非所有实验一起崩溃。

---

## 方案一：显式 Experiment API（注册制）

### 设计概述

新增 experiment 级别的 API，实验在使用沙箱之前必须先注册，声明资源 quota。系统预留资源后，保证已注册实验始终有资源可用。

```text
┌─────────────────────────────────────────────────┐
│  实验生命周期                                      │
│                                                   │
│  POST /experiment                                 │
│    { "experiment_id": "rl-train-001",            │
│      "quota": { "cpu": 8000, "instances": 4 } }  │
│    → 201: 注册成功，资源已预留                       │
│    → 429: 集群资源不足，无法注册                      │
│                                                   │
│  POST /env-instance                              │
│    { "labels": {"experiment": "rl-train-001"} }   │
│    → 200: 在已预留 quota 内创建                     │
│    → 403: 超出该实验 quota 上限                     │
│                                                   │
│  DELETE /experiment/rl-train-001                  │
│    → 200: 释放预留资源                             │
└─────────────────────────────────────────────────┘
```

### 数据模型

```go
type Experiment struct {
    ID          string            `json:"id"`
    Quota       ResourceQuota     `json:"quota"`
    Status      string            `json:"status"`      // "active" | "draining" | "terminated"
    CurrentUsed ResourceUsage     `json:"current_used"`
    CreatedAt   time.Time         `json:"created_at"`
    Owner       string            `json:"owner"`
    Labels      map[string]string `json:"labels"`
}

type ResourceQuota struct {
    MaxInstances int   `json:"max_instances"`
    CPUMilli     int64 `json:"cpu_milli"`
    MemoryMB     int64 `json:"memory_mb"`
}
```

### 准入流程

```text
POST /experiment (注册)
    → 检查: cluster_total - Σ(已注册实验 quota) >= 请求 quota?
    → YES: 预留资源，返回 201
    → NO:  返回 429 "Insufficient cluster capacity"

POST /env-instance (创建实例)
    → 中间件检查: 该实验 current_used < quota?
    → YES: 允许创建
    → NO:  返回 403 "Experiment quota exceeded"

DELETE /experiment (去注册)
    → 标记 status = "draining"
    → 等待所有 instance 释放 (或强制清理)
    → 释放预留资源
```

### 存储需求

需要持久化实验注册信息（Redis 或 DB），因为 api-service 重启后需恢复预留状态。

---

## 方案二：隐式 Label 追踪（无注册）

### 方案二概述

不新增 experiment API。api-service 根据 `CreateEnvInstance` 请求中携带的 `experiment` label 自动识别实验，通过历史 instance 峰值来计算预留资源，在资源不足时只拒绝新加入的实验。

```text
┌──────────────────────────────────────────────────────┐
│  准入决策流程 (ExperimentAdmission middleware)          │
│                                                       │
│  POST /env-instance                                   │
│    { "labels": {"experiment": "rl-train-001"} }       │
│                                                       │
│  1. 提取 experiment label                              │
│  2. 是否已知实验? (有活跃 instance)                      │
│     → YES: 直接放行 (先来先得保护)                       │
│     → NO:  进入准入检查                                 │
│  3. 计算: reserved = Σ(各实验历史峰值 × 单实例CPU)       │
│           available = cluster_total - reserved         │
│  4. available > per_instance_cpu ?                     │
│     → YES: 放行新实验                                  │
│     → NO:  返回 429 拒绝                               │
└──────────────────────────────────────────────────────┘
```

### 数据模型（已实现）

```go
type ExperimentState struct {
    FirstSeen    time.Time
    CurrentCount int           // 当前活跃 instance 数
    PeakCount    int           // 滑动窗口内峰值
    PeakSamples  []PeakSample  // 峰值采样环形缓冲区
}
```

### 核心公式

```text
reserved_capacity = Σ (各活跃实验的历史峰值 instance 数 × 单实例 CPU)
available_for_new = cluster_total - reserved_capacity
新实验准入条件:   available_for_new > per_instance_cpu
```

### 去注册机制

通过**滑动窗口衰减**自动去注册：

- 实验无活跃 instance 后，历史峰值采样随时间过期
- 窗口（默认 15m）内所有采样过期后，实验记录自动删除
- 预留资源随之释放，新实验可进入

### 方案二存储需求

纯内存状态，无持久化依赖。api-service 重启后通过 `ListEnvInstances` 在 ~5m 内自动重建。

---

## 方案对比

| 维度 | 方案一：显式 API | 方案二：隐式追踪 |
|------|-----------------|-----------------|
| **上层改造** | 需要改造。SWEAgent 等调用方需新增 register/deregister 调用 | **零改造**。只需 label 中携带 experiment 字段（已支持） |
| **Quota 精确性** | 精确。用户声明多少就预留多少 | 近似。基于历史峰值推断，首次创建时无历史数据 |
| **资源利用率** | **较低**。预留即锁定，即使实验处于 inference 低谷期资源也不释放 | **较高**。预留跟随实际用量的峰值，低谷期其他实验可使用 |
| **去注册** | 需要调用方主动 DELETE，否则资源泄漏 | **自动**。滑动窗口衰减，无需人工干预 |
| **资源泄漏风险** | **高**。调用方 crash / 忘记去注册 → 永久占用 | **低**。自动衰减机制保证最终释放 |
| **Quota 评估负担** | **高**。用户需要预估峰值实例数，估大浪费，估小不够 | **无**。系统自动追踪 |
| **首次创建保护** | 注册时即保护 | 首次创建时无历史数据，靠 fail-open 放行 |
| **多副本一致性** | 需要**共享存储**（Redis/DB）同步实验注册状态 | 每个副本独立状态，保守估计（可接受） |
| **实现复杂度** | **高**。新增 API + 持久化 + 去注册超时清理 + SDK 适配 | **低**。中间件 + 内存状态，~200 行核心代码 |
| **API 兼容性** | 破坏性变更：新增必须调用的 API | **完全兼容**：现有 API 不变 |
| **Feature gate** | 需要，切换复杂 | 简单 `--admission-enabled` 开关 |
| **故障模式** | 注册服务不可用 → 新实验无法创建 | scheduler 不可用 → fail-open 放行所有请求 |

---

## 关键场景分析

### 场景 1：正常运行 — 3 个实验交错运行

| | 方案一 | 方案二 |
|---|---|---|
| 效果 | 三个实验各自在 quota 内运行 | 三个实验被自动识别，各自峰值被追踪 |
| 利用率 | 低：每个实验预留的 quota 在低谷期闲置 | 高：低谷期资源可被其他实验的突发使用 |

### 场景 2：资源紧张 — 第 4 个实验尝试加入

| | 方案一 | 方案二 |
|---|---|---|
| 效果 | 注册时直接拒绝 (429) | 创建第一个 instance 时拒绝 (429) |
| 时机 | 更早（注册阶段） | 稍晚（首次创建阶段） |
| 上层感知 | 需处理注册失败 | 需处理创建失败（已有错误处理路径） |

### 场景 3：实验异常退出（调用方 crash）

| | 方案一 | 方案二 |
|---|---|---|
| 效果 | **资源泄漏**，需额外超时清理机制 | 15m 窗口后自动释放，instance 由 TTL cleanup 回收 |
| 恢复时间 | 取决于超时清理间隔 | 等于滑动窗口 (15m) |

### 场景 4：api-service 重启

| | 方案一 | 方案二 |
|---|---|---|
| 恢复 | 从 Redis/DB 读取注册状态，立即可用 | ~5m 内通过 ListEnvInstances 重建，期间 fail-open |
| 风险 | 存储不可用时无法恢复 | 短暂窗口内可能过度放行 |

### 场景 5：用户 Quota 评估不准

| | 方案一 | 方案二 |
|---|---|---|
| 估大 | 资源浪费，其他实验无法注册 | N/A |
| 估小 | 实验运行中超额被拒，需重新注册 | N/A |
| 自适应 | 不支持 | **自动适应**实际使用模式 |

---

## 综合评估

### 方案一的优势场景

- **合规/企业级多租户**：需要严格的资源隔离和 quota 审计
- **资源计费**：按预留 quota 计费的商业模式
- **高确定性**：对 SLA 要求极高，不允许任何概率性行为

### 方案二的优势场景

- **内部 RL 训练平台**（当前场景）：快速迭代，上层不想改代码
- **资源利用率敏感**：集群资源有限，不允许空预留
- **实验行为不可预测**：无法准确预估 quota
- **快速上线**：零上层改造，feature gate 即开即关

---

## 推荐

对于当前 RL 训练场景，**方案二（隐式追踪）更适合**，原因：

1. **零改造成本** — SWEAgent 等上层系统不需要任何修改，已经通过 label 携带 experiment 信息
2. **资源利用率高** — RL 实验的锯齿波特征意味着方案一会有大量空闲预留
3. **自适应** — 不需要用户评估 quota，系统自动追踪实际使用模式
4. **容错性好** — 实验 crash 后自动释放，不存在资源泄漏
5. **已部分实现** — 核心代码已就绪，feature gate 可控

### 可选增强（渐进演进到混合模式）

方案二可以渐进增强为"混合模式"，在不破坏现有行为的前提下叠加方案一的能力：

- 新增**可选的** `POST /experiment` 注册 API，允许预声明 quota
- 有注册的实验：按声明的 quota 预留（方案一行为）
- 无注册的实验：按历史峰值追踪（方案二行为）
- 这样既保持向后兼容，又为有高 SLA 需求的实验提供确定性保障

---

## 附录：方案二已实现组件

| 文件 | 说明 |
|------|------|
| `service/experiment_admission.go` | 核心准入控制服务（峰值追踪、集群资源轮询、准入决策） |
| `service/experiment_admission_test.go` | 13 个单元测试（准入逻辑、滑动窗口、并发安全、容错降级） |
| `middleware/experiment_admission.go` | Gin 中间件（提取 experiment label、调用准入检查） |
| `metrics/admission_metrics.go` | Prometheus 指标（admission_total、reserved_capacity、experiment_count、peak_instances） |
| `main.go` | CLI flags 和组件接线（`--admission-enabled`、`--scheduler-addr`、`--per-instance-cpu`、`--peak-window`） |

### 启用方式

```bash
api-service \
  --admission-enabled \
  --scheduler-addr=http://huse-scheduler-0:14457 \
  --per-instance-cpu=2000 \
  --peak-window=15m
```
