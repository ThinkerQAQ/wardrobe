# Wardrobe

[简体中文](README_ZH.md)

- [1. Introduction](#1-introduction)
- [2. Quick Start](#2-quick-start)
- [3. Configuration](#3-configuration)
- [4. Optional Features](#4-optional-features)
- [5. Development](#5-development)
- [6. License](#6-license)

## 1. Introduction

It is easy to remember buying clothes, but harder to know which ones you still own, how they fit, and what goes together. Purchase histories, product pictures, sizes and outfit ideas are usually scattered across different places.

Wardrobe brings these records into one local workspace. You can review purchases, organize clothes and accessories, confirm what you own, keep fit notes and photos, and save outfits made from your actual inventory. Use the Web UI or an optional Edge/Chrome side panel; connect an MCP client only if you want AI assistance.

**The public repository contains a standalone Wardrobe Core.** It supports local inventory, order-file imports and outfits, but does **not** include the private JD/Taobao website collectors. Installing the browser extension does not add retailer capture capabilities. You do not need a retailer account, ChatGPT or browser automation to start.

## 2. Quick Start

Install the app, open its Web UI and check an order-to-wardrobe workflow. The Web UI works without MCP or a browser extension.

### 2.1. Requirements

Use Windows, Linux or macOS on an **x64/ARM64** machine with a browser. Official Release archives include the executable; **Go and Node.js are not required** for normal installation.

If you have an existing Wardrobe installation, keep its original `Data/` directory. Do not accidentally start a second, empty database in Windows or WSL.

### 2.2. Download and install

1. Open the [latest Release](https://github.com/ThinkerQAQ/wardrobe/releases/latest) and download the archive for your OS/CPU: `wardrobe-windows-amd64.zip`, `wardrobe-windows-arm64.zip`, `wardrobe-linux-amd64.tar.gz`, `wardrobe-linux-arm64.tar.gz`, `wardrobe-darwin-amd64.tar.gz` or `wardrobe-darwin-arm64.tar.gz`.
2. Download `SHA256SUMS.txt` from the **same Release**. Compare the archive hash to its **matching filename**: run `Get-FileHash .\wardrobe-windows-*.zip -Algorithm SHA256` (Windows PowerShell), `sha256sum ./wardrobe-linux-*.tar.gz` (Linux), or `shasum -a 256 ./wardrobe-darwin-*.tar.gz` (macOS) in the downloads directory. Stop on a mismatch.
3. Extract the **entire** archive. It contains `wardrobe.exe` / `wardrobe`, `config.example.toml`, `scripts/` and `browser-extension/`.
4. Start the installer from the extracted folder:

**Windows:** double-click `Setup-Wardrobe.cmd`. Answer **N** when asked about the optional ChatGPT Tunnel. Default install path: `%LOCALAPPDATA%\Wardrobe`. Alternatively, run this in PowerShell to skip the Tunnel question:

```powershell
powershell.exe -NoProfile -ExecutionPolicy Bypass -File .\scripts\Setup-Wardrobe.ps1 -NoTunnel
```

**Linux/macOS:** from the extracted folder run:

```bash
bash scripts/setup.sh --no-tunnel
```

Default install path: `~/.local/share/wardrobe`. Setup copies the app and extension assets, preserves the existing `Data/`, and configures local startup where supported. **Copying extension files does not install the browser extension.**

### 2.3. Open the local workspace

Open [http://127.0.0.1:17936](http://127.0.0.1:17936). You should see **Orders, Wardrobe, Hairstyles, Body Profile, Outfits and Skills**. Empty inventory on a fresh installation is normal.

If the page cannot be reached, start the installed app manually:

**Windows PowerShell:**

```powershell
powershell.exe -NoProfile -ExecutionPolicy Bypass -File "$env:LOCALAPPDATA\Wardrobe\scripts\Start-Wardrobe-Local.ps1"
```

**Linux/macOS:**

```bash
bash "$HOME/.local/share/wardrobe/scripts/start-local.sh"
```

Keep the terminal running if your OS has no available user service manager. Open [http://127.0.0.1:17936/api/health](http://127.0.0.1:17936/api/health); it should return `{"app":"wardrobe","status":"ok"}`. **No pairing code is required.** The extension can show an offline workspace without Core, but saved data requires the running service.

### 2.4. Create and verify an example item

This **fictional test order** is only for a **fresh test database**. **Skip it if you already have personal wardrobe data**; use an existing, verified order instead. The example writes a persistent record to the current database.

Send one synthetic order to the local Core API:

**Windows PowerShell:**

```powershell
$body = '{"platform":"Demo","items":[{"order_id":"SYNTHETIC-001","line_id":"shirt-1","name":"Demo 衬衫","quantity":1}]}'
Invoke-RestMethod -Uri 'http://127.0.0.1:17936/api/orders/import' -Method Post -ContentType 'application/json' -Body ([Text.Encoding]::UTF8.GetBytes($body))
```

**Linux/macOS:**

```bash
curl -fsS 'http://127.0.0.1:17936/api/orders/import' \
  -H 'Content-Type: application/json' \
  --data '{"platform":"Demo","items":[{"order_id":"SYNTHETIC-001","line_id":"shirt-1","name":"Demo 衬衫","quantity":1}]}'
```

On first use the response includes `"inserted":1`. Now complete the UI steps:

1. Open **Orders**. Confirm that `Demo 衬衫` appears.
2. Open **Wardrobe** and click **Organize wearables** (`整理穿搭入衣橱` in Chinese). This action is on the **Wardrobe** page, not Orders.
3. Confirm that the shirt appears in inventory with ownership **Unconfirmed / 待确认**. A synthetic purchase is not evidence of actual ownership.
4. Restart the same installation with the same configuration and verify the shirt persists.

The public Core does not show usable **Sync JD/Taobao** buttons because retailer collectors are not included. You can import your own file through the local API; see [Case 3](#43-case-3-import-an-order-file).

### 2.5. Install the browser side panel (optional)

Setup places `browser-extension/` in the **installed** Wardrobe directory. To enable browser-side **automatic local startup**, you must load the unpacked extension and register the Native Messaging Host with **its actual Extension ID**:

1. Open `edge://extensions` or `chrome://extensions`, enable **Developer mode**, and click **Load unpacked**.
2. Select the installed `browser-extension/` folder, which directly contains `manifest.json`. Defaults: Windows `%LOCALAPPDATA%\Wardrobe\browser-extension`; Linux/macOS `~/.local/share/wardrobe/browser-extension`. Do not select the ZIP or the source checkout.
3. Open the extension's **Details** and copy its 32-character **Extension ID**.
4. Register from the installed binary, replacing the placeholder:

**Windows PowerShell:**

```powershell
Set-Location "$env:LOCALAPPDATA\Wardrobe"
.\wardrobe.exe --register-native-host YOUR_EXTENSION_ID
```

**Linux/macOS:**

```bash
"$HOME/.local/share/wardrobe/wardrobe" --register-native-host YOUR_EXTENSION_ID
```

5. **Reload** the extension and click Wardrobe in the browser toolbar. Its side panel displays the same workspace. If Core is stopped, the bundled UI remains visible and attempts **Start local service** through the Native Host; once connected, inventory appears.

This registration cannot be completed before the extension's ID exists. No manual pairing code or API token is required. Re-register after moving the installation or changing the extension ID. If the Native Host is missing, use [2.3](#23-open-the-local-workspace) to start Core.

### 2.6. Troubleshooting

- **Dashboard unavailable:** start Core as in [2.3](#23-open-the-local-workspace), check `/api/health` and confirm port `17936` belongs to the intended instance.
- **Side panel stays offline:** check that the extension was loaded from the **installed** folder, its ID was registered, and the local service is healthy. Reload it after registration.
- **Wardrobe is empty after updating:** check the original `Data/config.toml` and `storage.data_dir` before making changes; avoid duplicate Windows/WSL databases.
- **Example import already exists:** the same order is deduplicated. Run the demo only once in a disposable installation.
- **Retailer sync buttons are absent:** this is expected in the public Core.
- **macOS blocks execution:** use the operating system's security controls to review an unsigned app; notarization is not claimed.

[Open an Issue](https://github.com/ThinkerQAQ/wardrobe/issues) for reproducible problems, including OS and sanitized errors. Do not upload `Data/`, private orders or photos.

## 3. Configuration

Setup creates `Data/config.toml` **next to the installed executable** and retains it during upgrades. See [config.example.toml](config.example.toml):

```toml
[server]
address = "127.0.0.1:17936"
open_browser = true

[storage]
data_dir = "."
```

A relative `data_dir = "."` refers to the configuration file's folder, normally `Data/`. The packaged browser extension expects port `17936`; changing `server.address` does not update it automatically.

**Backup and upgrade:** stop Core, back up the **whole `Data/` folder** (SQLite, WAL files, images and configuration) and any `Private/` Tunnel credentials, then install the new version over the **intended existing installation**. Verify the health endpoint and saved inventory afterward. The Web UI's **Export Backup** downloads JSON data, **not** a complete SQLite/photo backup.

## 4. Optional Features

### 4.1. Case 1: Local MCP connection

MCP lets an assistant search inventory, add manually verified products and save outfits; the Web UI does not depend on it.

1. Locate the **installed** `wardrobe` / `wardrobe.exe` and `Data/config.toml`.
2. Add this stdio server in your local MCP client, replacing both paths with **absolute paths to the same installation**:

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

Windows uses `wardrobe.exe`; forward slashes work in JSON paths. Reconnect and call `get_wardrobe_capabilities`, followed by `search_wardrobe` using `{"query":"Demo 衬衫"}` if you ran the example, or an empty query otherwise. The result should match the Web UI.

For a **manual item**, first search for duplicates, then call `create_wardrobe_product` with a unique source key and the actual product facts. The API requires `platform="手动"` and `verification="manual"` for manually entered items, even when the UI is English. For a disposable test database:

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

Record the returned ID/version. Duplicate keys are rejected, and edits require the current version. Do not invent purchase or ownership evidence.

### 4.2. Case 2: Remote ChatGPT

For remote use, obtain **your own** authorized Secure MCP Tunnel ID and runtime API key. Rerun the installer (Windows `Setup-Wardrobe.cmd`; Linux/macOS `bash scripts/setup.sh`) and opt into Tunnel setup. It downloads and checks the official Tunnel client, then creates a dedicated **stdio MCP** connection.

Complete ChatGPT's **Plugins → Add custom MCP server → Tunnel** flow yourself where supported. Reconnect the tools and check `get_wardrobe_capabilities` and `search_wardrobe` before any writes. The Tunnel does **not** expose the entire local Web UI. Windows protects runtime keys with DPAPI; Unix writes user-owned mode-0600 credential files.

### 4.3. Case 3: Import an order file

Public Core accepts normalized **JSON, CSV and XLSX** via its **local** `POST /api/orders/import-file` endpoint. **The current Web UI has no file-upload form**; do not look for an **Import file** button.

Prepare your authorized, normalized order export with real order IDs and names, and call the local endpoint from a trusted tool:

```bash
curl -fsS 'http://127.0.0.1:17936/api/orders/import-file' \
  -F 'platform=YourSource' \
  -F 'file=@/absolute/path/to/normalized-orders.json'
```

Replace the platform and filename with the real source. Review items under **Orders**, then click **Wardrobe → Organize wearables** to create unconfirmed inventory candidates. Image caching is separate and may require trusted source configuration. The public Core has **no built-in retailer website collectors**.

### 4.4. Case 4: Outfits and photos

Open **Wardrobe** to confirm ownership, fit and product details. In **Outfits**, select items already in your inventory and save a combination. **Body Profile**, **Hairstyles** and **Skills** have their own screens.

A lookbook card can combine an authorized portrait and existing product photos. It does **not** constitute an AI virtual try-on; the real try-on rendering endpoint is not implemented. Records and images are stored locally, but anything explicitly sent to a cloud-connected MCP assistant may reach that AI service.

## 5. Development

Source development needs Git and the Go toolchain declared in [go.mod](go.mod); Node.js is only needed for frontend checks:

```bash
git clone https://github.com/ThinkerQAQ/wardrobe.git
cd wardrobe
go test ./...
go vet ./...
go build -o wardrobe .
mkdir -p Data
./wardrobe -data ./Data -open=false
```

On Windows, use `go build -o wardrobe.exe .` then `.\wardrobe.exe -data .\Data -open=false` (create `Data` first). Browse [http://127.0.0.1:17936](http://127.0.0.1:17936).

```bash
node --test web/tests/*.test.cjs
node --test browser-extension/tests/*.test.cjs
node scripts/sync-browser-ui.mjs --check
bash scripts/audit-public.sh
```

CI also checks installer data preservation and builds for Windows/Linux/macOS x64/ARM64. Files under `skills/` are **runtime Markdown assets embedded into the binary**, not redundant end-user tutorials; they remain in the repository. Use PR review and Squash Merge.

## 6. License

Original Wardrobe code uses the [MIT License](LICENSE). Third-party dependencies retain their licenses, documented in [third-party notices](THIRD_PARTY_NOTICES.md) and bundled `THIRD_PARTY_LICENSES/`. Retailer data, product photography and personal images are not relicensed.

**Security:** Wardrobe is a **single-user local application**, not a publicly exposed multi-tenant Web server. Never proxy the full HTTP UI, export endpoint or unauthenticated local APIs onto the public Internet. Protect `Data/`, `Private/`, photos, orders and MCP tokens. The current extension panel uses the local UI **without pairing**; an older paired-import API remains only for compatibility. Back up SQLite only after stopping database access so WAL files are included.

Cloud MCP calls can transmit requested fields/images to the connected AI provider. Keep extensions and data sources authorized and respect third-party rights. Report security issues privately to **contact@thinkerqaq.com** with sanitized reproduction steps, not credentials, addresses or personal photographs.
