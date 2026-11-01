# Wardrobe

[English](README.md)

- [1. Introduction](#1-introduction)
- [2. Quick Start](#2-quick-start)
- [3. Configuration](#3-configuration)
- [4. Optional Features](#4-optional-features)
- [5. Development](#5-development)
- [6. License](#6-license)

## 1. Introduction

买过哪些衣服，和衣柜里现在还有哪些衣服，并不是一回事。购物记录能告诉我们买过什么，却不一定知道衣服还在不在、穿着是否合身，也很难直接看出哪些单品适合一起搭配。商品图片、尺码、购买记录和日常搭配分散在不同地方，整理起来比较麻烦。

Wardrobe 把这些信息集中到本地管理。你可以整理衣服和配饰，核对购买记录、确认实际持有情况，保存尺码、照片和穿着反馈，再从已有单品中组合穿搭。平时可以使用 Web 工作台或 Edge/Chrome 侧边栏，需要 AI 辅助时再连接 MCP。

**公开仓库提供的是可以独立使用的 Wardrobe Core。** 它支持本地衣橱、订单文件导入和搭配管理，**不包含京东、淘宝的私有网页采集适配器**。安装扩展也不会自动获得电商同步能力。首次使用不需要电商账号、ChatGPT 或其他浏览器自动化工具。

## 2. Quick Start

先安装本地应用，打开工作台，再完成一次「订单 → 衣橱」的实际操作。**Web UI 不依赖 MCP 或浏览器扩展**。

### 2.1. 准备环境

需要 Windows、Linux 或 macOS 的 **x64/ARM64** 电脑，以及浏览器。正式 Release 已包含可执行文件，**日常安装不需要 Go 或 Node.js**。

如果已有 Wardrobe 数据，**保留原来的 `Data/` 目录**。不要在 Windows、WSL 之间无意建立另一份空数据库。

### 2.2. 下载与安装

1. 打开[最新 Release](https://github.com/ThinkerQAQ/wardrobe/releases/latest)，选择对应压缩包：`wardrobe-windows-amd64.zip`、`wardrobe-windows-arm64.zip`、`wardrobe-linux-amd64.tar.gz`、`wardrobe-linux-arm64.tar.gz`、`wardrobe-darwin-amd64.tar.gz` 或 `wardrobe-darwin-arm64.tar.gz`。
2. 同时下载 **`SHA256SUMS.txt`**，核对压缩包与清单中**相同文件名**的 SHA-256。Windows 在下载目录执行 `Get-FileHash .\wardrobe-windows-*.zip -Algorithm SHA256`；Linux 执行 `sha256sum ./wardrobe-linux-*.tar.gz`；macOS 执行 `shasum -a 256 ./wardrobe-darwin-*.tar.gz`。不一致时停止安装。
3. **完整解压**，确认目录中有 `wardrobe.exe` / `wardrobe`、`config.example.toml`、`scripts/` 和 `browser-extension/`。
4. 运行对应系统的安装入口：

**Windows：**在解压目录双击 `Setup-Wardrobe.cmd`；向导询问是否连接 ChatGPT Tunnel 时回答 **N**。默认安装至 `%LOCALAPPDATA%\Wardrobe`。也可以直接在解压目录的 PowerShell 中执行以下命令，跳过 Tunnel 提问：

```powershell
powershell.exe -NoProfile -ExecutionPolicy Bypass -File .\scripts\Setup-Wardrobe.ps1 -NoTunnel
```

**Linux/macOS：**在解压目录打开终端：

```bash
bash scripts/setup.sh --no-tunnel
```

默认安装到 `~/.local/share/wardrobe`。安装程序复制应用和扩展文件，在系统支持时配置本地启动，**保留已有 `Data/`**。但复制扩展文件**不等于浏览器已经安装该扩展**。

### 2.3. 打开本地工作台

访问 [http://127.0.0.1:17936](http://127.0.0.1:17936)，左侧应有「订单库」「衣橱」「发型」「身体数据」「搭配」「Skills」。全新安装时没有任何衣物是正常的。

网页打不开时，先启动**已安装目录**中的本地服务：

**Windows PowerShell：**

```powershell
powershell.exe -NoProfile -ExecutionPolicy Bypass -File "$env:LOCALAPPDATA\Wardrobe\scripts\Start-Wardrobe-Local.ps1"
```

**Linux/macOS：**

```bash
bash "$HOME/.local/share/wardrobe/scripts/start-local.sh"
```

系统没有可用的用户后台服务管理器时，保持终端运行。再访问 [http://127.0.0.1:17936/api/health](http://127.0.0.1:17936/api/health)，应返回 `{"app":"wardrobe","status":"ok"}`。**不需要配对码**。Core 关闭时扩展仍能显示离线工作台，但读取与保存衣橱数据需要本地服务。

### 2.4. 验证一条订单进入衣橱

下面是**虚构测试订单**，只建议在**全新测试数据库**中执行；已有真实数据时请跳过，改用已核实的订单。示例会保存到当前数据库，并非临时预览。

通过本机 API 导入一条测试订单：

**Windows PowerShell：**

```powershell
$body = '{"platform":"Demo","items":[{"order_id":"SYNTHETIC-001","line_id":"shirt-1","name":"Demo 衬衫","quantity":1}]}'
Invoke-RestMethod -Uri 'http://127.0.0.1:17936/api/orders/import' -Method Post -ContentType 'application/json' -Body ([Text.Encoding]::UTF8.GetBytes($body))
```

**Linux/macOS：**

```bash
curl -fsS 'http://127.0.0.1:17936/api/orders/import' \
  -H 'Content-Type: application/json' \
  --data '{"platform":"Demo","items":[{"order_id":"SYNTHETIC-001","line_id":"shirt-1","name":"Demo 衬衫","quantity":1}]}'
```

首次执行应返回 `"inserted":1`。然后：

1. 打开「**订单库**」，确认列表中有「Demo 衬衫」。
2. 进入「**衣橱**」，点击「**整理穿搭入衣橱**」。按钮在**衣橱**页面，不在订单库。
3. 确认衣橱中出现「Demo 衬衫」，持有状态仍为「**待确认**」；测试订单不能证明实际持有。
4. 用相同配置重启服务，刷新页面，确认该衣物仍在。

公开 Core 不包含京东、淘宝网页自动采集功能，因此对应的同步按钮不可用。本地订单文件可通过 API 导入，见 [Case 3](#43-case-3导入订单文件)。

### 2.5. 安装浏览器侧边栏（可选）

安装程序会把 `browser-extension/` 复制到**已安装 Wardrobe 的目录**。要在浏览器中使用侧边栏，并支持自动启动本地服务，仍需**加载扩展、获取扩展 ID，再注册 Native Messaging Host**：

1. Edge 打开 `edge://extensions`，Chrome 打开 `chrome://extensions`，开启「开发者模式」，点击「加载解压缩的扩展程序」。
2. 选择**安装目录**中的 `browser-extension/`（直接包含 `manifest.json`），不要选 ZIP 或仓库根目录。Windows 默认是 `%LOCALAPPDATA%\Wardrobe\browser-extension`；Linux/macOS 默认是 `~/.local/share/wardrobe/browser-extension`。
3. 进入扩展「**详细信息**」，复制其 **32 位扩展 ID**。
4. 用**已安装的可执行文件**注册，替换占位符：

**Windows PowerShell：**

```powershell
Set-Location "$env:LOCALAPPDATA\Wardrobe"
.\wardrobe.exe --register-native-host YOUR_EXTENSION_ID
```

**Linux/macOS：**

```bash
"$HOME/.local/share/wardrobe/wardrobe" --register-native-host YOUR_EXTENSION_ID
```

5. 在扩展管理页「**重新加载**」，点击工具栏中的 Wardrobe 图标。侧边栏应显示与 Web UI 相同的工作台。如果本地服务没启动，仍会显示离线界面，并通过 Native Host 尝试「**启动本地服务**」；连接成功后展示真实衣橱数据。

必须先加载扩展，是因为**扩展 ID 只有加载后才能确定**。无需手动输入配对码或 API Token。移动安装位置或更换扩展 ID 后须重新注册；如果 Host 不可用，可按 [2.3](#23-打开本地工作台) 手动启动 Core。

### 2.6. 常见问题

- **网页打不开：**按 [2.3](#23-打开本地工作台) 启动服务，检查 `/api/health` 和 `17936` 端口是否被其他进程占用。
- **侧边栏显示离线：**先检查是否加载了**安装目录**中的扩展、ID 是否正确、Native Host 是否注册，然后重新加载扩展。
- **升级后衣橱为空：**核对原 `Data/config.toml` 与 `storage.data_dir`，不要切换成另一份 Windows/WSL SQLite。
- **测试订单重复：**重复导入会去重；仅在隔离测试库执行一次即可。
- **京东/淘宝同步按钮不存在：**公开 Core 不含电商网站采集器，是正常结果。
- **macOS 拦截程序：**根据系统安全提示决定是否允许未公证的可执行文件。

问题仍存在时可[提交 Issue](https://github.com/ThinkerQAQ/wardrobe/issues)，附操作系统、复现步骤和脱敏日志，**不要上传 `Data/`、订单或个人照片**。

## 3. Configuration

安装程序在**已安装可执行文件**旁创建 `Data/config.toml`，升级时保留。默认配置见 [config.example.toml](config.example.toml)：

```toml
[server]
address = "127.0.0.1:17936"
open_browser = true

[storage]
data_dir = "."
```

`data_dir = "."` 以配置文件所在目录为基准，通常是 `Data/`。发布版浏览器侧边栏固定连接本机 `17936` 端口；修改 `server.address` 后扩展不会自动调整。

**备份与升级：**先停止服务，备份**完整 `Data/`**（SQLite、WAL、照片、`config.toml`）及 `Private/` 中的 Tunnel 凭据，再在原安装位置更新。升级后先检查本地健康接口和已有单品。页面「导出备份」只导出 JSON 记录，**不等于完整数据库与图片备份**。

## 4. Optional Features

### 4.1. Case 1：连接本地 MCP 客户端

MCP 可以让 AI 搜索衣橱、导入人工核实的商品并保存搭配，但**不是 Web UI 或扩展的运行前提**。

1. 找到**已安装**的 `wardrobe` / `wardrobe.exe` 及 `Data/config.toml`。
2. 在本地 MCP 客户端添加以下 stdio 配置，使用**同一套安装位置**的绝对路径：

```json
{
  "mcpServers": {
    "wardrobe": {
      "command": "/absolute/path/wardrobe/wardrobe",
      "args": ["-mcp-stdio", "-config", "/absolute/path/wardrobe/Data/config.toml"]
    }
  }
}
```

Windows 的命令以 `wardrobe.exe` 结尾，JSON 可以使用正斜杠路径。重连后调用 `get_wardrobe_capabilities`，再调用 `search_wardrobe`；做过测试的可以传入 `{"query":"Demo 衬衫"}`，否则查询自己的衣物，并与 Web UI 对照。

如果需要录入**手动单品**，先搜索查重，再调用 `create_wardrobe_product`。`platform="手动"`、`verification="manual"` 是接口要求，即使英文界面也使用该值；`source_key` 必须唯一。独立测试库可用以下参数：

```json
{
  "source_key": "manual:example:shirt-1",
  "platform": "手动",
  "order_id": "",
  "verification": "manual",
  "name": "Sample shirt",
  "category": "衬衫",
  "color": "blue",
  "size": "M"
}
```

记录返回的 ID/version；相同来源键会被拒绝重复导入，修改时须使用最新 version。不能凭猜测虚构购买记录或本人持有状态。

### 4.2. Case 2：ChatGPT 远程连接

远程使用需要**你自己的 Secure MCP Tunnel、工作区授权和运行密钥**。重新运行安装向导（Windows：`Setup-Wardrobe.cmd`；Linux/macOS：`bash scripts/setup.sh`），这次选择 Tunnel 配置。向导下载并校验官方 Tunnel 客户端，创建**独立 stdio MCP** 连接。

随后在账号支持时，通过 ChatGPT 的「**插件 → 添加自定义 MCP 服务器 → Tunnel**」选择自己的 Tunnel，完成授权。重连后先调用只读的 `get_wardrobe_capabilities` 和 `search_wardrobe` 验收。Tunnel **不会把整个本地 Web UI 发布到公网**。Windows 用 DPAPI 保护运行密钥，Unix 使用用户私有的 0600 凭据文件。

### 4.3. Case 3：导入订单文件

公开 Core 通过本机 `POST /api/orders/import-file` 导入**归一化 JSON、CSV 和 XLSX**。**当前 Web UI 没有上传订单文件的表单**，不要在界面中寻找「导入文件」按钮。

准备有权使用的订单数据，核对订单号、名称和规格，不要上传 Cookie 或未经筛选的原始平台响应。使用本机 `curl`：

```bash
curl -fsS 'http://127.0.0.1:17936/api/orders/import-file' \
  -F 'platform=YourSource' \
  -F 'file=@/absolute/path/to/normalized-orders.json'
```

将平台名和文件路径换成实际值。导入后在「**订单库**」检查，再进入「**衣橱 → 整理穿搭入衣橱**」建立待确认衣物。补齐商品图是独立流程，部分远程图片来源需要另行配置可信主机。公开仓库**不包含电商私有采集适配器**。

### 4.4. Case 4：搭配与图片

在「衣橱」核对商品、持有状态和合身反馈，只有本人确认后再标记为实际拥有。到「搭配」页面选择现有单品，保存组合。「身体数据」「发型」「Skills」分别管理对应内容。

效果卡可以使用已授权的人像和商品图片生成静态搭配图，但**不是 AI 真人试穿**：真实试穿渲染接口尚未实现。记录和照片保存在本地，但通过云端 MCP 明确发送给 AI 的内容可能传输到相应服务。

## 5. Development

源码开发需要 Git 及 [go.mod](go.mod) 声明的 Go 工具链；仅在运行前端测试时需要 Node.js：

```bash
git clone https://github.com/ThinkerQAQ/wardrobe.git
cd wardrobe
go test ./...
go vet ./...
go build -o wardrobe .
mkdir -p Data
./wardrobe -data ./Data -open=false
```

Windows 可使用 `go build -o wardrobe.exe .`、`.\wardrobe.exe -data .\Data -open=false`（必要时先创建 `Data`），然后访问 [http://127.0.0.1:17936](http://127.0.0.1:17936)。

```bash
node --test web/tests/*.test.cjs
node --test browser-extension/tests/*.test.cjs
node scripts/sync-browser-ui.mjs --check
bash scripts/audit-public.sh
```

CI 还验证安装器保留已有数据，并构建 Windows/Linux/macOS 的 x64/ARM64 版本。`skills/` 中的 Markdown 是**嵌入二进制的运行时资源**，不是冗余教程，不应删除。采用功能分支、PR Review 和 Squash Merge。

## 6. License

Wardrobe 原创代码采用 [MIT License](LICENSE)。第三方依赖遵循各自许可证，详见[第三方声明](THIRD_PARTY_NOTICES.md)及发行包中的 `THIRD_PARTY_LICENSES/`。电商内容、购买记录、商品照片及个人图片不属于源码授权范围。

**安全与隐私：**Wardrobe 是仅供**单用户、本机**使用的应用，不能把完整 HTTP 工作台、导出接口或未保护的服务暴露到公网。保护 `Data/`、`Private/`、照片、订单与 MCP Token。现有侧边栏通过本地同源界面访问数据，**不需要配对码**；旧版带配对的订单接口仅为兼容而保留。备份前先停止数据库访问，以免遗漏 WAL。

云端 MCP 调用可能将指定字段或图片传给相应 AI 服务；应审查扩展权限和第三方素材的使用授权。安全问题请发送至 **contact@thinkerqaq.com**，仅提供脱敏复现信息，不要在公开 Issue 中提交凭据、收货地址、购物记录或私人照片。
