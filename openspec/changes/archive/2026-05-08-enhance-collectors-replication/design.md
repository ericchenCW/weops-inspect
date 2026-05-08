## Context

姊妹 change `align-collectors-with-bk-env` 让 MySQL / Redis / MongoDB / Sentinel 的多节点采集走通,但仍然把节点视为平权 — 每个节点单独连接探活,不感知主从角色。

新版 `bk.env`(WeOps 部署)在 `BK_MYSQL_IP_COMMA` 之外明确给出了:

```
BK_MYSQL_MASTER_IP_COMMA   = <master IPs>
BK_MYSQL_SLAVE_IP_COMMA    = <slave IPs>
BK_REDIS_MASTER_IP_COMMA   = <master IPs>
BK_REDIS_SLAVE_IP_COMMA    = <slave IPs>
```

这表示部署侧已经把"谁是主、谁是从"显式记录在了 env 里。结合现网经验,主从复制故障(IO/SQL 线程停掉、延迟飙升、角色翻转)远比"连接不通"更高频,且**仅靠连接探活无法发现**。本 change 就是把这部分盲点补上。

旧版 `bk.env` 不包含 `*_MASTER/SLAVE_IP_COMMA`。因此实现必须做向下兼容 — 这两组 env 为空时不破坏现有行为。

## Goals / Non-Goals

**Goals:**

- 让 MySQL slave 节点的 `Slave_IO/SQL_Running` 与延迟在巡检报告中可见
- 让 MySQL master 的 `read_only` 状态被校验
- 让 Redis 节点的 `role` 与 env 配置一致性被校验
- 让 Redis slave 的 `master_link_status` 与同步指标在报告中可见
- 让 Sentinel 选出的 master 与 env 中 master 是否一致被对照
- 复制相关阈值可由 env 覆盖

**Non-Goals:**

- 不做数据一致性校验(checksum、行数对比)
- 不引入新的 driver / 客户端库
- 不实现自动故障切换或主从切换建议
- 不针对每个业务模块的 DB / vhost 做凭据级体检(留给将来)
- 不动 MongoDB(MongoDB 副本集成员状态在姊妹 change 已覆盖,本次不再扩展)

## Decisions

### D1. 主从角色来源:env 而非运行时探测

- **选择**:从 `BK_MYSQL_MASTER_IP_COMMA / BK_MYSQL_SLAVE_IP_COMMA / BK_REDIS_MASTER_IP_COMMA / BK_REDIS_SLAVE_IP_COMMA` 读取角色划分
- **替代 1**:运行时连接所有节点,通过 `SHOW SLAVE STATUS` / `INFO replication` 自动判定 — 否决,因为 env 已经显式声明,运行时探测反而多一次绕路;且当我们想做"角色校验"时必须有一个"声明的真值"作对照
- **替代 2**:全部走全集 `BK_MYSQL_IP_COMMA`,只在节点上探测角色 — 否决,失去了"env 配置 vs 运行时实际"的对照能力
- **后果**:env 不显式划分时(旧 bk.env)collector 跳过复制采集,在报告中此节标记 "N/A — env 未配置主从信息"

### D2. MySQL slave 状态字段范围

- **采集**:`Slave_IO_Running`、`Slave_SQL_Running`、`Seconds_Behind_Master`、`Last_IO_Error`、`Last_SQL_Error`、`Master_Host`
- **不采集**:`Read_Master_Log_Pos`、binlog file/pos 系列(对运维报告价值低,且字段易随版本变化)
- **判定**:任一 `Running != Yes` → critical;`Seconds_Behind_Master > INSPECT_MYSQL_REPL_LAG_THRESHOLD` → warn

### D3. MySQL master 仅校验 `read_only`

- **采集**:`SHOW VARIABLES LIKE 'read_only'`
- **判定**:`read_only = ON` 在 master 节点上 → warn(可能是误配置或未及时切换)
- **不做**:`super_read_only` / `binlog_do_db` / `gtid_mode` 等更细的判定 — 不同部署习惯差异大,容易引入噪音告警

