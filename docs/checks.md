# 巡检判定规则参考

本文档描述 `add-platform-checks-coverage` 改造后引入的判定流水线、四档状态语义、
各组件判定规则速查、以及告警邮件的签名/通知决策状态机。受众是值班、二次开发、
调优的同学。运维操作向（部署、阈值速查、邮件配置）见 [README.md](../README.md)。

> 本文档**只覆盖本次改造引入的判定逻辑**。主机 CPU/Mem/Disk、复制（replication）等
> 已有逻辑见 `openspec/specs/host-metrics-collection/` 与
> `openspec/specs/replication-collection/`。

---

## 1. 流水线总览

```
原始数据                  统一判定                                出口
─────────                 ────────                                ────

主机 SSH                 ┌─ CheckHost ─────────────────────────┐
HostMetrics ─────────────┤                                      │
                         │                                      │
service 采集             ├─ CheckService           ──┐          │
ServiceModule ───────────┤  + CheckServiceContainers │          │
                         │  + CheckServiceCollectError          │
                         │                                      │
ESCluster[] ─────────────┼─ CheckES                             │
RedisNode[] ─────────────┼─ CheckRedis                          ├─► []CheckResult
SentinelClusterStatus ───┼─ CheckRedisSentinel                  │   = report.AllChecks
MongoCluster[] ──────────┼─ CheckMongo                          │
RabbitMQStatus ──────────┼─ CheckRabbitMQ                       │
BKMonitorV3Section ──────┼─ CheckBKDeps                         │
ReplicationReport ───────┘─ CheckReplication                    │
                                                                │
                                  ┌─────────────────────────────┘
                                  │
                                  ▼
                    ┌─────────────────────────────┐
                    │         三个出口            │
                    └─────────────────────────────┘

  Summary                  邮件                          HTML 着色
  ───────                  ────                          ─────────
  Summarize 计数:          ExtractAlerts:                Checker 同步在
  - OK                     仅 Status==Warn               渲染结构上回填
  - Warn                   ↓                             Status 字段
  - Unknown                Signature(items)              ↓
  - Total = OK+Warn+       ↓                             模板按 Status
       Unknown             Decide(now, prev, sig...)     调 statusClass()
  Notice 不计入            ↓                              着色（状态-类
                           SMTP Send                     的映射见 §4.4）
```

**单一事实源**：HTML 红字、Summary 计数、邮件告警三处都读 `CheckResult.Status`。
模板里**禁止**出现 `{{if gt .X N}}` / `{{if eq .X "ok"}}` 之类的内联判断；
CI 通过 `make lint` 守卫这一点。

---

## 2. 四档状态语义

| 状态      | 计入 Total | 桶       | 进邮件 | HTML class      | 视觉 |
|-----------|------------|----------|--------|-----------------|------|
| `ok`      | ✓          | OK       | —      | `status-ok`     | 绿   |
| `warn`    | ✓          | Warn     | ✓      | `status-warn`   | 红   |
| `unknown` | ✓          | Unknown  | —      | `status-na`     | 灰   |
| `notice`  | —          | （不计） | —      | `status-warn`   | 红   |

### 为什么 Warn 与 Notice 同色不同语义？

**Warn**：明确异常，需要值班响应。例：节点不可达、RabbitMQ 队列积压。

**Notice**：超阈值但本轮不视为告警。例：ES RAM 96%（阈值 95），运维心里有数即可，
不用立即起夜。区分语义的目的是允许"红字提示"而**不**让邮件变得吵闹。如果某个
Notice 项后续上升为应告警，把它在对应 checker 里改成 `StatusWarn` 即可（一处改动）。

### 为什么需要 Unknown 桶？

历史症状：`job-analysis` 子模块在某些主机上没注册，采集到的 `Status=""`。

- 旧 `CheckService` 跳过空状态 → 不进 Summary
- 旧 `notify.ExtractAlerts` 把空状态视为非 active → 进邮件
- 结果：控制台 `0 告警` 仍发邮件

修复方案是把"应该有数据没采到"明确归为 `Unknown`：进 Summary 让运维知道这个
主机这条子服务没采到，但**不**视为告警。

