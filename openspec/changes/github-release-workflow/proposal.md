## Why

目前 `weops-inspect` 没有自动化的二进制分发渠道，发布版本需要手工在本地交叉编译并上传。需要一个可重复、可追溯的发布流程，让维护者通过手动选择 tag 即可产出 Linux amd64/arm64 二进制并附加到对应的 GitHub Release。

## What Changes

- 新增 GitHub Actions 工作流，通过 `workflow_dispatch` 手动触发，输入参数为已存在的 Git tag。
- 工作流在 tag 对应的 commit 上以纯 Go 方式（`CGO_ENABLED=0`）交叉编译 `linux/amd64` 与 `linux/arm64` 两份二进制。
- 编译时通过 `-ldflags -X` 注入版本号到 `main.version` 变量。
- 在 `main.go` 中新增 `version` 变量及 `-v` / `--version` flag，运行 `weops-inspect -v` 可打印版本号。
- 产物以 `weops-inspect-linux-amd64` / `weops-inspect-linux-arm64` 命名上传到对应 tag 的 GitHub Release；Release 不存在则自动创建，已存在则覆盖同名资产（重跑同一 tag 语义）。

## Capabilities

### New Capabilities
- `release-binary-distribution`: 通过 GitHub Actions 手动构建并发布 Linux 多架构二进制到 GitHub Release。
- `version-reporting`: 应用启动参数支持打印自身版本号，版本由构建期注入。

### Modified Capabilities
<!-- 无 -->

## Impact

- 新增文件：`.github/workflows/release.yml`。
- 修改文件：`main.go`（新增 `version` 变量和 `-v` flag 处理）。
- 仓库 Settings：依赖默认的 `GITHUB_TOKEN`，需具备 `contents: write` 权限以创建/更新 Release。
- 不影响运行时行为；不引入新依赖。
