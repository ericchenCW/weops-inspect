## ADDED Requirements

### Requirement: SSH 用户与端口

系统 SHALL 支持通过 env 配置 SSH 用户与端口:

- `INSPECT_SSH_USER` → SSH 用户名,默认 `root`(已有)
- `INSPECT_SSH_PORT` → SSH 端口,默认 `22`

#### Scenario: 默认端口

- **WHEN** `INSPECT_SSH_PORT` 未设置
- **THEN** SSH 客户端连接时使用端口 `22`

#### Scenario: 自定义端口

- **WHEN** `INSPECT_SSH_PORT=2222`
- **THEN** SSH 客户端连接时使用端口 `2222`

### Requirement: SSH 私钥路径

系统 SHALL 支持通过 `INSPECT_SSH_KEY_PATH` 指定私钥文件路径;未设置时回落到当前默认行为(SSH agent / 默认 `~/.ssh/id_rsa`)。

#### Scenario: 指定私钥

- **WHEN** `INSPECT_SSH_KEY_PATH=/data/keys/inspect.pem`
- **THEN** SSH 客户端使用 `/data/keys/inspect.pem` 作为私钥

#### Scenario: 未指定私钥

- **WHEN** `INSPECT_SSH_KEY_PATH` 未设置
- **THEN** SSH 客户端使用现有默认认证策略

### Requirement: NOPASSWD sudo 包装

系统 SHALL 支持通过 `INSPECT_SSH_USE_SUDO=true` 在每条远程命令前加 `sudo `,且仅适用于目标主机已配置 NOPASSWD sudo 的场景。系统 SHALL NOT 接受 sudo 密码注入或交互式密码输入。

#### Scenario: 启用 sudo

- **WHEN** `INSPECT_SSH_USE_SUDO=true` 且远程命令为 `cat /proc/loadavg`
- **THEN** 实际执行的命令为 `sudo cat /proc/loadavg`

#### Scenario: 默认不启用 sudo

- **WHEN** `INSPECT_SSH_USE_SUDO` 未设置
- **THEN** 实际执行命令不包含 `sudo` 前缀
