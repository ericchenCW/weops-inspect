## ADDED Requirements

### Requirement: 版本号变量
`main` 包 SHALL 声明一个可被链接器覆盖的字符串变量 `version`，默认值为 `dev`，构建时通过 `-ldflags "-X main.version=<tag>"` 注入实际版本。

#### Scenario: 未注入版本时使用默认值
- **WHEN** 二进制由 `go build` 直接构建（无 ldflags 注入）
- **THEN** `version` 取默认值 `dev`

#### Scenario: 通过 ldflags 注入
- **WHEN** 构建命令带 `-ldflags "-X main.version=v1.2.3"`
- **THEN** 运行时 `version` 的值为 `v1.2.3`

### Requirement: 版本打印 flag
二进制 SHALL 支持 `-v` 与 `--version` 命令行 flag，命中时打印当前 `version` 值并以退出码 `0` 退出，不执行任何巡检逻辑。

#### Scenario: 短 flag 打印版本
- **WHEN** 用户运行 `weops-inspect -v`
- **THEN** 标准输出打印 `version` 字符串后程序退出，退出码为 `0`

#### Scenario: 长 flag 打印版本
- **WHEN** 用户运行 `weops-inspect --version`
- **THEN** 行为与 `-v` 相同

#### Scenario: 不影响默认运行
- **WHEN** 用户运行 `weops-inspect` 不带 `-v` / `--version`
- **THEN** 程序按既有逻辑加载配置并执行巡检，行为不变
