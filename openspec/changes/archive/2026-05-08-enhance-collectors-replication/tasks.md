## 1. 前置确认

- [x] 1.1 确认姊妹 change `align-collectors-with-bk-env` 已实现并 archive(`Config.MySQLIPs / RedisIPs / RedisSentinelIPs` 与 collector 拆路存在) — **已实现,未 archive,但代码就位**
- [ ] 1.2 在测试环境用 `reference/bk.env` source 后确认 `BK_MYSQL_MASTER_IP_COMMA` / `BK_MYSQL_SLAVE_IP_COMMA` / `BK_REDIS_MASTER_IP_COMMA` / `BK_REDIS_SLAVE_IP_COMMA` 可被读取 — **需真实环境**

## 2. Config 字段与阈值

- [x] 2.1 在 `config/config.go` 中新增 `MySQLMasterIPs []string`、`MySQLSlaveIPs []string`、`RedisMasterIPs []string`、`RedisSlaveIPs []string` 字段
- [x] 2.2 `Load()` 中通过 `parseIPList` 解析对应 `BK_*_MASTER/SLAVE_IP_COMMA`
- [x] 2.3 `Thresholds` 中新增 `MySQLReplLagSec int`、`RedisReplIOSec int` 字段
- [x] 2.4 复用 `parseIntEnv`(姊妹 change 已新增)读取 `INSPECT_MYSQL_REPL_LAG_THRESHOLD`(默认 60)、`INSPECT_REDIS_REPL_IO_THRESHOLD`(默认 10),非法数字硬退出

## 3. Model 类型扩展

- [x] 3.1 在 `model/types.go` 中新增 `MySQLReplicationStatus`(含 `IORunning`、`SQLRunning`、`SecondsBehindMaster`、`LastIOError`、`LastSQLError`、`MasterHost`、`Status`)
- [x] 3.2 新增 `MySQLMasterStatus`(含 `ReadOnly`、`Status`)
- [x] 3.3 新增 `RedisReplicationStatus`(含 `Role`、`MasterHost/Port`、`MasterLinkStatus`、`MasterLastIOSeconds`、`MasterSyncInProgress`、`ConnectedSlaves`、`RoleConsistencyStatus`、`LinkStatus`)
- [x] 3.4 新增 `MySQLSlaveStatus`(包 IP + Replication 指针)与 `ReplicationReport` 聚合体;由 `InspectReport.Replication *ReplicationReport` 提供给报告层
- [x] 3.5 在 Sentinel 集群结果中新增 `MasterEnvMatch string`(`ok`/`warn`/`N/A`)字段

## 4. MySQL collector 复制采集

- [x] 4.1 实现 `collectMySQLSlave(...)`(在 `collector/replication.go`),执行 `SHOW SLAVE STATUS\G`,空结果集 → not-configured-as-slave
- [x] 4.2 实现 `collectMySQLMaster(...)`,执行 `SELECT @@read_only`
- [x] 4.3 在 `CollectReplication` 中遍历 `Config.MySQLMasterIPs / MySQLSlaveIPs`
- [x] 4.4 阈值判定:`Slave_IO_Running != Yes` 或 `Slave_SQL_Running != Yes` → critical;`Seconds_Behind_Master > Thresholds.MySQLReplLagSec` → warn;NULL → 视作 0
- [x] 4.5 主从字段为空时跳过(`CollectReplication` 在所有列表为空时返回 nil)

## 5. Redis collector 复制采集

- [x] 5.1 实现 `collectRedisReplication(...)`,执行 `INFO replication` 并解析
- [x] 5.2 在 `CollectReplication` 中遍历 `Config.RedisMasterIPs / RedisSlaveIPs`
- [x] 5.3 角色一致性判定:实际 `role` 与 env 声明不符 → `RoleConsistencyStatus="warn"`
- [x] 5.4 链路状态判定:slave `master_link_status != up` → `LinkStatus="critical"`;`master_last_io_seconds_ago > Thresholds.RedisReplIOSec` → `LinkStatus="warn"`

## 6. Sentinel 与 env master 交叉校验

- [x] 6.1 实现 `CrossCheckSentinelMaster(s, masterIPs)`,在 `main.go` 中 `CollectRedisSentinel` 后调用
- [x] 6.2 不一致时设置 `MasterEnvMatch="warn"`
- [x] 6.3 一致时设置 `MasterEnvMatch="ok"`
- [x] 6.4 `Config.RedisMasterIPs` 为空时,`MasterEnvMatch="N/A"`

## 7. Checker / 报告渲染

- [x] 7.1 在 `checker/rules.go` 中新增 `CheckReplication`,涵盖 master `read_only`、slave 复制状态
- [x] 7.2 同上,涵盖 Redis 角色一致性与 link/IO 阈值
- [x] 7.3 在 `render/templates/opensources.html.tmpl` 中追加"主从复制健康"段
- [x] 7.4 Sentinel 表格增加"与 env 一致"列(`MasterEnvMatch`)
- [x] 7.5 `report.Summary` 通过 `allChecks = append(allChecks, CheckReplication(...)...)` 纳入复制检查计数

## 8. 验证

- [ ] 8.1 在新版 bk.env 环境跑完整巡检:确认 MySQL slave 节点报告中含 `Slave_IO/SQL_Running` 与 `Seconds_Behind_Master` — **需真实环境**
- [ ] 8.2 确认 MySQL master 节点 `read_only` 状态可见 — **需真实环境**
- [ ] 8.3 确认 Redis slave 节点报告中含 `master_link_status` 与 `master_last_io_seconds_ago` — **需真实环境**
- [ ] 8.4 确认 Sentinel master 与 env master 交叉校验项呈现 — **需真实环境**
- [ ] 8.5 在旧 bk.env 环境(无 `*_MASTER/SLAVE_*`)跑同样命令,确认复制段标 `N/A`,不报错,平权探活仍正常 — **需真实环境**
- [ ] 8.6 验证两个新阈值 env 覆盖生效;非法数字时进程退出 — **需真实环境**
- [x] 8.7 `go build ./...` 与 `go vet ./...` 通过
