# Wardrobe

[English](README.md)

## 项目简介

本地衣橱与穿搭管理应用，提供 SQLite、Web 界面、Edge/Chrome 侧边栏和 MCP 工具，集中管理衣物、图片、搭配、发型与身体尺寸。公开 Core 可以独立运行，不包含也不依赖电商适配器、分发工具或浏览器自动化项目。

## 快速开始

### 1. 前置条件

使用发行包需要 Windows、Linux 或 macOS、浏览器，以及支持本地 stdio 的 MCP 客户端，用于下面的首次单品录入。不需要 Go、Node.js、私有仓库或账户 Cookie。源码编译所需 Go 版本以 [go.mod](go.mod) 为准。

### 2. 下载与安装

从 [Releases](https://github.com/ThinkerQAQ/wardrobe/releases/latest) 下载对应系统/CPU 的压缩包和 `SHA256SUMS.txt`。Windows x64 选择 `windows-amd64`，Windows ARM64 选择 `windows-arm64`，Linux 选择 `linux-{amd64,arm64}`，macOS 选择 `darwin-{amd64,arm64}`。核对压缩包 SHA-256 与校验清单一致后解压：

- **Windows：**双击 `Setup-Wardrobe.cmd`，本地使用可跳过可选 Tunnel 步骤。默认安装目录为 `%LOCALAPPDATA%\Wardrobe`。
- **Linux/macOS：**在解压目录运行 `bash scripts/setup.sh --no-tunnel`，默认安装到 `~/.local/share/wardrobe`。

安装程序初始化配置并启动应用，保留已有数据和配置。访问 <http://127.0.0.1:17936>，无需配对码。已有数据在其他目录时，应继续使用原目录并修改配置，避免建立另一份空库。

### 3. 连接本地 MCP 客户端

在客户端 MCP 设置中添加服务器，将两个路径都替换为安装目录的绝对路径。Windows 命令以 `wardrobe.exe` 结尾，路径可使用正斜杠。

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

重连客户端，调用 `get_wardrobe_capabilities`，再用 `{"query":"Quick Start shirt"}` 调用 `search_wardrobe`，确认是否已创建过测试单品。stdio 与 Web 必须使用同一配置和数据目录。

### 4. 新建并验证一个单品

在全新测试安装中，以以下参数调用 `create_wardrobe_product`。这是明确的手动示例，不是虚构的订单记录：

```json
{
  "source_key": "manual:quickstart:shirt-1",
  "platform": "手动",
  "order_id": "",
  "verification": "manual",
  "name": "Quick Start shirt",
  "category": "衬衫",
  "color": "blue",
  "size": "M"
}
```

`手动` 是 API 规定的来源值，英文界面同样使用该值。记录返回的单品 ID/version，再次搜索，刷新 Web 并进入「衣橱」，应看到同一件单品。持有、合身和评分在本人明确确认前保持待确认状态。使用同一配置重启，单品仍应存在。这才算完成验收，只有空页面能打开还不够。重复导入同一个来源键会被拒绝。

### 5. 常见问题

- **升级后衣橱为空：**检查 `storage.data_dir` 和实际配置路径，不要覆盖已有 SQLite，也不要连接另一份 WSL 数据库。
- **端口占用：**使用已有实例，或同步修改 `server.address`；扩展默认连接 17936。
- **stdio 没有工具：**使用绝对路径，刷新连接，再调用能力工具。stdio 等待协议消息，不是交互式 Shell。
- **重复或版本冲突：**搜索、读取已有记录，用最新 version 更新。
- **macOS 信任提示：**二进制尚未经过 Apple 公证，需要用户自行处理系统授权。

## 配置说明

安装程序在可执行文件旁创建 `Data/config.toml`。手动安装时，先将 [config.example.toml](config.example.toml) 复制到该位置，再连接客户端：

```toml
[server]
address = "127.0.0.1:17936"
open_browser = true

[storage]
data_dir = "."
```

相对 `data_dir` 以配置文件所在目录为基准，`.` 即该 `Data/`。可用 `-config`、`-data`、`-addr`、`-open` 显式覆盖；服务器环境可用 `-open=false`。备份或迁移已停止的安装时，保留整个数据目录、图片、SQLite/WAL 与配置，不要提交到 Git。

## 可选功能

### 浏览器侧边栏

在 `edge://extensions` 或 `chrome://extensions` 开启开发者模式，选择「加载已解压的扩展」，加载安装目录的 `browser-extension/`。复制扩展 ID，用已安装可执行文件运行 `--register-native-host YOUR_EXTENSION_ID`，替换为真实 ID。重开侧边栏，验证「启动本地服务」。移动程序或扩展 ID 改变后需重新注册。扩展内置界面离线仍可浏览，数据与保存需要 Core。

### 远程 ChatGPT

拥有自己的 Tunnel / 工作区权限和 runtime 密钥时，重跑安装并选择 Tunnel 集成。安装程序下载和校验官方 tunnel-client。用户自行完成「Add custom MCP server → Tunnel」，刷新工具，调用 `get_wardrobe_capabilities`、`search_wardrobe` 验收。Tunnel 仅连接专用 stdio MCP，不转发完整 HTTP 界面。Windows 使用 DPAPI 保存密钥，Unix 使用用户所有的 0600 文件。不要复用开发者 Tunnel 或凭据。

### 订单、搭配和图片

Core 通过本地 API 导入 CSV/XLSX/归一化 JSON。最小订单格式为 `{"platform":"example","items":[{"order_id":"demo-1","line_id":"sku-a","name":"Example coat","spec":"M","quantity":1}]}`，最新契约见 `order_file_import.go`、`orders.go`。公开 Core 不含电商采集器；可选来源适配器在构建时组合，手动衣橱无需它们。

导入后在「衣橱 → 整理穿搭入衣橱」生成待确认候选；「补齐商品图」是独立操作，电商图片下载需要可选的可信图片主机策略。「搭配」页面可选择已有真实单品并保存组合。MCP 图片上传使用授权 JPEG/PNG 字节及最新记录版本。固定画册使用提供的人像与库存商品图生成 1200×1800 卡片，不是经过核验的真人试穿；试穿渲染接口尚未实现，返回 HTTP 501。

`skills/` 中的模板和引用被嵌入二进制，属于运行时资产。用户编辑的 Skill 保存于 SQLite，不修改源码。

## 开发与验证

```sh
git clone https://github.com/ThinkerQAQ/wardrobe.git
cd wardrobe
go test ./...
go vet ./...
go build -o wardrobe .
./wardrobe -data ./Data -open=false
```

使用 `go.mod` 声明的 Go 工具链，Go 可能自动下载对应版本。Windows 使用 `go build -o wardrobe.exe .` 和 `./wardrobe.exe`。访问本地界面，MCP 使用同一 `Data/`。

前端检查：`node --test web/tests/*.test.cjs`、`node --test browser-extension/tests/*.test.cjs`、`node scripts/sync-browser-ui.mjs --check`。公开边界检查：`bash scripts/audit-public.sh`。CI 执行 Go 测试/vet、前端及安装检查、Windows 构建与数据保留测试、依赖许可和漏洞检查。发行目标为 Windows/Linux/macOS x64/ARM64，交叉编译不能证明 macOS 原生运行或账户授权已验收。

## 许可证与合规

原创代码采用 [MIT](LICENSE)，保留[第三方声明](THIRD_PARTY_NOTICES.md)及发行包中的依赖许可。源码许可不覆盖电商服务、订单数据、商品图或个人照片。仅使用有权访问的数据和素材，并遵守服务条款。经 MCP 请求的字段或图片可能传入所连接的 AI 服务，本地存储不代表所有处理均不经过云端。不要公开 Data、凭据、备份或私有适配器构建。详见[安全说明](SECURITY.md)。
