## ADDED Requirements

### Requirement: AllHosts 包含 BK 模块及基础设施节点

系统 SHALL 将以下 IP 来源全部并入 `Config.AllHosts`(去重后):

- 9 个 BK 业务模块 IP(PaaS / CMDB / Job / GSE / APPO / APPT / IAM / UserMgr / NodeMan)
- ES7 节点(`BK_ES7_IP_COMMA`)
- RabbitMQ 节点(`BK_RABBITMQ_IP_COMMA`)
- MySQL 节点(`BK_MYSQL_IP_COMMA`)
- MongoDB 节点(`BK_MONGODB_IP_COMMA`)
- 单点 Redis 节点(`BK_REDIS_IP_COMMA`)
- Redis Sentinel 节点(`BK_REDIS_SENTINEL_IP_COMMA`)

#### Scenario: 基础设施 IP 与模块 IP 重叠

- **WHEN** `BK_CMDB_IP_COMMA=10.11.24.61` 且 `BK_MYSQL_IP_COMMA=10.11.24.61`
- **THEN** `AllHosts` 中 `10.11.24.61` 只出现一次

#### Scenario: 基础设施 IP 不在任何模块上

- **WHEN** ES7 IP `10.11.24.63` 不在任何 BK 模块 IP 列表中
- **THEN** `AllHosts` 包含 `10.11.24.63`,且对其执行 SSH 主机指标采集

### Requirement: 主机指标采集流程不变

系统 SHALL 对 `AllHosts` 中每台主机走两阶段 CPU 采样并并发收集 OS 指标(CPU、内存、磁盘、inode、文件句柄、运行天数等),保留现有 `collector/host.go` 的采集字段集合。

#### Scenario: 单台主机 SSH 不可达

- **WHEN** 某台 `AllHosts` 中的主机 SSH 连接失败
- **THEN** 报告中该主机条目 `Error` 字段记录 SSH 错误,不阻塞其他主机采集
