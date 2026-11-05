# Wardrobe

[简体中文](README_ZH.md)

## 1. Introduction

Wardrobe is a local-first workspace for clothes, accessories, outfits, hairstyles and fit notes. Import order files, organize owned items and save outfits. SQLite and photos stay on your computer.

The **public Wardrobe Core** includes a Go local service, Web UI, optional Edge/Chrome side panel and MCP tools. The public code and releases **do not include retailer-specific collectors**. Using the wardrobe does not require ChatGPT or a Tunnel.

## 2. Quick Start

### 2.1. Install

Download from [Releases](https://github.com/ThinkerQAQ/wardrobe/releases). No Go or Node.js is required to use the installed application.

#### Graphical Interface

| Platform | Native installer | Steps |
| --- | --- | --- |
| Windows (x64) | `Wardrobe-Setup-v1.1.3-windows-amd64.exe` | Open installer, choose a folder, select **Open Wardrobe** on completion. |
| Linux (x64) | `Wardrobe-Setup-v1.1.3-linux-amd64.run` | Make executable, open installer, launch Wardrobe. |
| macOS (Intel) | `Wardrobe-Setup-v1.1.3-darwin-amd64.dmg` | Open DMG, run installer and launch Wardrobe. |

Native installers are available **only where those exact files appear in the Release**. For other platforms, use an available portable archive. When upgrading, keep the original installation directory and preserve `Data/` and `Private/`. If necessary, uninstall old program files with Wardrobe-Maintenance first, retaining data.

#### Command Line

The native installer supports Qt Installer Framework unattended installation using the **same package and install logic**:

```powershell
# Windows
.\Wardrobe-Setup-v1.1.3-windows-amd64.exe --accept-licenses --default-answer --confirm-command install
```

```bash
# Linux; grant execute permission first. macOS: run installer inside mounted DMG.
./Wardrobe-Setup-v1.1.3-linux-amd64.run --accept-licenses --default-answer --confirm-command install
```

Check `--help` on your downloaded installer before automated deployment. For a portable installation, extract the `wardrobe-<os>-<arch>` archive and run `wardrobe` (or `wardrobe.exe`). Optional portable setup commands are `bash scripts/setup.sh --non-interactive --no-start` (Linux/macOS) and `scripts/Setup-Wardrobe.ps1 -NonInteractive -NoStart` (Windows).

### 2.2. Configuration

#### Graphical Interface

Start Wardrobe and open [http://127.0.0.1:17936](http://127.0.0.1:17936). Inventory and outfits work without MCP. **MCP Connection** is optional: login startup and automatic Tunnel connection are separate options. **Save** writes settings without changing the running connection; **Restart** applies saved changes.

To enable the optional Edge/Chrome side panel, open your browser's extensions page, enable Developer mode and **Load unpacked**, then select `browser-extension/` in the installed Wardrobe folder. Register its actual extension ID from the installed binary:

```bash
wardrobe --register-native-host YOUR_EXTENSION_ID
```

The extension can show its offline workspace without a local server; reading and editing inventory still requires Wardrobe to run. The browser extension is not installed automatically.

#### Command Line / Configuration File

Wardrobe creates `Data/config.toml` next to the installed binary if it is missing:

```toml
[server]
address = "127.0.0.1:17936"
open_browser = false

[storage]
data_dir = "."
```

Override with `wardrobe -config /path/to/Data/config.toml` and `wardrobe -data /path/to/Data`. The HTTP service only binds loopback. MCP CLI and GUI share the same Go implementation:

```bash
wardrobe mcp status
wardrobe mcp start
wardrobe mcp stop
wardrobe mcp restart
wardrobe mcp connect
wardrobe mcp disconnect
```

Use `wardrobe mcp install-client` for the verified official Tunnel Client. Send configuration JSON via standard input to `wardrobe mcp configure`; never put Runtime API Keys in CLI arguments. Saved keys stay local (Windows DPAPI; restricted files on Unix) and are not exposed by the status API.

### 2.3. Usage

Import your own CSV, XLSX or JSON order file in Orders, or add clothes manually. In Wardrobe, **Organize wearables** makes pending items from eligible purchases; **Cache product photos** is separate. Confirm ownership and fit, then save combinations in Outfits. Hairstyles, body profile and custom Skills are also available.

The public Core cannot capture JD/Taobao website orders. Refunds are not proof of returns.

### 2.4. Connect ChatGPT

In **MCP Connection**, install the official Tunnel Client if needed, enter your own **Tunnel ID** and **Runtime API Key**, save, start, then connect. **Ready** indicates Tunnel health only; ChatGPT plugin authorization must be completed separately in [ChatGPT plugins](https://chatgpt.com/plugins).

**Local STDIO (no Tunnel)** can connect to the same data, with paths adapted to your installed application:

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

## 3. Development

```bash
go test ./...
go vet ./...
go build .
node scripts/sync-browser-ui.mjs --check
```

Edit shared UI under `web/` and run `node scripts/sync-browser-ui.mjs` to update the extension's identical resources. `scripts/package_installer.py` builds native Qt Installer Framework packages; `--prepare-only` validates a clean release stage. Never include `Data/`, `Private/`, photos or credentials in release packages.

## 4. License

Wardrobe Core is [MIT licensed](LICENSE). See [THIRD_PARTY_NOTICES.md](THIRD_PARTY_NOTICES.md) for third-party code and materials. Native MCP lifecycle adapted from [SyndiRelay #29](https://github.com/ThinkerQAQ/syndirelay/pull/29) via [Reverfolio #40](https://github.com/ThinkerQAQ/reverfolio/pull/40).
