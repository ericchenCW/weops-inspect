## Context

`CollectES` 现以 `cfg.ES7IPs[0]` 为唯一探测入口:

```go
host := cfg.ES7IPs[0]
port := "9200"
```

实际部署中 `BK_ES7_IP_COMMA` 第一个 IP 常常是中控/构建节点(只装 ES 客户端或不装),9200 不监听。这导致整个 ES 体检 fail,即便其它两个数据节点 200 OK。

## Goals / Non-Goals

**Goals:**
- 单节点宕机不导致整体 ES 巡检失败。
- 报告中给出每节点 reachability,运维一眼能定位是哪台 ES 没起。
- 用并发避免 N×5s 串行超时累加。

**Non-Goals:**
- 不主动把"集群级 health"打散到每节点(集群信息是 cluster-wide,只需从一个可达节点取一次)。
- 不改 ES 客户端实现(仍走 `curl`)。
- 不改 `--max-time 5` 默认值(单次探测的合理上限)。

## Decisions

### D1: fan-out + 选首个可达

```
ES7IPs ──┬─ goroutine probe(.60:9200) ─┐
         ├─ goroutine probe(.63:9200) ─┼─→ collect NodeReachability[]
         └─ goroutine probe(.64:9200) ─┘
                                       │
                                       └─ pick first reachable → cluster_health & cat nodes
```

实现方案:用 `sync.WaitGroup` + 切片(预分配 N 长度,index = i,goroutine 写各自下标,无需 mutex)。

### D2: 节点可达性探测用 `curl http://<ip>:9200/`

替代:

| | 实现 | 取舍 |
|---|---|---|
| **A. curl GET /**(选定) | 复用现有 `curlGet`,记 HTTP code / 错误码 | 与现有 ES7 单点探测语义一致,exit 7 = unreachable |
| B. `net.DialTimeout` 9200 | 更快、不依赖 curl | 失去 HTTP 层信号(端口 listen 但 ES 死锁会被误判为 ok) |

选 A。考虑到 9200 是 HTTP 端口,curl 一次握手成本低。

### D3: 选 leader 不重排,保持 `cfg.ES7IPs` 的顺序语义

按 `cfg.ES7IPs` 顺序遍历 `NodeReachability`,选第一个 `Status="ok"` 的 IP 作为集群入口。这样运维若想"优先使用某节点取集群信息",在 `BK_ES7_IP_COMMA` 中把它排前即可。

### D4: 当首选节点之外的 IP 也写到 `cluster.Instance`

`Instance` 字段当前是 `host:9200`,代表"集群入口"。改动后 Instance 取实际选中的入口 IP。运维看 Instance 列即知本次 cluster_health 来自哪台。

### D5: `--max-time 5` 在并发场景下不放大延迟

并发后,N 个 down 节点最多累加 5s 整体延迟(并行),不再 N×5s 串行。无需调整超时。

### D6: 模型扩展

`model.ESCluster` 新增 `NodeReachability []ESNodeReach`;新增 `ESNodeReach{IP, Status, Detail}` 结构。HTML 模板在 ES 章节追加 reachability 表(IP / 状态 / 详情)。JSON 兼容 — 旧字段不动,新字段为追加。

## Risks / Trade-offs

- [若所有节点都 ok 但 cluster 处于 `red`] → `ESCluster.Status="red"` 字段照常体现,与 reachability 正交,不冲突。
- [并发起 N 个 curl 子进程在大集群上有 fork 开销] → ES7 集群通常 <20 节点,可忽略。
- [选首个可达 IP 取 `_cat/nodes` 拿到的节点 IP 与 `NodeReachability` 中 IP 不一定逐一对齐] → 二者维度不同(reachability = 我能否连上 9200;`_cat/nodes` = ES 集群感知到的节点 IP)。文档/模板里把它们各成一栏即可,不强行 join。

## Migration Plan

- 二进制升级即可,无 env 变化。
- 报告 JSON 中 `ES7[*].NodeReachability` 是新字段,旧消费方读取时忽略即可,无 breaking。
- 回滚:还原代码,行为退回"取首个 IP"。