---

## 3. 各组件判定规则速查

每条规则的格式：`字段 / 条件 → Status`。Field 命名遵循 §5 约定。

### 3.1 Service（蓝鲸模块子服务）

来源：[checker/rules.go](../checker/rules.go) `CheckService` /
`CheckServiceContainers` / `CheckServiceCollectError`。

| 字段                          | 条件                          | Status   |
|-------------------------------|-------------------------------|----------|
| `ServiceModule.Status`        | `""`（空字符串）              | Unknown  |
| `ServiceModule.Status`        | `"active"`                    | OK       |
| `ServiceModule.Status`        | 其他                          | Warn     |
| `ServiceModule.HealthzAPI`    | `""` 或 `"N/A"`               | （不产生）|
| `ServiceModule.HealthzAPI`    | `"ok"`                        | OK       |
| `ServiceModule.HealthzAPI`    | 其他                          | Warn     |
| `ServiceStatus.Error`         | 非空（采集本身失败）           | Notice   |
| `ServiceStatus.ContainersExited` | `> Thresholds.ServiceContainersExited` | Notice |

### 3.2 Elasticsearch

来源：[checker/es.go](../checker/es.go) `CheckES`。

| 字段                                      | 条件                                  | Status  |
|-------------------------------------------|---------------------------------------|---------|
| `ESCluster.Error`                         | 非空                                  | Warn（且跳过该集群下属节点检查） |
| `ESCluster.Status`                        | `"green"`                             | OK      |
| `ESCluster.Status`                        | 非空且非 `"green"`                    | Notice  |
| `ESCluster.UnassignedShards`              | `> Thresholds.ESUnassignedShards`     | Notice  |
| `ESCluster.PendingTasks`                  | `> 0`                                 | Notice  |
| `ESNode.HeapPercent`                      | `> Thresholds.ESHeapPercent`          | Notice  |
| `ESNode.RAMPercent`                       | `> Thresholds.ESRAMPercent`           | Notice  |
| `ESNodeReach.Status`                      | `"unreachable"`                       | Warn    |

### 3.3 Redis 单点

来源：[checker/redis.go](../checker/redis.go) `CheckRedis`。

| 字段                          | 条件                                       | Status  |
|-------------------------------|--------------------------------------------|---------|
| `RedisNode.Error`             | 非空                                       | Warn    |
| `RedisNode.CeleryQueue`       | `> Thresholds.RedisCeleryQueue`            | Notice  |
| `RedisNode.MonitorQueue`      | `> Thresholds.RedisMonitorQueue`           | Notice  |

### 3.4 Redis Sentinel

来源：[checker/redis.go](../checker/redis.go) `CheckRedisSentinel`。

| 字段                                      | 条件                              | Status  |
|-------------------------------------------|-----------------------------------|---------|
| `SentinelClusterStatus.Error`             | 非空                              | Warn    |
| `SentinelClusterStatus.MasterReachable`   | `false`                           | Warn    |
| `SentinelClusterStatus.MasterEnvMatch`    | `"warn"`                          | Warn    |
| `SentinelClusterStatus.MasterEnvMatch`    | `"ok"` / `"N/A"`                  | OK / 不产生 |
| `SentinelClusterStatus.Status`            | `"ok"`                            | OK      |
| `SentinelClusterStatus.Status`            | 其他                              | Warn    |
| `SentinelNodeStatus.Reachable`            | `false`                           | Warn    |

### 3.5 MongoDB

来源：[checker/mongo.go](../checker/mongo.go) `CheckMongo`。

| 字段                          | 条件                          | Status  |
|-------------------------------|-------------------------------|---------|
| `MongoCluster.Error`          | 非空（跳过成员检查）          | Warn    |
| `MongoMember.Health`          | `1`                           | OK      |
| `MongoMember.Health`          | `≠ 1`                         | Notice  |

### 3.6 RabbitMQ

来源：[checker/rabbitmq.go](../checker/rabbitmq.go) `CheckRabbitMQ`。

