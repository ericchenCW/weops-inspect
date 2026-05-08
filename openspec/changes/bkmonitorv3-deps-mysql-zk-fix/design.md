## Context

`CollectBKMonitorV3Deps` 在生产环境出现两类系统性误报:

- **MySQL** 探两个端点(PaaS / Monitor),实际只有一套 MySQL,通过 Consul 服务发现 `mysql.service.consul:3306` 暴露;部分新部署里 `BK_*_MYSQL_HOST` env 干脆不导出,导致 probe 直接 `skip` 而看不出健康状态。
- **ZooKeeper** 用 `ruok` 4lw 命令探活,但蓝鲸 ZK 默认 `4lw.commands.whitelist=srvr`,`ruok` 被禁,probe 总返回 `fail: no ruok response (4lw disabled?)`,实际 ZK 完全健康。

## Goals / Non-Goals

**Goals:**
- 让 MySQL 探测成为单条、默认值贴近蓝鲸生产实际(Consul service)。
- 让 ZK 探测不再因 4lw whitelist 误报。
- 保留 env 覆盖能力,不绑死。

**Non-Goals:**
- 不切换 MySQL 客户端实现(仍走 `mysql` CLI)。
- 不引入 ZK 协议库做 srvr 解析。TCP connect 已足够。
- 不改 RabbitMQ / Redis / ES7 / InfluxDB 探测逻辑。

## Decisions

### D1: MySQL 合并为单条 `Item="mysql"`

PaaS 与 Monitor 两个 service 使用同一物理 MySQL 已是事实标准,分别探测只重复消耗连接、放大假阳性。合并并标记 `Item="mysql"`,Detail 不再区分 service 维度。

加载优先级:`BK_MONITOR_*` > `BK_PAAS_*` > 默认 `mysql.service.consul:3306`。这种回退顺序保留了"用户可强制指定 monitor 专用 MySQL"的能力,同时让最常见的"两 service 同库"场景零配置即可工作。

### D2: ZK 改为纯 TCP `DialTimeout`

替代方案对比:

| | 实现 | 取舍 |
|---|---|---|
| **A. 纯 TCP connect**(选定) | `net.DialTimeout` 5s,关闭,return ok | 简单,与 4lw 解耦 |
| B. 发送 `srvr` 解析多行响应 | 加协议解析 | 可拿到 `Mode: leader/follower`,但白名单仍可能禁,且解析逻辑增加维护负担 |
| C. 发送 4 字节 magic 期待 RST | 区分"是 ZK"vs"是别的服务" | 假阳性低但实现晦涩 |

选 A。`Detail` 中明示"TCP-only probe; 4lw whitelist may exclude ruok",运维有歧义可手动 `nc -zv` 与 `echo srvr | nc` 复核。

### D3: 报告 `Item` 集合 BREAKING

旧:`{redis, mysql-paas, mysql-monitor, rabbitmq, zookeeper, es7, influxdb}` 共 7 项。
新:`{redis, mysql, rabbitmq, zookeeper, es7, influxdb}` 共 6 项。

下游消费者(若存在)需更新。无双写兼容,理由与 bkmonitorv3-role-per-host change 一致:无外部契约,双写反而模糊语义。

## Risks / Trade-offs

- [ZK TCP 探活会把"端口已监听但 ZK 进程僵死/选举失败"的场景误判为 ok] → 接受。判活粒度本就是 ZK 自身要解决的问题;巡检定位为"连通性"。后续如果想细化,可在新 change 中加 srvr 协议解析。
- [MySQL 默认 `mysql.service.consul` 在没跑 Consul 的极少数环境会 DNS 解析失败] → 这种场景下用户应显式设置 `BK_MONITOR_MYSQL_HOST`。返回 `unreachable` 详细到 mysql CLI 的报错即可。
- [合并 MySQL 后,只显示 1 条结果,失去了"PaaS service 自己用什么 MySQL"的辨识度] → 接受。当前架构这两个 service 复用同库是事实,辨识度无价值。
