## ADDED Requirements

### Requirement: 检查状态四元枚举

系统 SHALL 使用 `OK / Warn / Unknown / Notice` 四种检查状态：

- `OK`：已采集到数据，且各阈值/期望值均满足，进 Summary OK 桶。
- `Warn`：明确的告警项，进 Summary Warn 桶，进告警邮件。
- `Unknown`：本应有数据但采集不到（如远端未注册的子服务），进 Summary Unknown 桶，**不**进告警邮件。
- `Notice`：着色提示项，**不**进 Summary 任何桶，**不**进告警邮件，仅供 HTML 报告渲染时着色。

#### Scenario: Warn 与 Notice 的区分
- **WHEN** ES 集群 `cluster.Error` 非空
- **THEN** 该项 Status SHALL 为 `Warn`，进入 Summary.Warn 与告警邮件
- **WHEN** ES 节点 `RAMPercent` 大于阈值
- **THEN** 该项 Status SHALL 为 `Notice`，HTML 单元格着红色，但 Summary.Warn 不增加，邮件不收录

#### Scenario: Unknown 的语义
- **WHEN** Service 模块的 `Status` 字段为空字符串（远端未注册该子服务）
- **THEN** 该项 Status SHALL 为 `Unknown`，Summary.Unknown 增加，但邮件不收录

### Requirement: ES 集群检查

系统 SHALL 对每个采集到的 ES 集群产生以下 CheckResult：

- `cluster.Error` 非空 → `Warn`
- 节点 `Status == "unreachable"` → `Warn`
- `UnassignedShards > 0` → `Notice`
- 节点 `HeapPercent > Thresholds.ESHeapPercent` → `Notice`
- 节点 `RAMPercent > Thresholds.ESRAMPercent` → `Notice`
- `cluster.status != "green"`（cluster.Error 为空时）→ `Notice`
- `pending_tasks > 0` → `Notice`

#### Scenario: 集群采集失败
- **WHEN** ES 集群 `Error` 字段为 `"all nodes unreachable"`
- **THEN** Checker SHALL 产生一条 Warn CheckResult，Field 形如 `es.{endpoint}.cluster_error`
- **AND** 该集群下属节点的 reachability 检查 SHALL 跳过（避免重复告警）

#### Scenario: 节点不可达
- **WHEN** 节点列表中某个 IP 的 `Status == "unreachable"`
- **THEN** 该节点 SHALL 产生一条 Warn CheckResult，Field 形如 `es.{ip}.reachability`

#### Scenario: 节点 RAM 高于阈值
- **WHEN** 节点 `RAMPercent == 96` 且 `Thresholds.ESRAMPercent == 95`
- **THEN** 该节点 SHALL 产生一条 Notice CheckResult，Value 包含 `96%`
- **AND** Summary.Warn MUST 不增加，Summary.Total MUST 不增加

### Requirement: Redis 单点检查

系统 SHALL 对 `RedisStandalone` 切片产生以下 CheckResult：

- 整段 `Error` 非空 → `Warn`（顶层）
- 节点 `Error` 非空 → `Warn`
- 节点 `CeleryQueue > Thresholds.RedisCeleryQueue` → `Notice`
- 节点 `MonitorQueue > Thresholds.RedisMonitorQueue` → `Notice`

#### Scenario: 节点采集错误
- **WHEN** 节点 `10.10.26.235.Error == "redis info error: ..."`
- **THEN** Checker SHALL 产生一条 Warn CheckResult，Host 为该 IP

#### Scenario: Celery 积压超阈值
- **WHEN** 节点 `CeleryQueue == 5000`，`Thresholds.RedisCeleryQueue == 1000`
- **THEN** 该节点 SHALL 产生一条 Notice CheckResult

### Requirement: Redis Sentinel 检查

系统 SHALL 对 `RedisSentinel` 产生以下 CheckResult，全部为 `Warn`：

- `Sentinel.Error` 非空
- `MasterReachable == false`
- `MasterEnvMatch == "warn"`
- `Sentinel.Status != "ok"`
- 任一 sentinel 节点 `Reachable == false`

#### Scenario: master 不可达
- **WHEN** `RedisSentinel.MasterReachable == false`
- **THEN** Checker SHALL 产生一条 Warn CheckResult，Field 为 `redis_sentinel.master_reachable`

#### Scenario: sentinel 节点不可达
- **WHEN** sentinel 节点 `10.10.26.235.Reachable == false`
- **THEN** Checker SHALL 产生一条 Warn CheckResult，Host 为该 IP

### Requirement: MongoDB 检查

系统 SHALL 对 `MongoDB` 产生以下 CheckResult：

- `MongoDB.Error` 非空 → `Warn`
- 成员 `Health != 1` → `Notice`

#### Scenario: 副本集采集失败
- **WHEN** `MongoDB.Error == "mongo connect error: ..."`
- **THEN** Checker SHALL 产生一条 Warn CheckResult

#### Scenario: 成员不健康
- **WHEN** 成员 `Name == "10.10.26.236:27017"`，`Health == 0`
- **THEN** 该成员 SHALL 产生一条 Notice CheckResult，且 Summary.Warn 不增加

### Requirement: RabbitMQ 检查

