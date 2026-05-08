## Context

`weops-inspect` 的目的是对蓝鲸社区版部署做一次性巡检并产出报告。它通过 SSH 远程执行命令采集主机指标,并直连开源组件(MySQL / Redis / Mongo / ES / RabbitMQ)做健康检查。配置全部来自蓝鲸交付时生成的 `bk.env`(变量名以 `BK_` 开头)。

当前实现(见 `config/config.go`)对 `bk.env` 的实际形态有几处误解,最严重的是把数据库类组件 IP 当成单值字符串读,而 `bk.env` 里这些是 bash 数组字面量(只有 `*_IP_COMMA` 形式才是规范的、source 后可被外部读取的环境变量)。结果是 MySQL / Redis / MongoDB 的采集链路从入口就拿不到节点。

进一步地,真实 `bk.env` 揭示几个被忽略的拓扑事实:

- ES7 / RabbitMQ / MySQL / MongoDB 在标准部署中可能落在与 9 个 BK 业务模块**不重叠**的设备上,这些设备的 OS 指标当前完全采不到
- Redis 同时存在两套:**单点 Redis 集合**(`BK_REDIS_IP_COMMA`)和**Sentinel 集群**(`BK_REDIS_SENTINEL_IP_COMMA`),语义不同需要分开处理
- MongoDB 是 3 节点副本集(`BK_GSE_MONGODB_RSNAME=rs0`),单点直连不能反映集群健康
- 阈值与 SSH 配置目前硬编码,在不同客户场景下缺乏弹性

本次设计聚焦把这些不一致一次性扳正,但**不**扩大模块覆盖范围(不补 auth/ssm/license/bklog/apigw),也**不**做按模块凭据(paas/bklog/job 等各自 user/vhost/db)的精细体检 — 这些在探索阶段已与用户明确过,留待后续 change。

## Goals / Non-Goals

**Goals:**

- 修复 `BK_*_IP` 误读 bug,所有数据库类组件统一从 `*_IP_COMMA` 读取并支持多节点
- 让 `AllHosts` 包含所有需要 SSH 采指标的真实设备 IP(BK 模块 + 基础设施)
- Redis 单点 / Sentinel 两条独立路径
- MongoDB 以副本集语义连接并检查每个成员
- 阈值与 SSH 全部允许 env 覆盖
- 保持 `main.go` 三阶段(主机指标 / 服务状态 / 开源组件)的整体流水线不变

**Non-Goals:**

- 不扩展 `ModuleRegistry`(auth/ssm/license/bklog/apigw 不补)
- 不做按业务模块凭据的 MySQL DB / RabbitMQ vhost / Redis 实例级体检
- 不重写 `ssh/` 客户端的整体抽象,只增加配置项
- 不引入 sudo 密码注入等安全敏感能力(仅 NOPASSWD)
- 不动 `checker/rules.go` 的具体阈值逻辑,只让阈值来源可配

## Decisions

### D1. `Config` 字段:`MySQLIP/RedisIP/MongoDBIP` → `MySQLIPs/RedisIPs/MongoDBIPs`(`[]string`)

- **Why**:bk.env 里这三类组件本就是多节点部署(MongoDB 3 节点副本集、Redis 多哨兵 + 多单点、MySQL 集群)。继续用单值字符串无法真实反映拓扑。
- **替代**:保留单值字符串,内部用第一个 IP。否决,因为这就是当前 bug 的本质。
- **影响**:`collector/mysql.go` / `redis.go` / `mongo.go` / `main.go` 的字段访问需要同步改;Open source 类输出报告结构要支持多节点结果集。

### D2. `AllHosts` 合并基础设施 IP

- **Why**:基础设施节点的 OS 指标(磁盘 / inode / 文件句柄)往往是数据库类故障的根因。
- **怎么合**:在 `buildAllHosts` 中,除原 9 个模块外,再追加 `ES7IPs`、`RabbitMQIPs`、`MySQLIPs`、`MongoDBIPs`、`RedisIPs`(单点 Redis 节点设备),以及 `RedisSentinelIPs`(哨兵节点设备)。仍走去重 set。
- **风险**:这些设备的 SSH 凭据不一定与 BK 模块设备一致 → 由 SSH 多策略(D6)缓解;若仍连不上则按现有"SSH error"语义在报告里标错,不阻塞整体流程。

### D3. Redis 拆两路

- **Why**:单点 Redis 与 Sentinel 集群的可用性语义不同。单点是"每个节点能 PING 通",Sentinel 集群是"哨兵法定多数 + master 已知 + master 可达"。
- **数据结构**:报告中 `Redis` 字段拆为 `RedisStandalone []NodeStatus` 和 `RedisSentinel SentinelClusterStatus`(包含每个 sentinel 自身状态、发现的 master 地址、master 探活)。
- **替代**:用一个 collector 自动判断模式。否决,因为两份 IP 列表都存在(`BK_REDIS_IP_COMMA` 和 `BK_REDIS_SENTINEL_IP_COMMA`),直接按列表分别走更清晰。

