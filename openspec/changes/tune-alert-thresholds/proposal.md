## Why

当前巡检告警的默认阈值偏严, 在生产环境中产生大量低价值告警(CPU/内存/磁盘 75% 即报); 同时存在两类配置缺口与一处误报:

- 部分服务(如 `job-analysis`)在当前部署形态下不应纳入 status / healthz 检查, 但 `service_registry` 没有跳过机制
- RabbitMQ 中 `bk_bknodeman` vhost 的某些队列长期 0 消费者属于预期行为, 但被无条件告警
- `job-gateway` 的 actuator 健康检查端点跑在独立的 management 端口(19876), 而 `service_registry` 写死走业务端口(10503), 业务端口启用 TLS 后 healthz 永远命中 `Bad Request - This combination of host and port requires TLS` 被误判为 fail

需要一次性收口阈值与误报问题。

## What Changes

- **BREAKING**: CPU / 内存 / 磁盘 / Inode 默认阈值从 `75%` 调整为 `95%`
- **BREAKING**: 最大打开文件数默认阈值从 `102400` 调整为 `65536`(语义: `< 65536` 时告警, 即必须 ≥ 65536)
- **BREAKING**: 移除主机运行天数(`RunDays`)告警检查与对应的 `INSPECT_RUN_DAYS` 配置项
- 新增 `ServiceSpec.HealthzPort` 字段, 默认 `0` 表示沿用 `Port`; 非零时 healthz 探测使用该端口
- 新增 `ServiceSpec.SkipStatusCheck` / `SkipHealthzCheck` 布尔字段, 用于跳过对应检查
- `service_registry` 中 `job-gateway` 配置 `HealthzPort: 19876`(其它字段不变)
- `service_registry` 中 `job-analysis` 配置 `SkipStatusCheck: true` 与 `SkipHealthzCheck: true`
- 新增 `INSPECT_RABBITMQ_NO_CONSUMER_VHOST_BLACKLIST` 配置项(逗号分隔), 默认包含 `bk_bknodeman`; 命中黑名单的 vhost 不产生 "0 消费者" 告警, 但队列堆积阈值告警仍照常生效

## Capabilities

### New Capabilities

- `service-check-overrides`: 在 ServiceSpec 层面声明 healthz 端口覆盖与按服务跳过 status/healthz 检查的能力

### Modified Capabilities

- `threshold-config`: 调整 CPU / 内存 / 磁盘 / Inode / MaxOpenFiles 默认值; 移除 RunDays 阈值与 env; 新增 `INSPECT_RABBITMQ_NO_CONSUMER_VHOST_BLACKLIST` env 与默认值

## Impact

- **代码**:
  - `config/config.go`: 调整 5 项默认值; 删除 `RunDays` 字段与 `INSPECT_RUN_DAYS` 解析; 新增 RabbitMQ vhost 黑名单字段与解析
  - `checker/rules.go`: 删除 RunDays 判定块
  - `model/`: `Thresholds` 结构体删除 `RunDays`; 视情况保留 `HostMetrics.RunDays` 采集字段(不输出告警即可)
  - `collector/service_registry.go`: `ServiceSpec` 新增 3 个字段; `job-gateway` 配置 `HealthzPort`; `job-analysis` 配置两个 skip
  - `collector/service.go`: 拼 healthz URL 时使用 `HealthzPort` 兜底; 在采集与判定阶段尊重 skip 标志
  - RabbitMQ checker 层: 在 0 消费者判定时按 vhost 黑名单过滤
- **配置变更**:
  - 删除: `INSPECT_RUN_DAYS`
  - 新增: `INSPECT_RABBITMQ_NO_CONSUMER_VHOST_BLACKLIST`
- **测试**:
  - `config/config_test.go` 需更新默认值断言, 添加新 env 解析用例
  - 视情况补 service skip / healthz port 的单元测试
- **运维**: 默认阈值变化对已部署用户有可观测影响, 需要在 release notes 中说明; 仍依赖原 75% 阈值的用户需通过 env 显式设置
