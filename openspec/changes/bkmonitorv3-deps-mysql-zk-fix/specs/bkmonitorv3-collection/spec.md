## MODIFIED Requirements

### Requirement: bkmonitorv3 模块依赖连通性采集

系统 SHALL 新增模块依赖采集器, 在中控本机依次探测 bkmonitorv3 配置的依赖端点, 对齐 `bk-install/health_check/deploy_check.py:bkmonitorv3()` 的检查范围:

- Redis 单点登录 (`BK_MONITOR_REDIS_HOST` / `BK_MONITOR_REDIS_PORT` / `BK_MONITOR_REDIS_PASSWORD`)
- **MySQL 单条登录**:host 默认 `mysql.service.consul`,port 默认 `3306`,账号优先取 `BK_MONITOR_MYSQL_USER/PASSWORD`,空时回退到 `BK_PAAS_MYSQL_USER/PASSWORD`;允许 env 覆盖默认 host/port
- RabbitMQ AMQP 登录 (`BK_MONITOR_RABBITMQ_*`)
- ZooKeeper **TCP 连通性**探测(`BK_GSE_ZK_HOST:BK_GSE_ZK_PORT`),不发送 4lw 命令
- Elasticsearch 7 登录 (`BK_MONITOR_ES7_HOST:BK_MONITOR_ES7_REST_PORT` + `BK_MONITOR_ES7_USER/PASSWORD`)
- InfluxDB `/ping` (`BK_INFLUXDB_IP0:BK_MONITOR_INFLUXDB_PORT`)

每项产出 `{Item, Endpoint, Status, Detail}`, 其中 `Status ∈ {ok, fail, unreachable, skip}`。`Item` 取值集合为 `{redis, mysql, rabbitmq, zookeeper, es7, influxdb}`(共 6 项)。

#### Scenario: 全部依赖正常

- **WHEN** 所有依赖凭据正确且端点可达
- **THEN** 报告 `BKMonitorV3.Dependencies` 包含 6 条结果, 每条 `Status="ok"`

#### Scenario: 单项依赖失败

- **WHEN** `BK_MONITOR_RABBITMQ_PASSWORD` 错误, 其它依赖均正常
- **THEN** RabbitMQ 项 `Status="fail"`, `Detail` 记录鉴权失败信息, 其它依赖项不受影响

#### Scenario: MySQL 默认端点

- **WHEN** 未设置任何 `BK_*_MYSQL_HOST` / `BK_*_MYSQL_PORT`,但提供了 `BK_MONITOR_MYSQL_USER` 与 `BK_MONITOR_MYSQL_PASSWORD`
- **THEN** 巡检对 `mysql.service.consul:3306` 执行 `SELECT 1`,记 `Item="mysql"` 单条结果

#### Scenario: MySQL 账号回退

- **WHEN** 仅设置 `BK_PAAS_MYSQL_USER/PASSWORD`,未设 `BK_MONITOR_MYSQL_USER/PASSWORD`
- **THEN** 探测使用 `BK_PAAS_MYSQL_USER/PASSWORD` 连接 `mysql.service.consul:3306`

#### Scenario: ZK 仅做 TCP 连通

- **WHEN** ZK 已禁用 `ruok`(`4lw.commands.whitelist` 不含 `ruok`)但端口 2181 可建 TCP
- **THEN** ZK 项 `Status="ok"`, `Detail` 提示 "TCP-only probe; 4lw whitelist may exclude ruok"

#### Scenario: ZK 端口不可达

- **WHEN** `BK_GSE_ZK_HOST:BK_GSE_ZK_PORT` TCP 连接失败
- **THEN** ZK 项 `Status="unreachable"`, `Detail` 记录 dial 错误

#### Scenario: 依赖凭据缺失

- **WHEN** `BK_MONITOR_ES7_HOST` 未设置
- **THEN** ES7 项 `Status="skip"`, `Detail="missing config"`, 不阻塞其它探测

#### Scenario: bkmonitorv3 未部署时跳过依赖采集

- **WHEN** `BK_MONITORV3_IP_COMMA` 为空
- **THEN** 不执行依赖连通性采集, `BKMonitorV3.Dependencies` 不出现在报告
