## ADDED Requirements

### Requirement: bkmonitorv3 子角色 IP 独立加载

系统 SHALL 从以下 4 个环境变量分别加载 bkmonitorv3 各子角色的部署主机列表,字段名与 `Config` 字段对应:

- `BK_MONITORV3_MONITOR_IP_COMMA` → `Config.MonitorV3MonitorIPs`
- `BK_MONITORV3_INFLUXDB_PROXY_IP_COMMA` → `Config.MonitorV3InfluxDBProxyIPs`
- `BK_MONITORV3_TRANSFER_IP_COMMA` → `Config.MonitorV3TransferIPs`
- `BK_MONITORV3_UNIFY_QUERY_IP_COMMA` → `Config.MonitorV3UnifyQueryIPs`

每个角色字段为空时,SHALL 回退使用 `Config.MonitorV3IPs`(来自 `BK_MONITORV3_IP_COMMA`)以兼容旧配置。

#### Scenario: 角色 IP 单独配置

- **WHEN** `BK_MONITORV3_TRANSFER_IP_COMMA=10.10.26.236`
- **THEN** `Config.MonitorV3TransferIPs == ["10.10.26.236"]`

#### Scenario: 回退到旧变量

- **WHEN** `BK_MONITORV3_TRANSFER_IP_COMMA` 未设置且 `BK_MONITORV3_IP_COMMA=10.97.20.18`
- **THEN** `GetModuleHosts()` 中 `bkmonitorv3-transfer` 条目的 IP 列表为 `["10.97.20.18"]`

#### Scenario: 全部空

- **WHEN** `BK_MONITORV3_TRANSFER_IP_COMMA` 与 `BK_MONITORV3_IP_COMMA` 都未设置
- **THEN** `GetModuleHosts()` 中 `bkmonitorv3-transfer` 条目 IP 列表为空,采集流程跳过该角色

### Requirement: AllHosts 包含 bkmonitorv3 角色 IP

`Config.buildAllHosts()` SHALL 把 4 个角色 IP 列表与 `Config.MonitorV3IPs` 一并纳入去重,使主机级 OS 指标采集覆盖每台部署 bkmonitorv3 任一角色的主机。

#### Scenario: 角色 IP 出现在 AllHosts

- **WHEN** `BK_MONITORV3_MONITOR_IP_COMMA=10.10.26.235` 且 `BK_MONITORV3_TRANSFER_IP_COMMA=10.10.26.236`
- **THEN** `Config.AllHosts` 包含 `10.10.26.235` 与 `10.10.26.236`(若未被其它模块重复出现则各一次)