### D4. Redis 复制采集走 `INFO replication`

- **采集字段**:
  - `role`(master / slave)
  - master 节点:`connected_slaves`、`slaveX:state` 字段统计
  - slave 节点:`master_host`、`master_port`、`master_link_status`、`master_last_io_seconds_ago`、`master_sync_in_progress`
- **判定**:
  - `role` 与 env 声明不一致 → warn
  - slave 的 `master_link_status != up` → critical
  - slave 的 `master_last_io_seconds_ago > INSPECT_REDIS_REPL_IO_THRESHOLD` → warn

### D5. Sentinel 与 env 交叉校验

- **做法**:在姊妹 change 已实现的 `CollectRedisSentinel` 之后,把发现的 master 地址与 `RedisMasterIPs` 做集合对照
- **判定**:不一致(sentinel 报的 master IP 不在 env 配置的 master 列表里)→ warn,提示"实际 master 与配置不符"
- **替代**:不做对照 — 否决,因为这正是本 change 提升真实价值的关键之一(发现"env 没及时更新"或"非预期主从切换")

### D6. 阈值默认值

- `INSPECT_MYSQL_REPL_LAG_THRESHOLD` = 60(秒)
- `INSPECT_REDIS_REPL_IO_THRESHOLD` = 10(秒)
- 解析失败硬退出,与姊妹 change 的阈值处理一致

### D7. 与现有 Config / Collector 的接口

- 在姊妹 change 已建立的 `Config.MySQLIPs / RedisIPs / RedisSentinelIPs` 旁增加 4 个新字段;**不**取代原字段
- 平权探活(对全集 IP 的连接探活)继续保留,本 change 只追加复制状态采集步骤
- 报告结构通过新增字段(非修改/重命名)向后兼容

## Risks / Trade-offs

- **MySQL `SHOW SLAVE STATUS` 权限** → root 用户(`BK_MYSQL_ADMIN_USER=root`)有 `REPLICATION CLIENT` 权限,无问题;若客户使用降权账号需在文档说明
- **env 与实际主从不一致(常见的"切了主忘了改 env")** → 这正是 D5 的对照价值,不是风险
- **Redis ACL 限制** → 部分新版 Redis 启用 ACL 后 `INFO replication` 可能受限;评估在低版本(redis 5/6)中无问题,acl-限制场景给出明确报错
- **`*_MASTER/SLAVE` env 缺失环境** → 跳过复制采集,行为退化为姊妹 change 的平权探活
- **Sentinel master 与 env master 暂时不一致(刚切完未更新 env)** → 仅报 warn,不报 critical,避免在切换窗口期产生 paging 噪音

## Migration Plan

1. **前置**:`align-collectors-with-bk-env` 必须先 archive(本 change 依赖其建立的 Config 字段与 collector 拆路)
2. 按 specs / tasks 推进实现
3. **验证**:在新版 bk.env 环境跑一次完整巡检,确认:
   - MySQL slave 节点报告中含 `Slave_IO/SQL_Running` 与 `Seconds_Behind_Master`
   - master 节点 `read_only` 状态在报告中可见
   - Redis slave 节点报告中含 `master_link_status` 与 `master_last_io_seconds_ago`
   - Sentinel 集群与 env master 一致性校验生效
   - 旧 bk.env 环境(无 `*_MASTER/SLAVE_*`)跑同样命令,复制段落标记为 "N/A",不报错
4. **回滚**:还原此次 change 提交即可

## Open Questions

- **OQ1**:Redis 启用 ACL 时 `INFO replication` 的输出可能截断,需在实现时确认;若受限,降级为只报 `role`
- **OQ2**:`Seconds_Behind_Master` 在 GTID 模式下偶尔为 NULL(slave 刚追上时),实现时按 NULL → 视为 0 处理,文档说明
- **OQ3**:是否在报告 summary 中单独统计"复制健康"项数量(如 `ReplicationOK / ReplicationWarn`)?暂按"作为 MySQL/Redis 子检查项纳入总数"处理,不单立分类
