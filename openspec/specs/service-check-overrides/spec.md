# service-check-overrides Specification

## Purpose
TBD - created by archiving change tune-alert-thresholds. Update Purpose after archive.
## Requirements
### Requirement: ServiceSpec 支持独立 healthz 端口

系统 SHALL 在 `collector.ServiceSpec` 中支持 `HealthzPort int` 字段。当该字段为非零值时, healthz 探测使用该端口而不是 `Port`; 为 `0`(默认/未设置)时沿用 `Port`。该字段不影响 systemctl 状态采集和进程数采集, 仅影响 healthz HTTP 探测的目标端口。

#### Scenario: 未设置 HealthzPort

- **WHEN** `ServiceSpec{Port: 8000, HealthzPath: "/healthz"}` 且未设 `HealthzPort`
- **THEN** healthz 探测访问 `http://127.0.0.1:8000/healthz`

#### Scenario: 设置了 HealthzPort

- **WHEN** `ServiceSpec{Port: 10503, HealthzPort: 19876, HealthzPath: "/actuator/health"}`
- **THEN** healthz 探测访问 `http://127.0.0.1:19876/actuator/health`(systemctl 状态依然按 ServiceUnit 检查)

### Requirement: job-gateway healthz 走 management 端口

系统 SHALL 在 `service_registry` 中将 `job-gateway` 的 `HealthzPort` 配置为 `19876`, 其它字段(`ServiceUnit`, `ProcessName`, `Port=10503`, `HealthzPath`, `HealthzType=json_up`)保持不变。

#### Scenario: job-gateway 业务端口启用 TLS 时不再误报

- **WHEN** 10.10.26.237 上 job-gateway 监听 10503(TLS) 与 19876(http management)
- **THEN** healthz 通过 19876 端口拿到 `{"status":"UP"...}` 并判定为 `ok`(不再因 TLS 报错被误判 fail)

### Requirement: ServiceSpec 支持跳过 status / healthz 检查

系统 SHALL 在 `collector.ServiceSpec` 中支持 `SkipStatusCheck bool` 与 `SkipHealthzCheck bool` 字段。

- 当 `SkipStatusCheck=true`: collector 不输出该服务的 systemctl 状态采集, checker 不产生该服务的 status 检查项
- 当 `SkipHealthzCheck=true`: collector 不发起 healthz HTTP 请求, checker 不产生该服务的 healthz 检查项
- 两者独立, 可单独启用或同时启用

#### Scenario: SkipStatusCheck=true

- **WHEN** `ServiceSpec{Name: "foo", SkipStatusCheck: true}`
- **THEN** 报告中 foo 服务不出现 status 字段相关检查结果

#### Scenario: SkipHealthzCheck=true

- **WHEN** `ServiceSpec{Name: "foo", SkipHealthzCheck: true, HealthzPath: "/healthz"}`
- **THEN** 报告中 foo 服务不出现 healthz_api 字段相关检查结果, 且不发起 HTTP 请求

#### Scenario: 两者皆 false(默认)

- **WHEN** `ServiceSpec{Name: "foo"}` 未设 skip 字段
- **THEN** status 与 healthz 检查行为与本 change 之前一致

### Requirement: job-analysis 跳过 status 与 healthz 检查

系统 SHALL 在 `service_registry` 中将 `job-analysis` 配置为 `SkipStatusCheck: true` 与 `SkipHealthzCheck: true`。

#### Scenario: job-analysis 不出现在 status 与 healthz 检查项中

- **WHEN** 巡检对包含 job-analysis 的主机执行
- **THEN** 报告中 job-analysis 服务不产生 `status` 与 `healthz_api` 检查项(其它 job-* 服务正常输出)

