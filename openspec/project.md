# weops-inspect 项目上下文

## 项目定位

**weops-inspect** 是一个面向蓝鲸 (BlueKing) 运维平台的一次性巡检 CLI 工具。运行后会
对所有蓝鲸模块所在主机、蓝鲸自身服务进程、以及周边开源中间件（MySQL / Redis / MongoDB
/ Elasticsearch / RabbitMQ）做一次全量健康采集与阈值检查，最终产出一份 HTML + JSON
巡检报告。

- 一次性运行，不常驻
- 采集源：远程主机 SSH + 本地中间件 CLI / HTTP API
- 输出：`weops_inspection.html` + `weops_inspection.json`

## 技术栈

- 语言：Go (go.mod 中定义)
- 远程通道：自实现 SSH client (`ssh/`)，复用连接
- 模板渲染：Go `html/template` (`render/templates/`)
- 配置入口：环境变量（`BK_*` 系列）+ `-o` 命令行参数
- 外部依赖（运行环境需提供）：
  - `mysql`、`redis-cli`、`mongo`、`curl` CLI
  - 目标主机的 `systemctl / ps / df / free / dmidecode / ss` 等基础工具

## 目录结构与职责

```
weops-inspect/
├── main.go                    # 入口：三阶段编排 (host → service → component)
├── config/                    # BK_* 环境变量加载、阈值默认值、主机去重
├── ssh/                       # SSH 客户端封装
├── collector/                 # 各类采集器（核心目录）
│   ├── common.go              # 共享 logWriter
│   ├── host.go                # 主机指标（双采样 CPU + 批量 shell）
│   ├── service.go             # 蓝鲸子模块 systemctl/healthz/worker
│   ├── service_registry.go    # 蓝鲸模块→子模块清单（unit/process/port/healthz）
│   ├── mysql.go / redis.go / mongo.go / es.go / rabbitmq.go
├── checker/                   # 阈值规则与告警判定
├── model/                     # 报告与中间数据结构
├── render/                    # HTML 模板渲染
└── output/                    # 报告写盘 (HTML + JSON)
```

## 采集能力一览

| 能力 | 通道 | 采集对象 | 关键判定 |
|------|------|---------|---------|
| 主机巡检 | SSH 批量 shell | CPU/内存/磁盘/inode/负载/进程数/ulimit/网络/SELinux/防火墙/NTP/内核/硬件 | CPU≥75%, 磁盘≥75%, ulimit<102400, 运行天数≥365 等 |
| 蓝鲸服务巡检 | SSH | `paas/cmdb/job/gse/iam/usermgr/nodeman/appo/appt` 各子模块的 systemctl + healthz + worker 数 | ActiveState=active, healthz=ok |
| MySQL | 本地 mysql CLI | 版本、变量(max_conn/innodb/timeout)、主从角色、binlog 数 | 主从 IO/SQL 状态 |
| Redis | 本地 redis-cli | INFO、celery/monitor 队列长度 | — |
| MongoDB | 本地 mongo CLI | `rs.status()` 副本集成员健康 | — |
| Elasticsearch | curl HTTP | 集群 health + `_cat/nodes` | status=green/yellow/red |
| RabbitMQ | curl 管理 API | nodes/connections/channels/queues | mem_alarm、消息堆积、无消费者 |

## 关键设计约定

### 1. 远程命令"批处理 + 切分"模式
主机和服务采集都把多条 shell 拼成一条命令在 SSH 上一次执行，每段输出用
`===TAG===` 包裹；返回后用 `parseSections()` 切分。
**目的**：减少 SSH round-trip。新增主机/服务采集项时遵循同模式。

### 2. CPU 使用率两阶段采样
[host.go](collector/host.go) 先并发读取所有主机的 `/proc/stat`，等 5 秒，再读一次，
通过 idle/total 差值计算。这是单次巡检的硬延迟来源。

### 3. 配置全部走环境变量
没有配置文件。所有蓝鲸模块 IP 通过 `BK_<MODULE>_IP_COMMA` 注入，凭据通过
`BK_*_ADMIN_*`。`AllHosts` 字段会自动去重所有模块 IP。

### 4. "采集 + 检查"分层
collector 只产出原始指标 (`model.HostMetrics` 等)，checker 应用阈值生成
`CheckResult{Field, Value, Status}`，最终 `Summary` 仅统计 OK/Warn 两态。
新增检查项时：先扩 model 字段 → collector 填值 → checker 加规则。

### 5. 错误兜底而非 fail-fast
任一主机/中间件不可达只在对应结构体里填 `Error` 字段，整体报告继续生成。
不要在 collector 里 panic 或 return error 终止流程。

### 6. 并发模型
- 主机采集：`sync.WaitGroup` + `[]results` 按 idx 写入（无锁）
- 服务采集：`sync.WaitGroup` + `sync.Mutex` 保护 map
- 中间件采集：串行（单实例查询，无并发收益）

## 命名与风格

- 包名小写单数：`collector`, `checker`, `model`
- 导出函数动词开头：`CollectAllHosts`, `CheckHost`, `Summarize`
- 子模块注册采用 struct slice + map 索引（见 `ModuleRegistry`），便于增删
- JSON tag 全小写下划线（与历史 Python 巡检脚本字段保持兼容）
- 日志走 `collector.logWriter`（默认 stderr），不要直接 `fmt.Println`

## 已知约束 / 取舍

- DNS 解析检查在 service 采集中被简化为 `N/A`（[service.go:152](collector/service.go:152)）
- ES/MySQL/Redis/Mongo 仅采集首个 IP，多节点场景未完全覆盖
- 阈值目前硬编码在 `config.Load()`，未走环境变量
- HTML 模板与 JSON schema 形成隐式契约，字段重命名需同步两侧

## OpenSpec 使用约定

- 在本项目中提出变更时优先按"采集能力"为单位划分 capability：
  `host-collection` / `service-collection` / `mysql-collection` /
  `redis-collection` / `mongo-collection` / `es-collection` /
  `rabbitmq-collection` / `rule-checking` / `report-output`
- 命名遵循 kebab-case
- 任何修改 collector 的提案，design.md 必须说明：
  1. 是否新增 SSH 命令段 / API 调用
  2. 是否影响并发模型
  3. 是否破坏 HTML/JSON 输出契约
