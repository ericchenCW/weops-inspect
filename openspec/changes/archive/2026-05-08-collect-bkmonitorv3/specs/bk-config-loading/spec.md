## ADDED Requirements

### Requirement: bkmonitorv3 模块 IP 加载

系统 SHALL 从 `BK_MONITORV3_IP_COMMA` 解析为 `Config.MonitorV3IPs []string`, 未设置时为空切片; 该字段同时纳入 `Config.AllHosts` 去重集合, 用于 SSH 主机探活与进程巡检。

#### Scenario: 多节点 bkmonitorv3

- **WHEN** `BK_MONITORV3_IP_COMMA=10.97.20.18,10.97.20.19`
- **THEN** `Config.MonitorV3IPs = ["10.97.20.18","10.97.20.19"]` 且这两台 IP 出现在 `Config.AllHosts`

#### Scenario: 未部署 bkmonitorv3

- **WHEN** `BK_MONITORV3_IP_COMMA` 未设置
- **THEN** `Config.MonitorV3IPs` 为空切片, 不影响其它模块加载

### Requirement: bkmonitorv3 依赖凭据加载

系统 SHALL 在 `Config` 中新增以下字段并从对应环境变量读取, 用于 bkmonitorv3 依赖连通性采集; 任一变量未设置时对应字段为空字符串, 由采集器在运行时按 "missing config → skip" 处理:

- `MonitorRedisHost / MonitorRedisPort / MonitorRedisPassword` ← `BK_MONITOR_REDIS_HOST / _PORT / _PASSWORD`
- `MonitorMySQLHost / MonitorMySQLPort / MonitorMySQLUser / MonitorMySQLPassword` ← `BK_MONITOR_MYSQL_*`
- `PaasMySQLHost / PaasMySQLPort / PaasMySQLUser / PaasMySQLPassword` ← `BK_PAAS_MYSQL_*`
- `MonitorRabbitMQHost / MonitorRabbitMQPort / MonitorRabbitMQUser / MonitorRabbitMQPassword / MonitorRabbitMQVHost` ← `BK_MONITOR_RABBITMQ_*`
- `GseZKHost / GseZKPort` ← `BK_GSE_ZK_HOST / _PORT`
- `MonitorES7Host / MonitorES7RestPort / MonitorES7User / MonitorES7Password` ← `BK_MONITOR_ES7_*`
- `MonitorInfluxDBHost / MonitorInfluxDBPort` ← `BK_INFLUXDB_IP0` 与 `BK_MONITOR_INFLUXDB_PORT`

#### Scenario: 凭据齐全

- **WHEN** 所有 `BK_MONITOR_*` / `BK_PAAS_MYSQL_*` / `BK_GSE_ZK_*` / `BK_INFLUXDB_IP0` 在 env 中设置
- **THEN** `Config` 对应字段非空且与 env 值一致

#### Scenario: 部分凭据缺失

- **WHEN** 仅设置 `BK_MONITOR_REDIS_HOST` 而 `BK_MONITOR_REDIS_PASSWORD` 缺失
- **THEN** `Config.MonitorRedisHost` 非空, `Config.MonitorRedisPassword` 为 `""`, 加载阶段不报错
