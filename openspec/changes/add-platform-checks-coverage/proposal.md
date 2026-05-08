## Why

当前巡检的"红字 = 告警计数 = 邮件告警"三处判定彼此独立，覆盖范围不一致。
具体两类病：

1. `checker.CheckService` 跳过空 `Status`，但 `notify.ExtractAlerts` 把空状态判为告警，
   导致控制台输出 `0 告警` 同时仍发送告警邮件（如本周 `job-analysis` 案例）。
2. ES / Redis / MongoDB / RabbitMQ / bkmonitorv3 依赖联通性等组件**完全没有 Checker**，
   异常仅以模板内联 `{{if gt ...}}` 形式着色，既不进 `Summary.Warn` 也不进邮件
   （如本周 RabbitMQ `prod_bk_monitorv3.celery` 积压 36 万条但 0 告警）。

补齐缺失 Checker 并把判定收敛为单一事实源（CheckResult.Status）。

## What Changes

- 引入 `Unknown` 状态（采集不到的子服务，如空 `service.Status`）与 `Notice` 状态
  （着色但不进 Summary 不发邮件），原有 `OK` / `Warn` 保留语义不变。
- `Summary` 增加 `Unknown` 字段；`Notice` 不计入 `Total/OK/Warn/Unknown` 任一桶。
- 新增 `CheckES` / `CheckRedis` / `CheckMongo` / `CheckRabbitMQ` / `CheckBKDeps`
  五个 Checker 函数，覆盖各组件的告警与 Notice 项。
- `CheckService`：空 `Status` → `Unknown`（**BREAKING**：原行为是跳过不产生 CheckResult）。
- 渲染层重构：在 `ESCluster / ESNode / RedisStandaloneNode / RedisSentinelStatus / Sentinel /
  MongoMember / RabbitMQQueue / DependencyResult / DockerSummary` 等结构上补 `Status` 字段，
  HTML 模板按 `Status` 字段统一着色，删除模板内所有内联 `{{if gt ...}}` / `{{if eq ...}}`
  着色判断（**BREAKING**：模板渲染逻辑变化，但用户视觉一致）。
- 阈值集中：把模板里硬编码的 ES heap/RAM、Redis Celery/Monitor 队列、ES 未分配分片、Docker
  退出容器数等阈值搬入 `Thresholds`，新增对应 env 变量。
- `notify.ExtractAlerts` 改为只读 `CheckResult.Status == Warn`，删除其对 service /
  replication 的二次判定逻辑。

本轮告警范围（→ Warn，进 Summary 进邮件）：

```
Service     S3   空 Status → Unknown（不告警）
ES          ES1  cluster.Error
            ES5  node unreachable
Redis 单点  RD1  采集 Error
            RD4  节点 Error
Sentinel    RS1–RS5 全部
Mongo       MG1  MongoDB.Error
MySQL       MY1  节点 Error（+ 已有 replication）
RabbitMQ    MQ1–MQ8 全部
```

本轮 Notice 范围（→ Status 着色但不进 Summary 不发邮件）：

```
Service     S1 service 段采集 Error    S2 Docker 退出 > 0
ES          ES2 UnassignedShards>0    ES3 Heap%   ES4 RAM%
            ES6 cluster.status≠green  ES7 pending_tasks>0
Redis 单点  RD2 CeleryQueue   RD3 MonitorQueue
Mongo       MG2 node Health≠1
BKDeps      BK1 Status≠ok/skip
```

## Capabilities

### New Capabilities

- `platform-checks`: 把对蓝鲸基础设施组件（ES / Redis / Mongo / RabbitMQ / bkmonitorv3 依赖）
  的状态判定、告警/Notice 分级、阈值消费三个职责，从 collector 与 HTML 模板中收敛到一个
  统一的 checker 层。

### Modified Capabilities

- `alert-notification`: 状态枚举从 `OK/Warn` 扩展为 `OK/Warn/Unknown/Notice`；Summary
  增加 `Unknown` 字段；`ExtractAlerts` 改为只读 `Status == Warn`；告警邮件正文需要支持
  Unknown 项的展示。
- `threshold-config`: 新增 6 项阈值与对应 env 变量（ES heap/RAM/未分配分片、Redis Celery/Monitor
  队列、Docker 退出容器数）。
- `infra-component-collection`: 渲染结构补 `Status` 字段；collector 不再自行判定阈值
  红字（保留已有的 RabbitMQ ExceedingQueues / NoConsumerQueues 等"已筛选切片"产出，
  其他阈值判定迁移到 checker）。

## Impact

- **代码**：
  - `model/types.go`：CheckStatus 增加常量；多个 Status 字段；CheckSummary 增加 Unknown
  - `checker/`：新增 ES/Redis/Mongo/RabbitMQ/BKDeps 5 个 Check 函数；`CheckService` 修改空状态行为；`Summarize` 改写
  - `notify/alerts.go`：删除二次判定，改为单一 CheckResult.Status==Warn 过滤
  - `notify/email.go`：邮件正文支持 Unknown 项
  - `render/templates/*.tmpl`：所有内联着色判断替换为对 `Status` 字段的判断
  - `config/config.go`：新增 6 项阈值字段与 env 解析
  - `main.go`：新 Check 函数接入 `allChecks` 流水线
- **配置**：新增 6 个 env 变量（详见 design.md）
- **行为变化**：
  - 邮件告警条目数量会显著上升（之前漏报项现在会进邮件）
  - state.json 中 last_signature 在升级当次必然变化，会触发一次"签名变化→立即发送"
- **测试**：
  - 现有 `notify/*_test.go`、`checker/rules_test.go` 需要更新以适配 Unknown 桶
  - 新增 5 个 Check 函数各自的单元测试