### D4. MongoDB 以副本集连接

- **Why**:`BK_GSE_MONGODB_RSNAME=rs0` 显示真实是副本集。
- **怎么连**:URI 形如 `mongodb://user:pwd@ip1:27017,ip2:27017,ip3:27017/?replicaSet=rs0`,从 `Config.MongoDBIPs` 拼接。`rs` 名称从新 env `INSPECT_MONGO_RS_NAME` 读取(默认 `rs0`,因为 `BK_GSE_MONGODB_RSNAME` 是模块级 env,语义不通用)。
- **检查项**:连接成功后调用 `replSetGetStatus`,记录每个成员的 `stateStr`(PRIMARY / SECONDARY / etc.)。
- **依赖**:`go.mod` 已有 `mongo-driver`(待确认),如果版本过旧需要升级。

### D5. 阈值 env 覆盖

- 在 `Thresholds` 加载处增加 env 解析,优先级:env > 默认值。命名约定:
  - `INSPECT_CPU_THRESHOLD`(默认 75)
  - `INSPECT_DISK_THRESHOLD`(默认 75)
  - `INSPECT_INODE_THRESHOLD`(默认 75)
  - `INSPECT_MEM_THRESHOLD`(默认 75)
  - `INSPECT_MAX_OPEN_FILES`(默认 102400)
  - `INSPECT_RUN_DAYS`(默认 365)
- 解析失败(非法数字)→ 报错退出,避免静默用错值。

### D6. SSH 多策略

- 新增 env:
  - `INSPECT_SSH_PORT`(默认 22)
  - `INSPECT_SSH_KEY_PATH`(可选;为空则回落到默认 agent / `~/.ssh/id_rsa` 当前行为)
  - `INSPECT_SSH_USE_SUDO`(默认 `false`;`true` 时所有远程命令前加 `sudo `,**仅** NOPASSWD 场景适用)
- 不引入密码 sudo / 多用户 / 跳板机等;保持 `SSHUser` 仍是单一用户。

### D7. MySQL/Redis 端口

- 新增 env `INSPECT_MYSQL_PORT`(默认 3306)、`INSPECT_REDIS_PORT`(默认 6379)。
- bk.env 中只有带模块前缀的 PORT(如 `BK_PAAS_MYSQL_PORT`),无全局 PORT,故约定用 `INSPECT_*` 而非读取某个 BK_* 端口。
- `Config.MySQLPort / RedisPort` 字段保持 `string`,但默认来源改为新 env。

## Risks / Trade-offs

- **基础设施 IP SSH 不可达** → 巡检报告该节点显示 SSH error,但不影响其他节点;通过 D6 的 key/port/sudo 配置降低概率。
- **`Config` 字段重命名(BREAKING)** → 仅有内部消费者(`main.go` + `collector/*`),一次性同步改完即可,无外部用户影响。
- **MongoDB 副本集 URI 中混入不可达节点** → driver 自身有节点选择重试机制;在所有节点都不可达时 collector 记录连接失败。
- **阈值解析失败硬退出** → 与"silent default"对比的是"用户感知到了配错",更安全。
- **NOPASSWD-only sudo** → 在需要密码 sudo 的环境会失败,但避免了在巡检工具里管理 sudo 密码的安全复杂性。

## Migration Plan

1. 实现按 spec / tasks 顺序推进
2. 由于这是巡检工具(单次运行 / 无服务化部署),无需 staged rollout
3. 验证方式:在测试环境用 `bk.env` source 后跑一次完整巡检,对照报告核对:
   - MySQL/Redis/Mongo 是否输出多节点结果
   - 报告主机数 = `len(AllHosts)`,且包含 ES/RabbitMQ/Mongo/MySQL 节点
   - Redis 报告分 standalone 与 sentinel 两段
   - Mongo 报告含每个成员的 `stateStr`
4. 回滚:还原此次 change 的所有提交即可,无外部状态。

## Open Questions

- **Open Q1**:`replSetGetStatus` 需要 `clusterMonitor` 角色,当前用 `BK_MONGODB_ADMIN_USER=root` 应该够,但需要在实现时确认是否有权限拒绝场景。
- **Open Q2**:Sentinel 集群级状态的"健康"判定阈值 — 几个 sentinel 不可达视为 degraded?默认建议 "**任意一个** sentinel 不可达即 warn,master 不可达即 critical"。落实现时再敲定。
