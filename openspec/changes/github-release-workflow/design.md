## Context

`weops-inspect` 是一个 Go 1.25.1 的单 binary 巡检工具，目前没有自动化分发流程。最近的提交 `6d8a9f0` 已把 MySQL 客户端切换为原生驱动，整个项目可在 `CGO_ENABLED=0` 下构建，这为单 runner 矩阵交叉编译扫清了障碍。维护者希望保留对发版节奏的人工控制，因此触发方式必须是手动而非 tag push 自动触发。

## Goals / Non-Goals

**Goals:**
- 单一 workflow 文件覆盖 amd64 + arm64 的 Linux 二进制构建。
- 维护者在 GitHub Actions UI 选择 tag 即可发版，整个流程对 secrets 零依赖（仅用默认 `GITHUB_TOKEN`）。
- 同一 tag 可重跑，行为是覆盖式发布。
- 二进制可通过 `-v` 自描述其版本。

**Non-Goals:**
- 不构建 Windows、macOS 或其它非 Linux 目标。
- 不生成压缩包、checksum、SBOM 或签名（如未来需要再追加）。
- 不在 push tag 时自动发布；不集成 changelog 自动生成。
- 不发布到非 GitHub 渠道（Docker Hub、对象存储等）。

## Decisions

### 触发方式：`workflow_dispatch` + 必填 `tag` 输入
- **选**：仅 `workflow_dispatch`，输入字段 `tag`（必填，字符串）。
- **拒**：`push: tags: ['v*']`。原因：用户显式要求"手动选择 tag 构建"，避免推送 tag 即发布带来的误触。
- **拒**：让 workflow 创建 tag。原因：要求是"已存在的 tag"，工作流职责只做构建+发布，不承担打 tag。

### 构建策略：纯 Go 矩阵交叉编译
- **选**：单 `ubuntu-latest` runner，`strategy.matrix.goarch: [amd64, arm64]`，`CGO_ENABLED=0` + `GOOS=linux GOARCH=$goarch` 直接 `go build`。
- **拒**：使用 `ubuntu-24.04-arm` 原生 arm64 runner。原因：纯 Go 不需要原生指令集，交叉编译更简单且 runner 一致；如果未来重新引入 CGO 再切换。
- **拒**：`goreleaser`。原因：当前需求简单，工作流不到 60 行就能搞定，引入额外工具反而增加学习成本与配置面。

### 版本注入：`-ldflags "-X main.version=<tag> -s -w"` + `-trimpath`
- `-X main.version` 把 tag 字符串写入 `var version string = "dev"`。
- `-s -w` 去掉符号表与 DWARF，缩小体积。
- `-trimpath` 抹掉本机路径，构建可重现性更好。

### `-v` flag 实现：复用 `flag` 包
- `main.go` 当前已用标准库 `flag`。新增 `flag.Bool("v", ...)` 与 `flag.Bool("version", ...)`，命中后 `fmt.Println(version); return`。
- 选标准库而非引入 `cobra`/`pflag`：项目其他地方未使用，最小侵入。

### Release 上传：`gh release upload --clobber`
- **选**：`gh` CLI（runner 自带）。`gh release view` 探测，不存在则 `gh release create`，再用 `gh release upload "$tag" file1 file2 --clobber` 完成覆盖语义。
- **拒**：`softprops/action-gh-release` 等第三方 action。原因：`gh` 是官方工具、零额外依赖、`--clobber` 语义直接命中"重跑同一 tag 覆盖"的需求。

### 任务拓扑：build job 矩阵 + release job 汇聚
```
        ┌─ build (amd64) ─┐
trigger ─┤                 ├─► release (download artifacts → gh upload)
        └─ build (arm64) ─┘
```
- build job 用 `actions/upload-artifact` 把二进制传给 release job。
- release job `needs: build`，`actions/download-artifact` 拉回，再统一上传。
- 任一 build job 失败即整体失败，不会发布部分产物。

### 权限
- workflow 顶层声明 `permissions: contents: write`（其它默认 read），最小化默认 `GITHUB_TOKEN` 权限面。

## Risks / Trade-offs

- **风险**：未来若重新引入 CGO 依赖，arm64 交叉编译会断。**缓解**：在 design.md 与 workflow 注释里标注 `CGO_ENABLED=0` 是前提；若改动需重新评估，可换成 arm64 native runner。
- **风险**：`gh release create` 在并发重跑时可能竞争创建。**缓解**：用 `concurrency: group: release-${{ inputs.tag }}, cancel-in-progress: false` 串行化同 tag 的执行。
- **权衡**：不打 tar.gz、不出 checksum，简洁但用户下载后无法独立校验完整性。**判断**：当前发布频率与受众都很低，YAGNI；后续有需要再补。
- **权衡**：版本号注入到 `main.version` 而非更结构化的 build info（`runtime/debug.ReadBuildInfo`）。**判断**：标准库 `runtime/debug` 在用 `go install` 时能拿到 vcs 信息，但 `go build` 默认不带，注入更确定。
