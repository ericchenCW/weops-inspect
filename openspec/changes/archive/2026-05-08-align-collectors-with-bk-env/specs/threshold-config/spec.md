## ADDED Requirements

### Requirement: 阈值默认值

系统 SHALL 在 env 未设置时使用以下阈值默认值:

- CPU 使用率:75
- 磁盘使用率:75
- inode 使用率:75
- 内存使用率:75
- 最大文件句柄数:102400
- 主机运行天数:365

#### Scenario: 全部使用默认值

- **WHEN** 所有 `INSPECT_*_THRESHOLD` / `INSPECT_MAX_OPEN_FILES` / `INSPECT_RUN_DAYS` 都未设置
- **THEN** `Config.Thresholds` 各字段等于上述默认值

### Requirement: env 覆盖阈值

系统 SHALL 允许通过下列 env 覆盖阈值:

- `INSPECT_CPU_THRESHOLD` → CPU 使用率
- `INSPECT_DISK_THRESHOLD` → 磁盘使用率
- `INSPECT_INODE_THRESHOLD` → inode 使用率
- `INSPECT_MEM_THRESHOLD` → 内存使用率
- `INSPECT_MAX_OPEN_FILES` → 最大文件句柄数
- `INSPECT_RUN_DAYS` → 主机运行天数

#### Scenario: 覆盖 CPU 阈值

- **WHEN** `INSPECT_CPU_THRESHOLD=85`
- **THEN** `Config.Thresholds.CPUUsage` 等于 `85`

### Requirement: 阈值解析失败硬退出

系统 SHALL 在 env 提供了非法数字时(如 `INSPECT_CPU_THRESHOLD=abc`)立即返回错误,使主进程退出,不静默退回默认值。

#### Scenario: 非法数字

- **WHEN** `INSPECT_DISK_THRESHOLD=not-a-number`
- **THEN** `Config.Load()` 返回错误,错误信息包含变量名 `INSPECT_DISK_THRESHOLD`
