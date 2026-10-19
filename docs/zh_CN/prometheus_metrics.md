# Tunasync Prometheus 指标文档

本文档描述了 tunasync manager 暴露的 Prometheus 指标。这些指标可用于监控镜像同步状态、worker 健康状况和调度情况。

## 启用指标端点

默认情况下，Prometheus 指标端点是**禁用**的。要启用它，请在 manager 配置文件中添加：

```toml
[server]
addr = "127.0.0.1"
port = 14242
enable_metrics = true  # 启用 /metrics 端点
```

启用后，可通过 `http://addr:port/metrics` 查询指标。

---

## 指标列表

### Worker 指标

#### `tunasync_worker_registered`
- **类型**: Gauge
- **描述**: 注册的 worker 数量
- **示例**:
  ```
  tunasync_worker_registered 5
  ```

#### `tunasync_worker_last_online_timestamp{worker_id="<id>"}`
- **类型**: GaugeVec
- **描述**: 每个 worker 最后在线时间的 Unix 时间戳
- **标签**: 
  - `worker_id`: worker 的唯一标识符
- **示例**:
  ```
  tunasync_worker_last_online_timestamp{worker_id="worker1"} 1724851200
  ```

---

### Mirror 指标

#### `tunasync_mirror_job_status{mirror_name="<name>",worker_id="<id>",is_master="true|false"}`
- **类型**: GaugeVec
- **描述**: mirror 任务的当前状态
- **标签**:
  - `mirror_name`: mirror 的名称（如 `archlinux`, `debian`）
  - `worker_id`: 负责该 mirror 的 worker ID
  - `is_master`: 是否为 master 镜像（`true` 或 `false`）
- **状态值**:
  - `0`: none（无状态）
  - `1`: failed（同步失败）
  - `2`: success（同步成功）
  - `3`: syncing（正在同步）
  - `4`: pre-syncing（准备同步）
  - `5`: paused（已暂停）
  - `6`: disabled（已禁用）
- **示例**:
  ```
  tunasync_mirror_job_status{is_master="true",mirror_name="archlinux",worker_id="worker1"} 2
  tunasync_mirror_job_status{is_master="false",mirror_name="debian",worker_id="worker1"} 3
  ```

#### `tunasync_mirror_last_update_timestamp{mirror_name="<name>",worker_id="<id>"}`
- **类型**: GaugeVec
- **描述**: 最后一次成功更新的 Unix 时间戳
- **标签**:
  - `mirror_name`: mirror 的名称
  - `worker_id`: 负责该 mirror 的 worker ID
- **示例**:
  ```
  tunasync_mirror_last_update_timestamp{mirror_name="archlinux",worker_id="worker1"} 1724851200
  ```

#### `tunasync_mirror_last_started_timestamp{mirror_name="<name>",worker_id="<id>"}`
- **类型**: GaugeVec
- **描述**: 最后一次同步开始时间的 Unix 时间戳
- **标签**:
  - `mirror_name`: mirror 的名称
  - `worker_id`: 负责该 mirror 的 worker ID
- **示例**:
  ```
  tunasync_mirror_last_started_timestamp{mirror_name="archlinux",worker_id="worker1"} 1724850000
  ```

#### `tunasync_mirror_last_ended_timestamp{mirror_name="<name>",worker_id="<id>"}`
- **类型**: GaugeVec
- **描述**: 最后一次同步结束时间的 Unix 时间戳
- **标签**:
  - `mirror_name`: mirror 的名称
  - `worker_id`: 负责该 mirror 的 worker ID
- **示例**:
  ```
  tunasync_mirror_last_ended_timestamp{mirror_name="archlinux",worker_id="worker1"} 1724851200
  ```

#### `tunasync_mirror_next_schedule_timestamp{mirror_name="<name>",worker_id="<id>"}`
- **类型**: GaugeVec
- **描述**: 下次计划同步时间的 Unix 时间戳
- **标签**:
  - `mirror_name`: mirror 的名称
  - `worker_id`: 负责该 mirror 的 worker ID
- **示例**:
  ```
  tunasync_mirror_next_schedule_timestamp{mirror_name="archlinux",worker_id="worker1"} 1724937600
  ```

#### `tunasync_mirror_is_master{mirror_name="<name>",worker_id="<id>"}`
- **类型**: GaugeVec
- **描述**: 标识该 mirror 是否为 master（`1`=master，`0`=slave）
- **标签**:
  - `mirror_name`: mirror 的名称
  - `worker_id`: 负责该 mirror 的 worker ID
- **示例**:
  ```
  tunasync_mirror_is_master{mirror_name="archlinux",worker_id="worker1"} 1
  ```

#### `tunasync_mirror_size_gb{mirror_name="<name>",worker_id="<id>"}`
- **类型**: GaugeVec
- **描述**: mirror 的大小（单位：*GB*）。如果无法解析大小则默认为 `0`
- **标签**:
  - `mirror_name`: mirror 的名称
  - `worker_id`: 负责该 mirror 的 worker ID
- **示例**:
  ```
  tunasync_mirror_size_gb{mirror_name="archlinux",worker_id="worker1"} 50
  tunasync_mirror_size_gb{mirror_name="debian",worker_id="worker1"} 100
  ```

#### `tunasync_mirror_last_sync_duration_seconds{mirror_name="<name>",worker_id="<id>"}`
- **类型**: GaugeVec
- **描述**: 最后一次同步的耗时（单位：秒）
- **标签**:
  - `mirror_name`: mirror 的名称
  - `worker_id`: 负责该 mirror 的 worker ID
- **示例**:
  ```
  tunasync_mirror_last_sync_duration_seconds{mirror_name="archlinux",worker_id="worker1"} 3600
  ```

---

### 聚合指标

#### `tunasync_total_mirrors`
- **类型**: Gauge
- **描述**: 管理的 mirror 总数
- **示例**:
  ```
  tunasync_total_mirrors 150
  ```

#### `tunasync_mirrors_by_status{status="<status>"}`
- **类型**: GaugeVec
- **描述**: 按状态分类的 mirror 数量
- **标签**:
  - `status`: mirror 的状态（`none`, `failed`, `success`, `syncing`, `pre-syncing`, `paused`, `disabled`）
- **示例**:
  ```
  tunasync_mirrors_by_status{status="success"} 120
  tunasync_mirrors_by_status{status="syncing"} 5
  tunasync_mirrors_by_status{status="failed"} 3
  tunasync_mirrors_by_status{status="disabled"} 10
  ```
