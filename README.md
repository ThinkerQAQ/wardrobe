# Wardrobe

[简体中文](README_ZH.md)

## Introduction

A local wardrobe and outfit manager with SQLite, a Web UI, an Edge/Chrome side panel and MCP tools. Keep clothing records, photos, outfits, hairstyles and body measurements in one local application. Public Core runs independently: retailer adapters and publishing/browser automation projects are not required or included.

## Quick Start

### 1. Prerequisites

Packaged use needs a supported Windows, Linux or macOS machine, a browser, and a local stdio MCP client for the first item-entry workflow below. No Go, Node.js, private repository or account cookie is required. Compiling from source needs the Go version declared in [go.mod](go.mod).

### 2. Download and install

Download your CPU/OS archive and `SHA256SUMS.txt` from [Releases](https://github.com/ThinkerQAQ/wardrobe/releases/latest). Choose `windows-amd64` for Windows x64, `windows-arm64` for Windows ARM64, `linux-{amd64,arm64}` for Linux, or `darwin-{amd64,arm64}` for macOS. Verify the archive's SHA-256 against the manifest, extract it, then:

- **Windows:** double-click `Setup-Wardrobe.cmd`. Skip the optional Tunnel step for local use. The default installation is `%LOCALAPPDATA%\Wardrobe`.
- **Linux/macOS:** from the extracted directory run `bash scripts/setup.sh --no-tunnel`. The default installation is `~/.local/share/wardrobe`.

The installer initializes local configuration and starts the app. It preserves existing data/configuration. Open <http://127.0.0.1:17936>; no pairing code is required. If your existing data lives elsewhere, keep that directory and point configuration to it rather than starting a second empty installation.

### 3. Connect a local MCP client

Add this server to your client's MCP settings. Replace both paths with absolute paths in your installed directory; on Windows the command ends in `wardrobe.exe` and paths can use forward slashes.

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

Reconnect the client. Call `get_wardrobe_capabilities`, then `search_wardrobe` with `{"query":"Quick Start shirt"}` to check for a previous test item. Stdio and the Web UI must use the same config/data directory.

### 4. Create and verify one item

In a fresh test installation, call `create_wardrobe_product` with the following **synthetic manual record**, not a fabricated purchase:

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

`手动` is the required API value for manual records, including in the English UI. Note the returned item ID/version. Search again, then refresh the Web UI and open **Wardrobe**: the same item must appear. Ownership/fit/rating remain pending until explicitly confirmed. Restart the app with the same configuration and confirm that the item persists. This is the completion standard; an empty running page is not enough. Repeating the same source key is rejected as a duplicate.

### 5. Troubleshooting

- **Empty inventory after an upgrade:** check `storage.data_dir` and the exact config path. Do not overwrite an existing SQLite file or run against a second WSL copy.
- **Port occupied:** use the already-running instance or change `server.address` consistently; the browser extension expects port 17936.
- **Stdio tools missing:** use absolute paths, refresh the connection and call capabilities again. Stdio waits for protocol messages and is not an interactive shell.
- **Duplicate/version conflict:** search and read the existing item, then use its current version for updates.
- **macOS trust prompt:** the binary is not Apple-notarized; handle the operating system's approval yourself.

## Configuration

The installer writes `Data/config.toml` beside the executable. For a manual installation copy [config.example.toml](config.example.toml) there before connecting the client:

```toml
[server]
address = "127.0.0.1:17936"
open_browser = true

[storage]
data_dir = "."
```

Relative `data_dir` is resolved relative to the config file, so `.` means its `Data/` directory. CLI `-config`, `-data`, `-addr` and `-open` provide explicit overrides; `-open=false` is useful on a server. Keep the whole data directory, images, SQLite/WAL and config together when backing up or moving a stopped installation. Never commit them.

## Optional Features

### Browser side panel

Load the installed `browser-extension/` directory using **Developer mode → Load unpacked** at `edge://extensions` or `chrome://extensions`. Copy its extension ID and run the installed executable with `--register-native-host YOUR_EXTENSION_ID` (replace the ID). Reopen the panel and verify **Start local service**. Re-register after moving the executable or changing the extension ID. The bundled shell remains navigable offline; data and saves require Core.

### Remote ChatGPT

Rerun setup and select Tunnel integration if you have your own tunnel/workspace authorization and runtime key. Setup downloads and verifies official tunnel-client. Complete the account's **Add custom MCP server → Tunnel** flow yourself, refresh tools, then call `get_wardrobe_capabilities` and `search_wardrobe`. The tunnel transports dedicated stdio MCP, not the full HTTP UI. Windows stores the key using DPAPI; Unix uses a user-owned mode-0600 file. Do not share the developer's tunnel or secrets.

### Orders, outfits and images

Core accepts local CSV/XLSX/normalized JSON order imports through its local APIs. The minimal normalized payload is `{"platform":"example","items":[{"order_id":"demo-1","line_id":"sku-a","name":"Example coat","spec":"M","quantity":1}]}`; inspect `order_file_import.go` and `orders.go` for the maintained contracts. Public Core has no retailer collectors. Optional source-specific adapters are composed at build time; they are not required for manual inventory.

After import, **Wardrobe → Organize wardrobe items** creates pending candidates; **Fill product images** is a separate operation and retailer downloads require an optional trusted host policy. Use the **Outfits** page to select real inventory items and save a combination. MCP image upload accepts authorized JPEG/PNG bytes and the current record version. The fixed lookbook compositor produces a 1200×1800 card from supplied portrait and stored product photos; it does not generate a verified try-on. The try-on rendering endpoint remains unimplemented (HTTP 501).

Built-in `skills/` templates and references are runtime assets embedded in the binary. User-edited Skills are saved to SQLite, not the source files.

## Development

```sh
git clone https://github.com/ThinkerQAQ/wardrobe.git
cd wardrobe
go test ./...
go vet ./...
go build -o wardrobe .
./wardrobe -data ./Data -open=false
```

Use the Go toolchain declared by `go.mod`; Go may download it automatically. Windows builds use `go build -o wardrobe.exe .` and `./wardrobe.exe`. Open the local UI and use the same `Data/` for MCP.

Frontend checks: `node --test web/tests/*.test.cjs`, `node --test browser-extension/tests/*.test.cjs`, `node scripts/sync-browser-ui.mjs --check`. Public boundary check: `bash scripts/audit-public.sh`. CI runs Go tests/vet, frontend and installer checks, Windows build/installer preservation tests, dependency-license and vulnerability checks. Releases target Windows/Linux/macOS x64/ARM64; cross-builds do not establish native macOS runtime or account authorization.

## License

[MIT](LICENSE) covers original code. Retain [third-party notices](THIRD_PARTY_NOTICES.md) and bundled dependency licenses. Source licensing does not cover retailer services, order data, product photos or personal images. Use only data/media you may access and comply with service terms. Selected MCP fields/images can be transmitted to the connected AI service; local storage is not a promise of cloud-free processing. Never publish Data, credentials, backups or private adapter builds. See [security policy](SECURITY.md).
