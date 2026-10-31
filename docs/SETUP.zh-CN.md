[English](SETUP.md) · [首页](../README_ZH.md)

# Wardrobe 一键安装与配置向导

Wardrobe 是 Go + SQLite 本地衣橱，**不需要 Node/npm、Go 开发环境或 Edge 调试**。OpenAI Secure MCP Tunnel 仅用于把用户本人授权的本地 MCP 连接到 ChatGPT，不开放完整 Web UI。

## Windows：解压，双击

1. 前往 [Wardrobe 最新 Release](https://github.com/ThinkerQAQ/wardrobe/releases/latest)，下载 wardrobe-windows-amd64.zip（普通 PC）或 wardrobe-windows-arm64.zip（ARM PC）。
2. 解压后双击 **Setup-Wardrobe.cmd**。不需要管理员权限。
3. 自动安装在当前账户的 %LOCALAPPDATA%\Wardrobe，创建本地 SQLite 配置，启动 Web UI 并设置用户登录启动。
4. 如果选择连接 ChatGPT：安装器会下载官方 tunnel-client 并校验 SHA256；您需在 Platform 的 Tunnel 页面创建/选择自己的 tunnel_id，在运行密钥页面创建 restricted runtime API key，并在向导中输入。密钥隐藏输入，并经 Windows DPAPI 加密保存，不写入公开配置。
5. 向导校验并启动 Tunnel，最后指导在 ChatGPT Plugins → + → Add custom MCP server → Tunnel 中选择自己的 Tunnel 并授权。**插件授权仍需要账户本人操作。**

本地网页： http://127.0.0.1:17936 。重复运行向导不会覆盖 Data 中的个人数据库/照片/配置。可跳过 Tunnel，先仅使用本地 Web UI；之后再运行一次即可配置。

如果从 GitHub 克隆源代码且本机无二进制文件，也可双击 Setup-Wardrobe.cmd，向导会从最新正式 Release 下载并校验程序，不要求安装 Go。更新时若二进制被旧进程锁定，请先关闭旧程序再运行。

## Linux / macOS：一个命令

下载并解压与系统对应的 wardrobe-linux-amd64.tar.gz、wardrobe-linux-arm64.tar.gz、wardrobe-darwin-amd64.tar.gz 或 wardrobe-darwin-arm64.tar.gz，在解压目录运行：

    bash scripts/setup.sh

向导将应用安装到 ~/.local/share/wardrobe。Linux（有 systemd --user）注册当前用户服务；macOS 在有图形登录会话时注册 LaunchAgent。无可用服务管理器时会提示手动启动命令，不宣称已经后台常驻。受限 Tunnel Key 存放在本地文件，权限限制为 0600；使用高安全等级设备的用户应额外保护操作系统登录账户。macOS 二进制暂未签名公证；系统信任决定由用户自行作出，安装器不绕过系统安全设置。

## 用户必须授权的部分

- [OpenAI Platform Tunnels](https://platform.openai.com/settings/organization/tunnels)：创建或复用自己的 tunnel_id，确保与目标 ChatGPT 工作空间正确关联。
- [OpenAI Platform Keys](https://platform.openai.com/settings/organization/api-keys)：运行密钥需 Tunnels Read + Use；创建/管理 Tunnel 需 Tunnels Read + Manage。不要使用 Admin Key 来长期运行客户端。
- ChatGPT Web → Plugins → + → Add custom MCP server → Tunnel：用户检查工具权限，并确认创建。账户/工作空间的功能权限由 OpenAI 控制，安装器无法越过。

**每个用户用自己的 Tunnel，不能共享维护者的私人 Tunnel ID。** 只用本地 Web UI 不会把资料上传云端；用户主动通过 ChatGPT MCP 查询时，选择传输的商品字段/图片会进入对应 AI 服务。

## 隐私与安全

- Release 中只包含源码构建程序、安装器、无敏感数据的配置模板和文档。
- 应用与官方 tunnel-client 压缩包通过发布方 SHA256SUMS.txt 校验，来源分别是 ThinkerQAQ/wardrobe 与 openai/tunnel-client。
- 本地数据库、照片、购买订单、个人 API Key、密钥和配置不能提交 Git；本地 HTTP 管理端口禁止公开反向代理。
- 重装不得覆盖 Data；如果从以前的安装位置迁移，请先停止使用数据库的程序，完整迁移 Data 目录（含 WAL 和图片），不能在正在运行时只复制单独的 SQLite 主文件。
- Windows DPAPI 凭据绑定当前 Windows 用户/设备。Linux/macOS 本地密钥以权限 0600 保存，必须保护用户账户和备份。
- 当前用户登录启动不需要管理员权限，也不会自动连接电商账号。

## 开发与离线回归测试

Windows：

    powershell -NoProfile -File scripts/Setup-Wardrobe.ps1 -NoTunnel -NoStart -NoAutostart -NonInteractive -WardrobeBinary C:\path\to\wardrobe.exe -InstallDir "$env:TEMP\WardrobeSmoke"

Linux/macOS：

    bash scripts/setup.sh --no-tunnel --no-start --non-interactive --binary /path/to/wardrobe --install-dir /tmp/wardrobe-smoke

## 故障排查

- 本地页面无法连接：检查 127.0.0.1:17936 是否冲突；运行已安装目录的 scripts/Start-Wardrobe-Local.ps1 或 scripts/start-local.sh。
- Tunnel doctor 失败：检查 tunnel_id、受限密钥、所选组织与 Workspace 的关联、Tunnels 权限、网络访问，然后重新运行向导；不要删除数据库。
- ChatGPT 未显示工具：确认 Tunnel 运行，检查插件权限并刷新 MCP 工具清单，再测试 search_wardrobe。
- Windows SmartScreen 或 macOS Gatekeeper：校验 Release 的来源与 SHA256，系统保护提示需用户自行判断，安装器不会关闭系统安全功能。

官方依据：[Secure MCP Tunnel](https://developers.openai.com/api/docs/guides/secure-mcp-tunnels) 和 [tunnel-client](https://github.com/openai/tunnel-client)。