| 字段                                      | 条件                              | Status  |
|-------------------------------------------|-----------------------------------|---------|
| `RabbitMQStatus.Error`                    | 非空                              | Warn    |
| `RabbitMQStatus.QueuesError`              | 非空（queues API 失败）           | Warn    |
| `RabbitMQStatus.ClusterPartition`         | `true`                            | Warn    |
| `RabbitMQStatus.AbnormalConnections`      | `> 0`                             | Warn    |
| `RabbitMQAlarm.MemAlarm`                  | `true`                            | Warn    |
| `RabbitMQAlarm.DiskFreeAlarm`             | `true`                            | Warn    |
| `RabbitMQStatus.ExceedingQueues[]`        | 非空切片每条                       | Warn    |
| `RabbitMQStatus.NoConsumerQueues[]`       | 非空切片每条                       | Warn    |

> ExceedingQueues / NoConsumerQueues 的"是否进切片"由 collector 完成（含
> `RabbitMQNoConsumerVHostBlacklist` 黑名单与 `RabbitMQQueueBacklog` 阈值）。
> Checker 只把切片元素逐条转 CheckResult，不重新判定。

### 3.7 bkmonitorv3 依赖联通性

来源：[checker/bkdeps.go](../checker/bkdeps.go) `CheckBKDeps`。

| 字段                          | 条件                          | Status     |
|-------------------------------|-------------------------------|------------|
| `DependencyResult.Status`     | `"ok"`                        | OK         |
| `DependencyResult.Status`     | `"skip"`                      | （不产生） |
| `DependencyResult.Status`     | 其他（`fail` / `unreachable`）| Notice     |

---

## 4. 邮件告警决策

### 4.1 抽取 Warn 项

[notify/alerts.go](../notify/alerts.go) `ExtractAlerts(report)` 只读
`report.AllChecks` 中 `Status == StatusWarn` 的条目，**不读** Unknown / Notice / OK。

每条产生一个 `AlertItem{Host, Field, Value}`：
- `Host`：从 `report.Hosts[].Checks` 反查 Field 对应的 IP；查不到留空（如
  cluster-level 的 RabbitMQ.error）。
- `Field`：CheckResult.Field（命名约定见 §5）。
- `Value`：CheckResult.Value。

### 4.2 签名（去重 key）

[notify/signature.go](../notify/signature.go) `Signature(items)`：
- 输入：所有 Warn `AlertItem`。
- 算法：对 `host + "|" + field` 排序后 SHA-256。
- **不**包含 Value：CPU 76% → 78% 视为同一告警（同 host 同 field），不变签名。
- 空集签名为 `""`，与"有告警"场景可区分。
- Unknown / Notice 不参与签名。

### 4.3 决策状态机

[notify/trigger.go](../notify/trigger.go) `Decide(now, prev, warnCount, sig, cooldown)`：

```
                              ┌──────────────────────────┐
                              │   本次 Warn count == 0   │
                              ├──────────────┬───────────┤
                              │              │           │
                  prev.Status │  ok / 空     │  alert    │
                              ├──────────────┼───────────┤
              动作            │  None        │ SendRecovery │
                              └──────────────┴───────────┘

                              ┌──────────────────────────┐
                              │   本次 Warn count > 0    │
                              ├──────────────┬───────────┤
                              │              │           │
                              │ prev.Status  │  动作     │
                              │ 空 / ok      │  SendAlert │
                              ├──────────────┼───────────┤
                              │ alert        │  ↓        │
                              │   sig != prev.sig (告警集合变化) │ SendAlert (立即) │
                              │   sig == prev.sig            │
                              │     now - prev.sentAt ≥ 2h   │ SendAlert (重发) │
                              │     now - prev.sentAt < 2h   │ None    (抑制)   │
                              └──────────────┴───────────┘
```

冷却窗口默认 2 小时（`trigger.min_interval_minutes`）。

### 4.4 邮件正文

[notify/email.go](../notify/email.go) `BuildAlertBody`：

```
[WeOps 巡检告警] 2026-05-08 21:44:07
Summary: 共 152 项检查，142 正常，9 告警，1 未知

告警明细:
  10.10.26.237   load_average                              = 42.39/40.57/40.29 (cores: 16)
                 rabbitmq.prod_bk_monitorv3.celery.backlog = 317496 msgs / 0 consumers
                 rabbitmq.bk_bkmonitorv3.celery_cron.no_consumer = 69 msgs
                 ...

详见附件 weops_inspection.html。
```

