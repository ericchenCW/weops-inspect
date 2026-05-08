## ADDED Requirements

### Requirement: 不依赖外部 CLI 二进制

系统 SHALL 通过纯 Go 驱动直接连接 MySQL / Redis / MongoDB 进行采集,SHALL NOT 调用 `mysql` / `redis-cli` / `mongosh` 等外部二进制。RabbitMQ Mgmt API 与 ES HTTP 接口在本能力范围内可继续使用 `curl`,但 MUST 通过统一探针框架包装以共享超时与错误分类。

#### Scenario: 巡检机未安装 mysql 客户端

- **WHEN** 巡检机 PATH 中不存在 `mysql` 可执行文件,但目标 MySQL 节点正常
- **THEN** 巡检报告中 MySQL 段如实反映目标节点状态(版本、连接成功),不再因二进制缺失给出 `not available`

#### Scenario: 巡检机未安装 redis-cli

- **WHEN** 巡检机 PATH 中不存在 `redis-cli` 可执行文件
- **THEN** Redis 单点 / Sentinel 探测仍能完成,报告反映目标真实状态

#### Scenario: 巡检机未安装 mongosh

- **WHEN** 巡检机 PATH 中不存在 `mongosh` / `mongo` 可执行文件
- **THEN** MongoDB 副本集状态采集仍能完成

### Requirement: 每次组件探测受 ctx 超时约束

系统 MUST 让每个组件 collector 把所有网络调用包裹在 `context.Context` 之内,任何单次探测在框架默认 5 秒超时(或显式覆盖)未完成时立刻终止,SHALL NOT 让单个僵死节点阻塞整体巡检。

#### Scenario: MySQL 节点僵死

- **WHEN** `Config.MySQLIPs` 中某节点 TCP 端口可达但握手不响应
- **THEN** 5 秒后该节点结果标记 `ErrorClass = timeout`,巡检继续处理下一节点

#### Scenario: Mongo 副本集主不可达

- **WHEN** Mongo URI 中所有种子节点都无响应
- **THEN** 5 秒内 collector 返回 `ErrorClass = timeout`,主流程继续

### Requirement: 组件错误结果携带 ErrorClass

系统 SHALL 在所有组件 collector 的报告字段中新增 `ErrorClass` 字段(类型为字符串枚举,见 `collector-probe-framework`),并在每次探测失败时填入对应分类;字段缺省值为空字符串表示无错误。原有 `Error` 自由文本字段保持向后兼容。

#### Scenario: MySQL 鉴权失败

- **WHEN** 目标 MySQL 返回错误码 1045
- **THEN** 该节点 `Error` 含原始错误文本(密码已脱敏),`ErrorClass = "auth"`

#### Scenario: Redis 网络不可达

- **WHEN** 目标 Redis IP 无法建立 TCP 连接
- **THEN** `Error` 含 `connection refused` 或等价文本,`ErrorClass = "network"`

#### Scenario: 兼容老报告消费者

- **WHEN** 渲染层未消费 `ErrorClass` 字段
- **THEN** 系统行为与原有相同,基于 `Error` 是否非空判级

### Requirement: MySQL 列扫描兼容字段重命名

系统 SHALL 通过 `rows.Columns()` + `rows.Scan(&[]sql.RawBytes)` 的形式按列名读取 `SHOW SLAVE STATUS` / `SHOW REPLICA STATUS`,MUST 同时识别 `Slave_IO_Running` 与 `Replica_IO_Running` 等等价字段,以兼容 MySQL 5.7 / 8.0 / 8.4。

#### Scenario: MySQL 5.7 主从

- **WHEN** 5.7 节点执行 `SHOW SLAVE STATUS` 返回 `Slave_IO_Running`
- **THEN** 报告中复制线程状态字段被正确填充

#### Scenario: MySQL 8.4 主从

- **WHEN** 8.4 节点执行 `SHOW REPLICA STATUS` 返回 `Replica_IO_Running`
- **THEN** 报告中复制线程状态字段以与 5.7 一致的语义被填充

### Requirement: MySQL binlog 数量采集兼容

系统 SHALL 优先执行 `SHOW BINARY LOGS`,失败时回退到 `SHOW MASTER LOGS`,以兼容 MySQL 8.4 中的命令重命名。

#### Scenario: MySQL 8.4 不再支持 SHOW MASTER LOGS

- **WHEN** 目标节点为 8.4 且不支持 `SHOW MASTER LOGS`
- **THEN** collector 走 `SHOW BINARY LOGS` 路径,binlog 数量字段被正确填充
