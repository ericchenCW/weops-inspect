## Why

`CollectES` 现在只取 `cfg.ES7IPs[0]` 作为 ES 集群的探测入口:

```go
host := cfg.ES7IPs[0]
port := "9200"
```

只要这第一个 IP 上的 ES 没起(例如它是中控/构建节点,9200 监听不存在),整个 ES 集群体检就直接 `connection refused` fail,即使其它 2 个数据节点完全健康。

实测:

```
BK_ES7_IP_COMMA=10.11.24.60,10.11.24.63,10.11.24.64
curl 10.11.24.60:9200 → Connection refused
curl 10.11.24.63:9200 → 200 OK
curl 10.11.24.64:9200 → 200 OK
```

报告显示 ES 整体不可用 — 误报。

## What Changes

- **并发探测所有 IP 的 9200 reachability**:对 `cfg.ES7IPs` 中每个 IP 起 goroutine,做一次 `curl http://<ip>:9200/` (或 TCP connect) 的可达性检查,记录 `{IP, Status: ok|unreachable, Detail}`。
- 从可达节点中**任选第一个**调用 `_cluster/health` 与 `_cat/nodes` 取集群信息。
- `model.ESCluster` 新增 `NodeReachability []ESNodeReach` 字段,渲染层在 ES 段加一栏 per-node reachability 表。
- 当**所有** IP 都不可达时,整体 `Error="all nodes unreachable"`、`ErrorClass=ErrNetwork`,与现状 fail 行为对齐。
- `curlGet` 的 `--max-time 5` 在并发场景下作为单次探测上限可接受(N 个节点并行,而非串行 N×5s)。

## Capabilities

### New Capabilities
- (无)

### Modified Capabilities
- `infra-component-collection`: ES7 采集从"取首个 IP 集群级"扩展为"全节点 reachability + 首个可达节点取集群信息"。

## Impact

- **代码**
  - [collector/es.go](collector/es.go) — fan-out 探测逻辑;选 leader 节点取 health。
  - [model/types.go](model/types.go) — `ESCluster.NodeReachability` 新字段、`ESNodeReach` 新结构。
  - [render/templates/](render/templates/) — opensources 模板 ES 章节追加 reachability 表。
- **行为**
  - 部分节点宕机不再导致整体 fail。
  - 报告 JSON `ES7[*].NodeReachability` 为新增字段(向后兼容,旧消费方忽略即可)。
- **依赖 / 接口**:无外部 API 变化。