- Summary 行包含 Unknown 计数。
- 明细**仅展示 Warn 项**；Unknown / Notice 不进邮件正文（HTML 报告里仍可见）。
- HTML 报告作为附件 + 邮件 alternative body 一同发送。

### 4.5 持久化

[notify/state.go](../notify/state.go) `~/.config/weops/state.json`：

仅在 SMTP **发送成功**后写入 `last_sent_at / last_signature / last_status`。
失败保留旧基线，下次按旧状态判定，避免一次抖动让告警长期被误抑制。

---

## 5. CheckResult.Field 命名约定

签名稳定性依赖 Field 命名。新增 metric 时务必避免与历史 field 冲突。

```
es.{instance}.cluster_error                    # ES 集群错误
es.{instance}.cluster_status                   # ES 集群 yellow/red
es.{instance}.unassigned_shards                # ES 未分配分片
es.{instance}.pending_tasks                    # ES 待处理任务
es.{instance}.{ip}.heap                        # ES 节点 heap
es.{instance}.{ip}.ram                         # ES 节点 ram
es.{instance}.{ip}.reachability                # ES 节点 9200 可达性

redis.{ip}.error                               # Redis 节点错误
redis.{ip}.celery_queue                        # Redis celery 队列长度
redis.{ip}.monitor_queue                       # Redis monitor 队列长度

redis_sentinel.error
redis_sentinel.master_reachable
redis_sentinel.master_env_match
redis_sentinel.status
redis_sentinel.{ip}.reachable                  # sentinel 节点可达性

mongo.{instance}.error
mongo.{instance}.{member_name}.health

rabbitmq.error
rabbitmq.queues_error
rabbitmq.cluster_partition
rabbitmq.abnormal_connections
rabbitmq.node.{node}.mem_alarm
rabbitmq.node.{node}.disk_free_alarm
rabbitmq.{vhost}.{queue}.backlog               # ExceedingQueues 每条
rabbitmq.{vhost}.{queue}.no_consumer           # NoConsumerQueues 每条

bkdeps.{item}.status                           # mysql / redis / es7 / ...

service.{module}/{submodule}.status            # systemctl 状态
service.{module}/{submodule}.healthz           # healthz API
service.{module}.collect_error                 # service 段采集失败
service.{module}.docker.exited                 # Docker 退出容器数

# 主机段沿用旧 Field（无前缀），由 host_metrics-collection 规范定义：
cpu_usage / mem_usage / disk_usage(<mount>) / inode_usage(<mount>) /
max_open_files / selinux / firewalld / chronyd / load_average

# 复制段沿用旧 Field：
mysql_master(<ip>).read_only / mysql_slave(<ip>).replication /
redis(<ip>).role / redis(<ip>).link
```

---

## 6. 设计取舍速记

- **为什么模板用 `statusClass()` 而不是直接条件渲染？** 三个出口（HTML / Summary /
  邮件）共享同一 Status 字段是单一事实源。模板里如果写 `{{if gt .X N}}` 会让
  阈值散落到第二处（与 `Thresholds` 配置 + checker 形成三处真相）。
- **为什么 RabbitMQ Notice 全是 Warn 不是 Notice？** RabbitMQ 的所有告警项
  都是有"业务影响"的（积压、节点告警、连接异常），值班需要响应。后续若发现误报
  率高，可单独把 NoConsumerQueues 降为 Notice。
- **为什么 Mongo `Health != 1` 是 Notice 而非 Warn？** Health 字段在副本集恢复
  期间会短暂变 0，频繁告警噪音大。如果你的环境对此敏感，可以在 checker/mongo.go
  里改成 Warn。
- **为什么 bkdeps `fail` 是 Notice？** 现实中 bkdeps mysql 探测经常因为
  `mysql --execute` 的 stderr noise 误报 fail，可信度不足以告警。本质问题是
  collector 的 fail 判定不严格，应在 collector 侧修，而不是放大到邮件。