系统 SHALL 对 `RabbitMQ` 产生以下 CheckResult，全部为 `Warn`：

- `RabbitMQ.Error` 非空
- `ClusterPartition == true`
- `AbnormalConnections > 0`
- `QueuesError` 非空
- 节点 `MemAlarm == true`
- 节点 `DiskFreeAlarm == true`
- `ExceedingQueues` 中每个队列各一条
- `NoConsumerQueues` 中每个队列各一条

`ExceedingQueues / NoConsumerQueues` 的筛选与 vhost 黑名单仍由 collector 完成，
checker 直接将切片转为 CheckResult，不重新判定。

#### Scenario: 队列积压逐项展开
- **WHEN** `ExceedingQueues` 包含 `{vhost: prod_bk_monitorv3, queue: celery, MessageCount: 360547}`
- **THEN** Checker SHALL 产生一条 Warn CheckResult，Field 形如 `rabbitmq.{vhost}.{queue}.backlog`，Value 含消息数

#### Scenario: 节点内存告警
- **WHEN** 节点告警列表中某节点 `MemAlarm == true`
- **THEN** 该节点 SHALL 产生一条 Warn CheckResult

### Requirement: bkmonitorv3 依赖检查

系统 SHALL 对 `BKMonitorV3.Dependencies` 中每个 `DependencyResult` 产生 CheckResult：

- `Status == "ok"` → `OK`
- `Status == "skip"` → 不产生 CheckResult
- 其他 → `Notice`

#### Scenario: mysql 依赖失败
- **WHEN** 依赖 `mysql` 的 `Status == "fail"`
- **THEN** Checker SHALL 产生一条 Notice CheckResult，Summary.Warn MUST 不增加

### Requirement: Service 段补检与空状态处理

系统 SHALL 修改 `CheckService` 行为：

- `Status == ""` → `Unknown`（原行为为跳过，**BREAKING**）
- `Status == "active"` → `OK`
- 其他 → `Warn`

并新增 Notice 类检查项：

- service 段顶层采集 `Error` 非空 → `Notice`
- Docker `ContainersExited > Thresholds.ServiceContainersExited` → `Notice`

#### Scenario: 空 Status 归为 Unknown
- **WHEN** 子服务 `job-analysis.Status == ""`
- **THEN** Checker SHALL 产生一条 Unknown CheckResult，Summary.Unknown 增加，告警邮件不收录该项

#### Scenario: Docker 退出容器为 Notice
- **WHEN** Docker `ContainersExited == 3`，`Thresholds.ServiceContainersExited == 0`
- **THEN** SHALL 产生一条 Notice CheckResult，Summary.Warn MUST 不增加

### Requirement: HTML 渲染从 Status 字段着色

系统 SHALL 在以下渲染结构上携带 `Status` 字段（取值 `OK/Warn/Unknown/Notice`），
HTML 模板的所有 `td` 着色 SHALL 仅依据 `Status` 字段决定：

- `model.HostMetrics`（已有 CheckResult.Status，模板沿用）
- `model.ServiceModule`、`model.DockerSummary`
- `model.ESCluster`、`model.ESNode`、`model.ESNodeReach`
- `model.RedisStandaloneNode`、`model.RedisSentinelStatus`、`model.SentinelNode`
- `model.MongoMember`
- `model.RabbitMQQueue`、`model.RabbitMQAlarm`
- `model.DependencyResult`

模板中 SHALL 不再出现 `{{if gt ... <数字>}}` 或 `{{if eq .Status "active"}}` 之类
内联阈值/常量比较；现有判断改为 `{{if eq .Status "warn"}}status-warn{{...}}` 形式。

#### Scenario: 模板着色与 CheckResult 一致
- **WHEN** 同一份 InspectReport 输入 checker 与渲染流水线
- **THEN** HTML 表格中每个被标 `status-warn` 的单元格 MUST 在 `allChecks` 中存在
  对应的 `Status == Warn` 或 `Status == Notice` 的 CheckResult
- **AND** 反向不要求成立（部分主机指标 CheckResult 不必显示在表格中）

#### Scenario: 模板不再有内联数字比较
- **WHEN** 在 `render/templates/*.tmpl` 中 grep `{{if gt .` 或 `{{if eq .Status "active"}}`
- **THEN** 命中数 SHALL 为 0

### Requirement: Summary 桶扩展

系统 SHALL 在 `model.CheckSummary` 增加 `Unknown int` 字段，并修改 `Summarize`：

- `Total` 包含 OK + Warn + Unknown，**不**包含 Notice
- `OK` 仅包含 Status == OK
- `Warn` 仅包含 Status == Warn
- `Unknown` 仅包含 Status == Unknown
- Notice 项被丢弃，不参与计数

#### Scenario: Notice 不影响计数
- **WHEN** allChecks 含 5 OK、1 Warn、1 Unknown、3 Notice
- **THEN** Summary.Total == 7，Summary.OK == 5，Summary.Warn == 1，Summary.Unknown == 1

#### Scenario: 控制台输出包含 Unknown
- **WHEN** 巡检结束打印 Summary 行
- **THEN** stderr SHALL 输出 `共 N 项检查, X 正常, Y 告警, Z 未知`
