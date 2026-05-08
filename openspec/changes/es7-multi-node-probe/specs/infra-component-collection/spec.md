## MODIFIED Requirements

### Requirement: ES7 与 RabbitMQ 多节点采集保留

系统 SHALL 维持 ES7 与 RabbitMQ 的多节点采集能力,IP 列表分别来自 `Config.ES7IPs` 与 `Config.RabbitMQIPs`,凭据使用 `Config.Creds.ES7Password` 与 `Config.Creds.RabbitMQUser/RabbitMQPassword`。

ES7 采集流程 SHALL 满足:

- 对 `Config.ES7IPs` 中**每个** IP 并发执行 9200 端口可达性探测,每节点结果记入 `ESCluster.NodeReachability []ESNodeReach`,字段含 `IP / Status / Detail`,`Status ∈ {ok, unreachable}`。
- 从可达节点中**选第一个** IP 作为集群入口,对其 `_cluster/health` 与 `_cat/nodes?format=json` 取集群与节点指标,填入 `ESCluster.Status / NumberOfNodes / Nodes / ...` 等现有字段。
- 当**所有** IP 都不可达时,`ESCluster.Error="all nodes unreachable"`,`ErrorClass=ErrNetwork`,不阻塞其它组件采集。

RabbitMQ 采集行为本 change 不变更。

#### Scenario: 部分节点不可达不影响集群健康判定

- **WHEN** `BK_ES7_IP_COMMA=10.11.24.60,10.11.24.63,10.11.24.64`,且仅 `.60:9200` 拒接,`.63 / .64` 正常
- **THEN** `ESCluster.NodeReachability` 含 3 条记录,`.60` 为 `unreachable`,其它两条 `ok`;`ESCluster.Status / Nodes` 等字段从 `.63` 或 `.64` 取得;`ESCluster.Error` 为空

#### Scenario: 全部节点不可达

- **WHEN** 所有 IP 的 9200 端口均不可达
- **THEN** `ESCluster.NodeReachability` 全部为 `unreachable`,`ESCluster.Error="all nodes unreachable"` 且 `ErrorClass="network"`

#### Scenario: 单节点 ES7

- **WHEN** `BK_ES7_IP_COMMA=10.11.24.63` 且该节点健康
- **THEN** `NodeReachability` 含 1 条 `ok`,集群指标从该节点取得

#### Scenario: 现有 ES7 字段保持

- **WHEN** ES7 巡检完成
- **THEN** 报告中 `ES7[*]` 仍含 `Instance / ClusterName / Status / NumberOfNodes / Nodes / ...` 等原有字段;新增 `NodeReachability` 不替换它们
