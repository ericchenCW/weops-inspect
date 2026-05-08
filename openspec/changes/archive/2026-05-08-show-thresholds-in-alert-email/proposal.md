## Why

当前告警邮件正文（`notify/email.go::BuildAlertBody`）每行只展示 `host / field / value`，
收件人无法直接判断："这是刚过线还是严重越线？阈值是不是设得过松/过紧？"
要回答这个问题，收件人得跳到 HTML 附件、再回查 `config/config.go` 的默认阈值或环境变量
覆盖值，操作链很长。

把触发阈值（或期望值）作为一列直接展示在邮件正文，能让收件人在一眼内完成"严重程度
判断"和"是否误报"的初筛。

## What Changes

- `model.CheckResult` SHALL 新增可选字段 `Threshold string`，承载触发该 Status 的阈值
  或期望值的人类可读描述（如 `≥ 95%`、`< 65536`、`期望 active`）
- `checker/rules.go` SHALL 在产生 `Warn` / `Notice` 类 `CheckResult` 时填充 `Threshold`，
  覆盖范围：
  - 数值类: `cpu_usage`, `mem_usage`, `disk_usage`, `inode_usage`, `max_open_files`,
    `mysql_slave.replication`（lag）, `redis.link`（io seconds）, RabbitMQ 队列 backlog,
    Docker `service.*.docker.exited`
  - 状态期望类: `selinux`, `firewalld`, `chronyd`, `service.*.status`, `service.*.healthz`
- 关系/复杂规则类（`load_average`、`mysql_master.read_only`、`redis.role`、RabbitMQ no_consumer
  黑名单等）SHALL 留 `Threshold = ""`；规则描述维持现有 `Value` 表达
- `notify.AlertItem` SHALL 新增 `Threshold string` 字段，由 `ExtractAlerts` 透传
- `notify.BuildAlertBody` SHALL 在告警明细行追加阈值列（仅当 `Threshold != ""` 时显示
  `(阈值 X)` 后缀），列宽与现有 host/field/value 对齐
- 告警签名（`notify/signature.go`）MUST NOT 把 `Threshold` 纳入签名计算
- HTML 附件渲染（`render` 包模板）SHALL 在本次 change 中保持不变；HTML 内嵌正文同样
  不受影响（继续来自 `weops_inspection.html` 的 `<body>` 提取，已自带阈值由后续 change 处理）

## Capabilities

### New Capabilities
<!-- 无新能力 -->

### Modified Capabilities
- `alert-notification`: "告警邮件结构" Scenario 的 host/field/value 需要扩展为
  host/field/value/threshold；并新增"阈值不影响告警签名"的约束 Scenario

## Impact

- 代码:
  - `model/types.go`（或 `CheckResult` 所在文件）: 新增 `Threshold` 字段
  - `checker/rules.go`: 把 `add(field, value, status)` 局部 helper 扩展为可选 threshold 参数
    或新增第二个 helper；逐处补阈值字符串
  - `notify/alerts.go`: `AlertItem` 加字段；`ExtractAlerts` 拷贝
  - `notify/email.go`: `BuildAlertBody` 渲染调整
  - `notify/signature.go`: 显式注释"signature 仅基于 host+field"，无需逻辑改动（当前已不读 Threshold）
- 依赖: 无新增
- 兼容性: `CheckResult` 新增字段为 string 默认值 `""`；JSON 序列化向后兼容；下游消费者
  （HTML 模板、JSON 输出）若不读 `Threshold` 字段则零影响
- 与 `tune-alert-thresholds` change 的关系: 那个 change 调整阈值"数值"的代码任务已
  完成（剩端到端验证与发布文档），代码侧默认阈值已是 95% / 65536 等新值；本 change
  调整阈值"展示"，可独立推进，不会与之产生代码冲突
