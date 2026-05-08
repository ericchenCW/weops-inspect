## ADDED Requirements

### Requirement: 手动触发的发布工作流
仓库 SHALL 提供 GitHub Actions 工作流，仅通过 `workflow_dispatch` 事件触发，并接受一个必填输入 `tag` 表示要构建的 Git tag 名（例如 `v1.2.3`）。

#### Scenario: 维护者通过 Actions UI 手动触发
- **WHEN** 维护者在 GitHub Actions 页面选择该工作流并填入已存在的 tag
- **THEN** 工作流以该 tag 对应的 commit 为构建源启动

#### Scenario: 输入的 tag 不存在
- **WHEN** 输入了仓库中不存在的 tag
- **THEN** 工作流在 checkout 步骤失败，不产出任何二进制

### Requirement: 多架构 Linux 二进制构建
工作流 SHALL 以纯 Go（`CGO_ENABLED=0`）方式交叉编译 `linux/amd64` 与 `linux/arm64` 两份二进制，构建命令 MUST 通过 `-ldflags` 注入 `-X main.version=<tag>` 以及 `-s -w`，并使用 `-trimpath`。

#### Scenario: 两架构并行构建
- **WHEN** 工作流被触发
- **THEN** amd64 与 arm64 在矩阵中并行各自完成 `go build`，产物分别命名为 `weops-inspect-linux-amd64` 与 `weops-inspect-linux-arm64`

#### Scenario: 构建失败
- **WHEN** 任一架构的 `go build` 返回非零退出码
- **THEN** 工作流失败，不发布任何资产，已成功的架构产物不上传

### Requirement: 发布到 GitHub Release
工作流 SHALL 把两份二进制作为资产上传到与输入 tag 同名的 GitHub Release。Release 不存在时 MUST 自动创建；同名资产已存在时 MUST 覆盖。

#### Scenario: Release 不存在
- **WHEN** tag 对应的 Release 不存在
- **THEN** 工作流先创建 Release（标题为 tag 名），再上传两份二进制

#### Scenario: 同 tag 重跑
- **WHEN** 已存在 Release 且包含同名资产，重新触发同一 tag 的工作流
- **THEN** 旧资产被新构建产物覆盖，Release 中只保留最新版本

### Requirement: 最小权限令牌
工作流 SHALL 使用仓库默认的 `GITHUB_TOKEN`，并显式声明 `contents: write` 权限，不引入额外的 PAT 或 secrets。

#### Scenario: 默认令牌完成发布
- **WHEN** 工作流执行到上传 Release 资产步骤
- **THEN** 使用 `GITHUB_TOKEN` 即可完成创建 Release 与上传资产，无需额外凭证
