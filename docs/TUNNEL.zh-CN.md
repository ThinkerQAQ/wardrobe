[English](TUNNEL.md) · [首页](../README_ZH.md)

# Wardrobe · Secure MCP Tunnel 接入

这个目录只提供 Wardrobe 的**可公开接入模板和步骤**，不包含私人 Tunnel ID、API Key、个人 SQLite 或照片。

**两个不同的开源项目：**

- 本仓库 `ThinkerQAQ/wardrobe`：衣橱 Web UI、SQLite 和 MCP 服务，MIT License。
- [openai/tunnel-client](https://github.com/openai/tunnel-client)：OpenAI 已公开的通用 Tunnel 客户端，Apache-2.0 License。本仓库不复制或重新发布其代码和二进制。

## 运行架构

```text
ChatGPT（受用户授权的 MCP 插件）
        │
OpenAI Secure MCP Tunnel
        ↑  由本机 tunnel-client 主动建立 HTTPS 出站连接
Wardrobe -mcp-stdio
        │
本地 Data/wardrobe.db + images/
```

Tunnel 本身不是 AI 数据脱敏工具；如果 ChatGPT 调用 MCP，请求参数和相应字段/图片可能进入云端 AI 处理链路。

## 推荐：一键向导

普通用户请下载 GitHub Release，运行 Windows 的 Setup-Wardrobe.cmd 或 Linux/macOS 的 bash scripts/setup.sh，省去手动编辑 YAML。详见 [一键安装](SETUP.zh-CN.md)。以下步骤供开发者或手动排障使用。

## 首次接入（手动）

1. 安装或构建 [官方 tunnel-client](https://github.com/openai/tunnel-client)，并按 `tunnel-client help quickstart` 检查可用选项。
2. 将 [无密钥模板](../examples/tunnel-client.example.yaml) 复制到**本机私有目录**，修改其中 Wardrobe 的绝对路径。保持 `mcp-stdio` 模式，避免暴露完整 Web UI。
3. 在 OpenAI Platform 为自己的私有 Tunnel 获取 `tunnel_id` 和受限的运行时 API Key。用本机受保护的环境变量 `CONTROL_PLANE_TUNNEL_ID`、`CONTROL_PLANE_API_KEY` 传入，不写入公共文件。Admin Key 不用于长期运行。
4. 在已提供凭据的本机 Shell 中检查并启动：

```sh
tunnel-client doctor --config /absolute/private/path/tunnel-client.yaml
tunnel-client run --config /absolute/private/path/tunnel-client.yaml
```

5. 在允许自定义 MCP 的 ChatGPT 工作空间中添加私有 Tunnel 连接，并**检查实际工具权限**。首次调用建议先用只读 `search_wardrobe`。
6. 新版本新增 MCP 工具后，需重启对应的本地 MCP 进程，再在 ChatGPT 中刷新或重新安装插件。客户端对工具清单可能使用缓存；当前版本服务端有 **30 个工具**。2026-10-31 的 `duplicate initialize received` 已通过 Wardrobe 内置的 **stdio 顺序重初始化适配层**修复：当连接方在同一长连接中重新发送 `initialize` 时安全关闭旧会话并启动新会话，不再因为旧会话状态报错。此方案只支持**顺序**连接发现，不保证多个客户端同时独立会话；如存在真正多客户端并发，应通过严格限制到本机的无状态 HTTP MCP 进行会话隔离。刷新后建议连续调用两次只读工具，而不只看 Tunnel 进程状态。

**现有 Tunnel 正常时应复用**，不要每次重新创建新隧道。

## Skill 生效方式

- 四套默认 Skill 随源码编译；用户通过 Web UI 修改的版本是 SQLite 覆盖记录。
- `get_wardrobe_workflow(name=...)` 返回当前有效流程；导入 Skill 附带订单核验说明。
- 审美工作流目前还会附带 SQLite 的审美 notes。云端 ChatGPT 使用时，这些资料会随 MCP 响应发送，因此要按需调用。

## 安全边界

- 禁止公开转发 Wardrobe 的 `127.0.0.1:17936` 整个 HTTP 服务，尤其是 Web UI 与 `/api/export`。
- 个人 API Key、Tunnel ID、购物订单、Cookie、SQLite、相片都不要提交到 GitHub。
- 该接入不保证普通用户无需 OpenAI Platform 权限就能创建 Tunnel；账户权限以[官方说明](https://developers.openai.com/api/docs/guides/secure-mcp-tunnels)为准。
