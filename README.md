# weops-inspect

蓝鲸 (BlueKing) 运维平台一次性巡检 CLI。运行后对所有蓝鲸模块所在主机、蓝鲸服务
进程、以及周边开源中间件做一次全量健康采集，输出 `weops_inspection.html` 与
`weops_inspection.json` 报告。

```
weops-inspect -o /var/log/weops
```

配置以 `BK_*` / `INSPECT_*` 环境变量为准（与 `bk.env` 对齐）。

## 磁盘采集配置

主机磁盘 / inode 使用率通过远端 `df -ThP` / `df -iPT` 采集，按以下规则筛选要纳入
告警判断的挂载点：

| `CHECK_MOUNT_PATH`     | 行为                                                                 |
|-----------------------|----------------------------------------------------------------------|
| 留空（默认）          | 自动按文件系统类型采集所有"真实"磁盘（xfs/ext4/btrfs/zfs 等）       |
| `/data:/var:/home`    | 仅采集冒号分隔列表中**完全相等**的挂载点（不做前缀匹配）            |

无论哪种模式，伪文件系统（`tmpfs` / `devtmpfs` / `overlay` / `squashfs` / `shm` /
`proc` / `sysfs` / `cgroup` / `autofs` 等）始终排除，避免容器宿主机被淹没。

NFS / SMB（`nfs` / `nfs4` / `cifs` / `smbfs` / `smb3`）默认不采，需要时显式启用：

```sh
INSPECT_DISK_INCLUDE_NFS=true
```

> **Breaking change（v? 起）**：旧版本默认 `CHECK_MOUNT_PATH=/data`，未配置 `/data`
> 挂载的主机会静默漏采（最典型：LVM 把数据合并到 `/`）。新默认改为"采所有真实磁盘"，
> 升级后阈值告警面会扩大；想保持原行为请显式 `CHECK_MOUNT_PATH=/data`。
>
> 当 `df` 输出非空但筛选结果为 0 条时，主机条目的 `error` 字段会追加
> `disk: configured mount paths [...] did not match any of [...]` 形式的 warning，
> 便于发现配置和实际挂载不匹配的情况。

## 邮件告警通知

巡检结束后可选地把告警情况通过邮件推送给运维。配置文件位置：

```
~/.config/weops/config.json
```

可由环境变量 `WEOPS_CONFIG=/path/to/config.json` 覆盖。

### 配置示例

```json
{
  "email": {
    "enabled": true,
    "smtp_host": "smtp.example.com",
    "smtp_port": 465,
    "smtp_use_tls": true,
    "username": "alert@example.com",
    "password": "your-smtp-password",
    "from": "WeOps 巡检 <alert@example.com>",
    "to": ["ops@example.com", "dba@example.com"]
  },
  "trigger": {
    "min_interval_minutes": 120,
    "send_recovery": true
  }
}
```

字段说明：

- `email.smtp_use_tls`：`true` 走隐式 SSL/TLS（常见 465 端口），`false` 走明文/
  STARTTLS（常见 25/587 端口）。
- `trigger.min_interval_minutes`：同一组告警的去重窗口（默认 120）。窗口内同签名
  告警会被抑制；告警集合发生变化（新增/消失某项）时立即重发，不受窗口限制。
- `trigger.send_recovery`：上次发过告警邮件、本次恢复正常时是否发送恢复通知。

**安全提醒**：配置文件含明文密码，必须收紧权限：

```
chmod 600 ~/.config/weops/config.json
```

启动时若文件权限不是 0600，工具会在 stderr 打印 warning 提示。

### 通知行为

| 上次状态  | 本次告警数 | 条件                           | 动作       |
|-----------|-----------|--------------------------------|-----------|
| 空 / ok   | 0         | —                              | 不发      |
| 空 / ok   | >0        | —                              | 发告警    |
| alert     | 0         | —                              | 发恢复    |
| alert     | >0        | 签名相同 且 距上次 < 冷却窗口  | 抑制      |
| alert     | >0        | 签名变化（告警集合变了）        | 立即重发  |
| alert     | >0        | 签名相同 且 距上次 ≥ 冷却窗口  | 重发      |

发送状态保存在 `~/.config/weops/state.json`（与 config 同目录）。**仅在发送成功
后**写入；发送失败保留旧基线，下次按旧状态判定，避免一次 SMTP 抖动让告警长期被
误抑制。

通知子系统的任何错误都只在 stderr 输出，**不影响巡检退出码**——与 cron 共存友好。

## Crontab 周期巡检部署

工具本身不会修改 crontab，请手工添加。一个常用模式是每 5 分钟跑一次：

```cron
# /etc/crontab 或 crontab -e
*/5 * * * * /bin/bash -lc 'source /data/install/bk.env && /usr/local/bin/weops-inspect -o /var/log/weops' >>/var/log/weops/weops-inspect.log 2>&1
```

要点：

1. **必须先 source `bk.env`**：crontab 默认不加载用户 shell profile，`BK_*`
   环境变量不会自动可用。`bash -lc` 进入登录 shell 并显式 source。
2. **stderr 重定向**：工具的所有日志都走 stderr，重定向到日志文件便于排查。
3. **WEOPS_CONFIG 覆盖**（可选）：若想用非默认路径的配置文件：
   ```cron
   */5 * * * * WEOPS_CONFIG=/etc/weops/config.json /bin/bash -lc 'source /data/install/bk.env && /usr/local/bin/weops-inspect -o /var/log/weops' >>/var/log/weops/weops-inspect.log 2>&1
   ```
4. **冷却窗口与执行频率**：`min_interval_minutes` 应大于等于 cron 间隔的 1 倍，
   否则去重失效。每 5 分钟跑一次配合默认 120 分钟窗口是常见组合。
