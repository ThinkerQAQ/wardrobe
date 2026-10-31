[English](SECURITY.md) · [首页](README_ZH.md)

# 安全与隐私说明

Wardrobe 是运行在用户本机的单用户工具，**不是可直接公网部署的多用户服务**。

## 报告安全问题

请勿在公开 Issue、Pull Request、截图或日志中发送 Cookie、令牌、订单、收货地址或个人尺寸数据。

如发现可复现的安全问题，请先通过 `contact@thinkerqaq.com` 私下联系维护者，提供脱敏复现步骤、受影响版本和预期影响。

## 使用时注意

- 默认仅使用 `127.0.0.1` 本机地址，不要通过端口映射、反向代理或隧道将 Web 管理界面公开到公网。
- `Data/mcp-token` 是本地 MCP 凭据，严禁上传 Git、粘贴到 Issue 或分享给不可信应用。
- Web UI 和 MCP 的访问边界不同：MCP 有 Bearer Token；Web UI 主要依赖本机访问边界。
- 即使 SQLite 文件只在本地，经 MCP 传给云端 AI 的字段与图片仍可能离开本机，请按需提供。
- 定期备份整个 `Data/`；复制正在使用的 SQLite 文件前应关闭应用，避免遗漏 WAL。
- 淘宝、京东导出文件包含敏感购买记录；请保留在本地私有目录。不要安装来源不明的订单导出扩展。
- 真实商品图片、平台内容和第三方扩展不随 MIT 源码许可证自动获得再分发授权。

本仓库只提供源码和不含个人信息的演示数据。

## 安装器与凭据

仅从 ThinkerQAQ/wardrobe 与 openai/tunnel-client 官方 GitHub Releases 下载，校验 SHA256SUMS.txt。Tunnel ID 与受限运行密钥属于每个用户，不能共享、提交 Git、写入公开日志或发到聊天。Windows 使用本机当前用户 DPAPI 加密存放；Linux/macOS 为权限 0600 的用户私有文件。开源发布包不包含本地 SQLite、真人照片、购买记录、Tunnel Key。仅可启用 MCP Tunnel，不可把完整本地 HTTP 管理服务转发公网。

## 依赖安全与 CI

仓库已启用 Dependabot 漏洞告警及安全更新；CI 使用 `govulncheck` 检查可达的 Go 漏洞调用链。`main` 必须通过 `test` 和 `windows-installer`，且强制线性历史、解决 PR 讨论和管理员同等受保护；仅允许 Squash Merge，并自动删除已合并分支。发布二进制必须使用已包含安全补丁的 Go 工具链。

## 本机侧边栏与订单采集安全

Wardrobe 的 Edge/Chrome 侧边栏嵌入本机 Go Web UI（`/?sidepanel=1`），因此在 iframe 内通过同源接口访问本地 SQLite，不需要用户手动输入配对码。普通网页、API 和其他路径保持禁止嵌入；只有专用入口允许扩展来源的 iframe。扩展父页面受浏览器同源策略限制，不能直接读取内嵌应用的 DOM。订单采集须由用户主动开启，只转发白名单归一化商品字段，不读取 Cookie 或完整响应。旧版带密钥采集接口仅保留兼容，不属于新侧边栏流程。不得将本机 HTTP 端口公开代理到互联网。
