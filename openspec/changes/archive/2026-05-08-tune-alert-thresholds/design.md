## Context

巡检工具当前的告警阈值与服务检查策略写死在 `config/config.go` 与 `collector/service_registry.go` 中。生产实践中暴露三个问题:

1. **阈值过严**: 默认 75% 触发的告警在多数环境中是噪声, 用户已经通过 env 在各处覆盖, 不如直接抬高默认值
2. **服务检查没有 per-service 跳过开关**: 例如 job-analysis 在某些部署形态下不应纳入 healthz/status 范围, 但 registry 里没有跳过位
3. **健康检查端点假设单一端口**: `service_registry.ServiceSpec` 只有一个 `Port` 字段, 同时用于业务监听和 healthz 探测; Spring Cloud Gateway 这类网关会把 actuator 拆到独立 management 端口, 当前模型无法表达, 导致 job-gateway 永远误报 fail

约束:

- 现有 9 个 env-overridable 阈值已经被生产配置依赖, 改默认值会改变未显式配置用户的告警面
- `ServiceSpec` 是值类型 struct(非 pointer), 新增字段对零值兼容很重要——已有的 service entry 无需逐条改写
- RabbitMQ 检查代码尚未读 spec, 但黑名单是单点逻辑, 在 0 消费者判定处一个 set 检查即可

## Goals / Non-Goals

**Goals:**
- 抬高默认阈值, 减少噪声告警, 同时保留 env 覆盖
- 让 ServiceSpec 能表达"healthz 走独立端口"
- 让 ServiceSpec 能表达"按服务跳过 status / healthz 检查"
- 修复 job-gateway healthz 误报
- RabbitMQ 0 消费者按 vhost 黑名单过滤(默认放过 bk_bknodeman)

**Non-Goals:**
- 不展开 Spring Boot Actuator components 细节(展开 healthz fail 的根因到字段级)——独立改进项, 不在本轮
- 不把状态型规则(SELinux/Firewalld/Chronyd/load average)配置化——本轮不动
- 不引入 healthz scheme(http/https)字段——19876 是 http, 当前不需要
- 不改 collector 拼命令的整体结构, 只在最小处插入跳过逻辑

## Decisions

### D1. 默认阈值统一抬到 95%, MaxOpenFiles 抬到 65536

**选择**: CPU/Mem/Disk/Inode 默认 → 95; MaxOpenFiles 默认 → 65536(`< 65536` 告警)。

**理由**: 95% 是用户实际生产经验值; 65535 是 Linux 单进程默认 soft limit 的常见警戒线, 含义"必须 ≥ 65536"。判定逻辑 `< threshold` 不变, 只调阈值——避免改判定逻辑。

**替代**: 调到 90%——用户明确说 95%, 不另行决策。

### D2. RunDays 直接移除而非保留为可选

**选择**: 同时删除 `Thresholds.RunDays` 字段、`INSPECT_RUN_DAYS` env 解析、`checker/rules.go` 判定块。`HostMetrics.RunDays` 采集字段保留(它仍出现在采集结果中), 只是不再产生告警。

**理由**: 用户决策"不做告警"; 采集字段保留是因为它已被报告 render 使用且无害, 砍它会牵涉更多代码。

**替代**: 保留 env 但默认值设极大(如 999999)——会让代码仍带一个事实上无用的检查项, 不如直接删干净。

### D3. ServiceSpec 加 3 个新字段, 而不是引入 service-skip 配置文件

**选择**: 在 `collector.ServiceSpec` 上加 `HealthzPort int`, `SkipStatusCheck bool`, `SkipHealthzCheck bool`。零值即默认行为。

**理由**: registry 本来就是声明式 struct 列表, 加字段最直接; 三个字段都对零值兼容, 已有 entry 无需改动。如果走外部配置文件, 还要解析 + 合并, 复杂度不成比例。

**替代**: 在 `Thresholds` 里维护 `SkipServices []string`——两类配置混在一起, 且无法表达"只跳 healthz 不跳 status"这种细分。

