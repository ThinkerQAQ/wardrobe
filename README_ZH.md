# Wardrobe · 个人衣橱

[English](README.md)

## 1. Introduction

Wardrobe 是一款本地衣橱与穿搭管理应用。你可以导入订单文件、整理衣物和配饰、确认实际持有状态、记录尺码与合身情况，再组合、保存穿搭。SQLite 数据库与照片都保存在本机。

**公开版 Wardrobe Core** 包含 Go 本地服务、Web UI、可选的 Edge/Chrome 侧边栏和 MCP 工具。公开代码和发行包**不包含京东、淘宝专用订单采集器**。使用衣橱不需要 ChatGPT，也不需要 Tunnel。

## 2. Quick Start

### 2.1. Install · 安装

从 [最新 Release](https://github.com/ThinkerQAQ/wardrobe/releases/latest) 下载与你的系统对应的安装包。正常使用不需要安装 Go 或 Node.js。

#### Graphical Interface · 图形界面

| 平台 | 原生安装包 | 安装方式 |
| --- | --- | --- |
| Windows（x64） | Windows 图形安装包（`.exe`） | 运行安装向导、选择目录，完成后勾选 **Open Wardrobe**。 |
| Linux（x64） | Linux 图形安装包（`.run`） | 赋予执行权限并运行安装向导，完成后打开 Wardrobe。 |
| macOS（Intel） | macOS 磁盘映像（`.dmg`） | 打开 DMG 并运行安装向导，再打开 Wardrobe。 |

只有**最新 Release 中实际提供了该平台的安装器**，才算支持原生图形安装。其他平台可使用已有的便携包。升级时选择原安装目录，保留 `Data/` 和 `Private/`；如果必须先卸载，使用 Wardrobe-Maintenance 移除旧程序并保留数据，再安装到原目录。

#### Command Line · 命令行

原生安装器同时支持 Qt Installer Framework 无人值守参数，与图形安装使用**同一套安装逻辑**：

```powershell
# Windows：将下面的示例文件名换成下载的安装包文件名。
& ".\downloaded-installer.exe" --accept-licenses --default-answer --confirm-command install
```

```bash
# Linux：将示例文件名换成下载的安装包文件名，并先赋予执行权限。
# macOS：在已挂载的 DMG 中运行安装器。
./downloaded-installer.run --accept-licenses --default-answer --confirm-command install
```

自动化部署前以对应下载文件的 `--help` 参数说明为准。便携版从同一 Release 下载 `wardrobe-<os>-<arch>` 压缩包，解压后运行 `wardrobe` 或 `wardrobe.exe`。如果需要便携版安装脚本：Linux/macOS 运行 `bash scripts/setup.sh --non-interactive --no-start`，Windows 运行 `scripts/Setup-Wardrobe.ps1 -NonInteractive -NoStart`。

### 2.2. Configuration · 配置

#### Graphical Interface · 图形界面

启动 Wardrobe 后打开 [http://127.0.0.1:17936](http://127.0.0.1:17936)。衣橱与穿搭可以直接使用；只有需要 ChatGPT 时，才进入**MCP 连接**。**登录自启**与**Tunnel 自动连接**分别控制。**保存**只写入配置，不改变当前连接；**重启**使修改后的配置生效。

浏览器侧边栏共用 Web UI。打开 Edge/Chrome 扩展管理页，启用开发者模式，选择**加载已解压的扩展程序**，指向安装目录中的 `browser-extension/`。然后根据真实扩展 ID 注册 Native Messaging：

```bash
wardrobe --register-native-host YOUR_EXTENSION_ID
```

如果命令不在 PATH 中，使用已安装程序的完整路径。服务未启动时，侧边栏仍显示离线工作台，但读取和修改衣橱数据需要本地服务。安装器不会自动安装浏览器扩展。

#### Command Line / Configuration File · 命令行和配置文件

程序首次启动时，如果缺少配置文件，会创建安装目录中的 `Data/config.toml`：

```toml
[server]
address = "127.0.0.1:17936"
open_browser = false

[storage]
data_dir = "."
```

`wardrobe -config /path/to/Data/config.toml` 指定配置文件，`wardrobe -data /path/to/Data` 指定数据库与图片目录。HTTP 只允许本机回环监听。

MCP 命令与图形界面调用同一套 Go 实现：

```bash
wardrobe mcp status
wardrobe mcp start
wardrobe mcp stop
wardrobe mcp restart
wardrobe mcp connect
wardrobe mcp disconnect
```

`wardrobe mcp install-client` 安装经过校验的官方 Tunnel Client。命令行自动配置时，通过**标准输入**向 `wardrobe mcp configure` 传入 JSON，**不要在命令行参数中填写密钥**。Runtime API Key 只保存在本机（Windows 使用 DPAPI，Unix 使用限制权限的文件），状态 API 不返回明文。

### 2.3. Usage · 使用

在**订单库**导入自己的 CSV、XLSX 或 JSON 订单文件，也可以直接添加单品。在**衣橱**点击**整理穿搭入衣橱**，从符合条件的订单中建立待确认单品；**补齐商品图**是独立操作。核对持有状态和合身情况，再到**搭配**里组合、保存穿搭。发型、身体数据和 Skills 有独立页面。

公开 Core 无法采集京东、淘宝网页订单。退款不等于退货。

### 2.4. Connect ChatGPT · 连接 ChatGPT

打开**MCP 连接**，按需安装官方 Tunnel Client，填写自己的 **Tunnel ID** 和 **Runtime API Key**，保存后启动 Tunnel 服务并连接。**Ready** 只代表 Tunnel 健康检查成功，**不代表 ChatGPT 插件已授权**。还需要在 [ChatGPT 插件](https://chatgpt.com/plugins)中添加、授权对应 Tunnel。

**本地 STDIO MCP（无需 Tunnel）**：将下面的配置添加到本机 MCP 客户端，改成实际的安装绝对路径：

```json
{
  "mcpServers": {
    "wardrobe": {
      "command": "/absolute/path/wardrobe",
      "args": ["-mcp-stdio", "-config", "/absolute/path/Data/config.toml"]
    }
  }
}
```

## 3. Development · 开发

```bash
go test ./...
go vet ./...
go build .
node scripts/sync-browser-ui.mjs --check
```

在 `web/` 统一修改 UI，再运行 `node scripts/sync-browser-ui.mjs`，保持浏览器扩展资源一致。Qt Installer Framework 构建入口为 `scripts/package_installer.py`；`--prepare-only` 可验证干净的发行目录。发行包不得包含 `Data/`、`Private/`、个人照片或密钥。

## 4. License · 许可证

Wardrobe Core 使用 [MIT License](LICENSE)。第三方软件及素材遵守各自许可证，详见 [THIRD_PARTY_NOTICES.md](THIRD_PARTY_NOTICES.md)。原生 MCP 生命周期参考 [SyndiRelay #29](https://github.com/ThinkerQAQ/syndirelay/pull/29)，并适配自 [Reverfolio #40](https://github.com/ThinkerQAQ/reverfolio/pull/40)。
