## Why

当前数据采集逻辑与真实蓝鲸交付环境(`reference/bk.env`)严重不符:`config.go` 读取的 `BK_MYSQL_IP / BK_REDIS_IP / BK_MONGODB_IP` 在 env 中实际是 bash 数组字面量,导出后无法解析,导致 MySQL / Redis / MongoDB 节点列表为空,这三类组件几乎采集不到任何数据。同时主机指标只覆盖 9 个 BK 业务模块的 IP,基础设施节点(独立部署的 ES7 / RabbitMQ / MySQL / MongoDB)的 OS 层指标完全缺失;Redis sentinel 集群、Mongo 副本集、阈值可配、SSH 多策略等也都未支持。本次集中修复以让巡检结果真实可用。

## What Changes

- **修复配置解析 bug**:`config.go` 改为从 `BK_MYSQL_IP_COMMA / BK_REDIS_IP_COMMA / BK_MONGODB_IP_COMMA` 读取多节点列表,字段类型由单值 `string` 改为 `[]string`(**BREAKING**:`Config.MySQLIP/RedisIP/MongoDBIP` 字段重命名为复数形式)
- **扩大主机指标采集范围**:`AllHosts` 在原有 9 个 BK 模块 IP 基础上,合并 ES7 / RabbitMQ / MySQL / MongoDB 各组件 `*_IP_COMMA` 解析出的真实设备 IP,逐个 SSH 采集 OS 指标
- **Redis 拆为两路采集**:`BK_REDIS_IP_COMMA` 走单点逐个连接探活;`BK_REDIS_SENTINEL_IP_COMMA` 走集群级状态检查(哨兵自身 + master 发现)
- **MongoDB 副本集语义**:连接 URI 带 `replicaSet=rs0`(从 `BK_GSE_MONGODB_RSNAME` 读取,默认 `rs0`),并对每个成员状态做检查
- **阈值支持 env 覆盖**:CPU / 磁盘 / inode / open files / 运行天数全部允许通过 `INSPECT_*_THRESHOLD` 等 env 覆盖默认值
- **SSH 多策略**:新增 `INSPECT_SSH_PORT` / `INSPECT_SSH_KEY_PATH` / `INSPECT_SSH_USE_SUDO`(仅支持 NOPASSWD)
- **MySQL/Redis 端口默认 + 覆盖**:`INSPECT_MYSQL_PORT`(默认 3306)、`INSPECT_REDIS_PORT`(默认 6379)

## Capabilities

### New Capabilities

- `bk-config-loading`:从 `BK_*` 环境变量加载蓝鲸部署拓扑(模块 IP / 基础设施 IP / 凭据 / 端口),为后续采集提供配置
- `host-metrics-collection`:对全量 BK 模块及基础设施节点 SSH 采集 OS 层指标(CPU / 内存 / 磁盘 / inode / 文件句柄 / 运行天数等)
- `infra-component-collection`:采集 ES7 / MySQL / Redis(单点)/ Redis Sentinel(集群)/ MongoDB(副本集)/ RabbitMQ 等基础设施组件的健康状态
- `threshold-config`:阈值配置(默认值 + env 覆盖)
- `ssh-connection`:SSH 连接策略(用户 / 端口 / 私钥 / NOPASSWD sudo)

### Modified Capabilities

(无 — 现有 `openspec/specs/` 为空,首次落 specs)

## Impact

- **代码**:`config/config.go`、`collector/host.go`、`collector/redis.go`、`collector/mongo.go`、`collector/mysql.go`、`collector/es.go`、`collector/rabbitmq.go`、`ssh/`(SSH 客户端构造)、`main.go`(传参 / 字段访问)
- **环境变量**:新增 `INSPECT_MYSQL_PORT`、`INSPECT_REDIS_PORT`、`INSPECT_CPU_THRESHOLD`、`INSPECT_DISK_THRESHOLD`、`INSPECT_INODE_THRESHOLD`、`INSPECT_MEM_THRESHOLD`、`INSPECT_MAX_OPEN_FILES`、`INSPECT_RUN_DAYS`、`INSPECT_SSH_PORT`、`INSPECT_SSH_KEY_PATH`、`INSPECT_SSH_USE_SUDO`
- **依赖**:可能需要引入 / 升级 Go MongoDB driver 以支持 `replicaSet=` 选项;Redis 客户端需支持 sentinel 探活
- **行为变化**:输出报告中主机数量将显著增加(覆盖基础设施节点);MySQL/Redis/MongoDB 由单连接变为多节点轮询
- **不变**:`ModuleRegistry` 9 个模块清单维持不变;不做按模块凭据(paas/bklog/job 等)的 vhost/db 级体检