### D4. RabbitMQ 0 消费者黑名单走 env, 默认含 bk_bknodeman

**选择**: 新增 `INSPECT_RABBITMQ_NO_CONSUMER_VHOST_BLACKLIST`(逗号分隔), 默认 `bk_bknodeman`。env 设置时**完全替换**默认值(不是合并)。仅作用于 0 消费者判定; 队列堆积阈值与 vhost 无关。

**理由**: 与现有 9 个阈值同源, 都走 env, 风格一致。完全替换语义比合并简单且可预测——用户写 `""` 即可放过所有 vhost(无黑名单), 写 `foo,bar` 即可彻底替换默认。

**替代**: 用合并语义 + `RABBITMQ_NO_CONSUMER_VHOST_EXTRA` 风格——多一个 env, 复杂度无收益。

### D5. ServiceSpec 拼 healthz URL 处兜底, 不改 systemctl/进程逻辑

**选择**: 仅在 [collector/service.go:74-87](collector/service.go:74) 拼 healthz URL 的几行处:
```go
hzPort := sub.HealthzPort
if hzPort == 0 {
    hzPort = sub.Port
}
```
在 `case "http_status"/"http_alive"/"json_ok"/"json_up"` 各分支中用 `hzPort` 替代原来的 `sub.Port`。systemctl 状态、进程数采集仍走 `sub.ServiceUnit` / `sub.ProcessName`, 与 healthz 无关。

**理由**: 关注点分离, 改动最小。

### D6. SkipStatusCheck / SkipHealthzCheck 在 collector 与 checker 双侧短路

**选择**:
- collector(`collector/service.go`): 拼命令前判断 skip, skip 的项不进 `cmdParts`(不发 SSH 指令、不解析对应 section), 解析时跳过对应字段赋值
- checker(`checker/rules.go`): `CheckService` 判断 `sm.Status == ""` 视为不输出 status 检查; healthz 同理已有 `"N/A"` 跳过路径, 让 collector 在 skip healthz 时直接置 `"N/A"` 即可

**理由**: 双侧都改, 既省 SSH 流量(不发请求), 又保证 checker 不产生空检查项。

## Risks / Trade-offs

- **[阈值抬高错过真实告警]** → 95% 仍能覆盖典型容量危机; 用户可通过 env 单独压回
- **[RunDays 字段移除影响 render]** → 检查 `render/` 与 `model/` 中 RunDays 引用; 仅删 Threshold 字段与告警判定, 保留 HostMetrics.RunDays 输出, 兼容现有 render 模板
- **[ServiceSpec 新字段 zero-value 误判]** → `HealthzPort=0` 永远代表 "用 Port", 不会与合法端口冲突; 两个 skip bool 默认 false 即原行为
- **[RabbitMQ 黑名单 env 替换语义被误用]** → 文档 spec 中明确 "完全替换", 配 `""` 等于禁用黑名单; release notes 提示
- **[默认阈值变更后用户感知]** → 写入 README / release notes, 升级时显式列出

## Migration Plan

1. 升级前: 用户应记录当前所有 `INSPECT_*` env, 评估默认阈值变化的影响
2. 升级动作:
   - 部署新二进制
   - 仍需 75% 阈值的环境: 显式设 `INSPECT_CPU_THRESHOLD=75` 等 env
   - 仍需 RunDays 告警的环境: 移除 `INSPECT_RUN_DAYS` 配置(它现在不生效, 留着也无害但建议清理)
   - 不希望放过 bk_bknodeman 的环境: 设 `INSPECT_RABBITMQ_NO_CONSUMER_VHOST_BLACKLIST=""`
3. 回滚: 回退到上一版本二进制即可, 无数据迁移

## Open Questions

- 是否需要为 SkipStatusCheck / SkipHealthzCheck 提供 env 级开关(而非只能在 service_registry 里硬编码)? 当前 scope 不需要, 但可以是后续改进
