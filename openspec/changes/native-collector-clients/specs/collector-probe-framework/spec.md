## ADDED Requirements

### Requirement: 探针接口与统一结果结构

系统 SHALL 在 `collector` 包中提供统一的 `Probe` 接口,所有组件采集器(MySQL / Redis / MongoDB / RabbitMQ / ES 等)MUST 实现该接口。每次探测必须返回 `ProbeResult`,其中至少包含 `Target`(`ip:port`)、`Latency`、`Err` 与 `ErrClass` 五个字段。

#### Scenario: 实现接口

- **WHEN** 新增组件 collector
- **THEN** 该 collector 通过实现 `Probe.Run(ctx) ProbeResult` 接入框架,无需自行实现超时/计时/错误分类

#### Scenario: 返回结果

- **WHEN** 任一 `Probe.Run` 完成
- **THEN** 返回的 `ProbeResult` 携带本次探测目标地址、耗时、原始错误以及错误类别枚举

### Requirement: 探测受 ctx 超时约束

系统 SHALL 让每次 `Probe.Run` 在 `context.Context` 控制下执行,默认超时 5 秒(可在框架内改但不暴露到配置)。MUST 保证在 ctx 截止后驱动调用立刻返回,巡检主流程不被任何僵死目标阻塞。

#### Scenario: 目标无响应

- **WHEN** 目标在 5 秒内未完成 TCP 握手或协议握手
- **THEN** `ProbeResult.ErrClass = "timeout"`,`Err` 包含 `context deadline exceeded` 信息,主流程继续探测下一个目标

#### Scenario: 正常目标

- **WHEN** 目标在超时前正常返回
- **THEN** `ProbeResult.Latency` 记录从开始到结束的耗时,`Err = nil`,`ErrClass = ""`

### Requirement: 错误分类枚举

系统 SHALL 把每个 `ProbeResult.Err` 映射到下列 `ErrorClass` 之一:`network` / `auth` / `protocol` / `timeout` / `unknown` / `""`(无错误)。collector 在生成 `ProbeResult` 时 MUST 显式打标。

#### Scenario: 网络层错误

- **WHEN** 出现 `*net.OpError`、连接被拒、DNS 解析失败等
- **THEN** `ErrClass = "network"`

#### Scenario: 鉴权失败

- **WHEN** MySQL 返回 1045、Redis 返回 `NOAUTH` / `WRONGPASS`、Mongo 返回鉴权失败
- **THEN** `ErrClass = "auth"`

#### Scenario: 协议错误

- **WHEN** 服务端返回了响应但内容不可解析,或 HTTP 状态码不是 2xx
- **THEN** `ErrClass = "protocol"`

#### Scenario: 超时

- **WHEN** ctx 截止 / 驱动报告 i/o timeout
- **THEN** `ErrClass = "timeout"`

#### Scenario: 未分类

- **WHEN** 错误不属于以上类别
- **THEN** `ErrClass = "unknown"`,原始 `Err` 文本完整保留

### Requirement: 凭据不得出现在进程参数中

当 collector 通过纯 Go 驱动连接目标(MySQL / Redis / MongoDB)时,系统 MUST 通过驱动的握手协议传递用户名与密码,SHALL NOT 把密码作为 `os/exec` 的命令行参数。

#### Scenario: MySQL 探测

- **WHEN** 巡检机执行 MySQL 探测
- **THEN** 巡检进程的 `argv` 与 `/proc/<pid>/cmdline` 中均不出现 MySQL 密码明文

#### Scenario: Redis 探测

- **WHEN** 巡检机执行 Redis 探测
- **THEN** 巡检进程的 `argv` 中不出现 `-a <密码>` 或等价形式

### Requirement: 错误信息脱敏

系统 SHALL 在生成 `ProbeResult.Err` 与日志输出时,对 DSN / URI 中的密码进行脱敏(替换为 `***`),SHALL NOT 在任何 collector 错误返回中携带明文密码。

#### Scenario: MySQL 连接失败

- **WHEN** MySQL 驱动返回包含 DSN 的错误
- **THEN** `ProbeResult.Err.Error()` 中密码被替换为 `***`

#### Scenario: Mongo URI

- **WHEN** Mongo 驱动返回包含 URI 的错误
- **THEN** URI 中 `mongodb://user:***@host` 形式输出

### Requirement: 结构化日志钩子

系统 SHALL 让框架接受一个 `Logger` 接口(至少包含 `Probe(target, name string, latency time.Duration, errClass string, err error)` 方法),每次 `Probe.Run` 结束时调用一次,允许调用方接入自有日志实现。框架 SHOULD 在未注入 `Logger` 时 fallback 到 `log.Printf` 简单输出。

#### Scenario: 注入自定义 Logger

- **WHEN** 调用方注入实现了 `Logger` 接口的对象
- **THEN** 每次探测结果以结构化键值对形式被记录,字段至少含 `name`、`target`、`latency_ms`、`err_class`、`err`

#### Scenario: 未注入 Logger

- **WHEN** 框架未被注入 Logger
- **THEN** 退回 `log.Printf` 输出,不影响巡检功能
