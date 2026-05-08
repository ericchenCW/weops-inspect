## ADDED Requirements

### Requirement: bkmonitorv3 依赖 MySQL 单端点配置

`BKMonitorV3DepConfig` SHALL 仅保存单组 MySQL 字段 `MySQLHost / MySQLPort / MySQLUser / MySQLPassword`,加载顺序:

- `MySQLHost`:取 `BK_MONITOR_MYSQL_HOST`,空则取 `BK_PAAS_MYSQL_HOST`,均空则默认 `mysql.service.consul`
- `MySQLPort`:取 `BK_MONITOR_MYSQL_PORT`,空则取 `BK_PAAS_MYSQL_PORT`,均空则默认 `3306`
- `MySQLUser`:取 `BK_MONITOR_MYSQL_USER`,空则取 `BK_PAAS_MYSQL_USER`
- `MySQLPassword`:取 `BK_MONITOR_MYSQL_PASSWORD`,空则取 `BK_PAAS_MYSQL_PASSWORD`

`BKMonitorV3DepConfig` SHALL NOT 同时保存 `PaaSMySQL*` 与 `MonitorMySQL*` 两组字段。

#### Scenario: 默认端点

- **WHEN** 未设置任何 `BK_*_MYSQL_HOST` 与 `BK_*_MYSQL_PORT`
- **THEN** `BKMonitorV3.MySQLHost == "mysql.service.consul"` 且 `BKMonitorV3.MySQLPort == "3306"`

#### Scenario: env 覆盖

- **WHEN** `BK_MONITOR_MYSQL_HOST=10.10.26.235`
- **THEN** `BKMonitorV3.MySQLHost == "10.10.26.235"`,默认 host 不再生效

#### Scenario: 账号回退

- **WHEN** `BK_MONITOR_MYSQL_USER` 未设置,`BK_PAAS_MYSQL_USER=root`
- **THEN** `BKMonitorV3.MySQLUser == "root"`
