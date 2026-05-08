## 1. 数据结构与配置

- [x] 1.1 在 [model/types.go](model/types.go) 的 `DiskUsage` 结构中新增 `FsType string \`json:"fs_type"\`` 字段
- [x] 1.2 在 [config/config.go](config/config.go) 的 `Config` 结构新增 `DiskIncludeNFS bool` 字段, 由 `INSPECT_DISK_INCLUDE_NFS` 环境变量解析, 默认 `false`
- [x] 1.3 调整 `CHECK_MOUNT_PATH` 默认值: 从 `/data` 改为空串(空串语义=走默认全采筛选)

## 2. 文件系统类型筛选

- [x] 2.1 在 [collector/host.go](collector/host.go) 增加常量: `diskFsBlocklist`(map[string]bool, 包含 tmpfs/devtmpfs/overlay/squashfs/shm/proc/sysfs/cgroup/cgroup2/autofs/binfmt_misc/mqueue/pstore/debugfs/tracefs/ramfs/rpc_pipefs/fusectl/configfs/securityfs/hugetlbfs/fuse.lxcfs/fuse.gvfsd-fuse)
- [x] 2.2 增加常量: `diskFsAllowlist`(xfs/ext2/ext3/ext4/btrfs/zfs/f2fs/ufs/jfs/reiserfs, 实际实现还包含 vfat 以监控 /boot/efi)
- [x] 2.3 增加常量: `diskFsNFS`(nfs/nfs4/cifs/smbfs/smb3)
- [x] 2.4 实现 `shouldCollectFs(fsType string, includeNFS bool) bool`, 实现"黑名单 > NFS 开关 > 白名单"优先级

## 3. 远端命令与 parser 改造

- [x] 3.1 把 `hostBatchCmd` 中 `df -h` 改为 `df -ThP`, `df -i` 改为 `df -iPT`
- [x] 3.2 重写 `parseDiskUsage(output, mountPaths string, includeNFS bool, ...)`: 跳过表头, 按 7 列定位(Filesystem/Type/Size/Used/Avail/Use%/MountedOn), 填充 `FsType`
- [x] 3.3 实现"显式列表"分支: 当 `mountPaths` 非空, 仅完全相等命中的挂载进入候选, 但仍受黑名单过滤
- [x] 3.4 实现"默认全采"分支: 当 `mountPaths` 为空, 通过 `shouldCollectFs` 筛选
- [x] 3.5 inode 复用同一份 parser(传同一组开关), 输出到 `m.InodeUsage`

## 4. Warning 输出

- [x] 4.1 `parseHostOutput` 中, 在 disk 解析完成后, 若 `DF` 段非空但 `m.DiskUsage` 长度为 0, 拼接 warning 字符串并 append 到 `m.Error`(若已有 SSH error 则跳过)
- [x] 4.2 warning 格式: `disk: configured mount paths [...] did not match any of [...]`(显式配置场景)或 `disk: no real filesystem matched (set INSPECT_DISK_INCLUDE_NFS=true if NFS only?)`(默认场景且全部被过滤)
- [x] 4.3 校验 `Error` 字段在 SSH 失败场景下仍以 `SSH error:` 开头, 不被 warning 干扰 (SSH 失败在 CollectAllHosts 内提前 short-circuit, 永不进入 parseHostOutput, 天然隔离)

## 5. 测试与验证

- [x] 5.1 增加 parser 单元测试: 使用 BUILD60 真实 `df -ThP` 输出 fixture, 验证默认筛选下只剩 `/`, `/boot`, `/boot/efi`
- [x] 5.2 增加 parser 单元测试: 验证显式 `CHECK_MOUNT_PATH=/data` 在 BUILD60 fixture 下返回空数组并产生 warning
- [x] 5.3 增加 parser 单元测试: 验证 `INSPECT_DISK_INCLUDE_NFS=true` 时 NFS 行被纳入
- [x] 5.4 增加 parser 单元测试: 验证长 LVM 设备名(`/dev/mapper/very-long-volume-group-name-root`)被正确解析(POSIX 单行)
- [x] 5.5 增加 parser 单元测试: 验证 inode 输出共用 `FsType` 字段
- [ ] 5.6 在一台真实 BUILD60 类型主机上手工跑一遍, 确认报告 JSON 包含 `/` 的 98% 使用并触发告警 (留待用户在真机上验证)

## 6. 文档与 release notes

- [x] 6.1 更新 [README.md](README.md) 中 `CHECK_MOUNT_PATH` 与新增 `INSPECT_DISK_INCLUDE_NFS` 的说明
- [x] 6.2 在 README 的 "Breaking changes" 段落记录: 留空 `CHECK_MOUNT_PATH` 的部署升级后会采更多挂载, 阈值告警面会扩大
- [ ] 6.3 校对 [openspec/specs/host-metrics-collection/spec.md](openspec/specs/host-metrics-collection/spec.md) 在 archive 后正确合并新增的 4 条 requirement (留待 /opsx:archive 阶段执行)
