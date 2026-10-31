# One-click setup and configuration

[简体中文](SETUP.zh-CN.md) · [Home](../README.md) · [Secure Tunnel](TUNNEL.md)

Wardrobe is a local Go + SQLite application. **You do not need Go, Node.js, npm, browser automation or an admin account** to use the packaged application.

## Windows

1. Download the matching ZIP from [GitHub Releases](https://github.com/ThinkerQAQ/wardrobe/releases/latest): wardrobe-windows-amd64.zip (x64) or wardrobe-windows-arm64.zip (ARM64).
2. Extract the ZIP and double-click **Setup-Wardrobe.cmd**.
3. The wizard installs Wardrobe under your own %LOCALAPPDATA%\Wardrobe directory, creates a local config if missing, starts the Web UI, and registers a user-only startup shortcut.
4. Optionally choose ChatGPT integration. It downloads and SHA-256-verifies official openai/tunnel-client, asks for your own personal Tunnel ID and a restricted runtime API key, configures a private stdio MCP profile, and runs doctor checks.
5. Approve the private connector in ChatGPT yourself: Plugins → + → Add custom MCP server → Tunnel. The installer cannot grant OpenAI permissions or approve your connection.

Open **http://127.0.0.1:17936** for local use. Restarting the installer preserves Data, ratings, archived photos and configuration. To use it without ChatGPT, skip the optional Tunnel step.

The installer also supports running from a source checkout: if no local Wardrobe binary exists, it fetches the latest published official release and checks the SHA-256 file. Release users do not need to compile Go.

## Linux and macOS

Download the matching TAR.GZ asset from GitHub Releases (linux-amd64, linux-arm64, darwin-amd64 or darwin-arm64). Extract and run:

    bash scripts/setup.sh

Wardrobe is installed in ~/.local/share/wardrobe. The wizard registers a user-level systemd service when available on Linux or a LaunchAgent for interactive macOS user sessions. If no supported supervisor is detected, use the provided local start scripts. The separate ChatGPT Tunnel is optional.

On Linux/macOS the restricted runtime key is stored only in a user-owned local 0600 file. On Windows it is encrypted with user-bound Windows DPAPI in a private application folder. Protect your OS user account and backups. The macOS binary is not currently Apple-notarized; any system trust prompts must be handled by the user, never bypassed by the installer.

## Required OpenAI permissions (if connecting ChatGPT)

- [Tunnel management](https://platform.openai.com/settings/organization/tunnels): create or reuse a Tunnel ID **belonging to you**, with the appropriate organization/workspace association.
- [Runtime API keys](https://platform.openai.com/settings/organization/api-keys): create a restricted key with Tunnels Read + Use. Managing Tunnels may require Tunnels Read + Manage. **Do not use an admin key for the background client.**
- ChatGPT: create/authorize your personal MCP plugin only if your plan and workspace permit it.

The installer cannot create authorization on someone else's behalf or bypass organization permissions. Each user needs their own private Tunnel. Users without this access can still use the full local Web UI.

## Security, reinstall and troubleshooting

The GitHub release carries only binaries, installation scripts, documentation and non-sensitive TOML examples. Downloaded Wardrobe and tunnel-client archives are checked against the publishers' SHA256SUMS.txt. Your local SQLite, product images, profile data, purchases and secrets must **never** be uploaded to GitHub.

The Web UI stays on 127.0.0.1:17936; the Tunnel starts **only** the dedicated local stdio MCP process. **Do not publicly expose the full Web UI or /api/export.** ChatGPT MCP requests may still send the selected personal fields/photos to your connected AI service; Tunnel is not redaction.

If an existing installation used a different Data directory, do not silently overwrite it. Shut down the old application and migrate the whole directory (including SQLite WAL and image files) or continue using the old location.

For Windows installation diagnostics, run scripts/Start-Wardrobe-Local.ps1 in the installed directory. For Linux/macOS, run scripts/start-local.sh. If Tunnel doctor fails, verify the personal Tunnel ID, restricted key, organization/workspace mapping and network connectivity, then rerun setup. Tool discovery in ChatGPT may need a plugin refresh even when Tunnel health is green.

## Non-interactive installation tests

Windows example:

    powershell -NoProfile -File scripts/Setup-Wardrobe.ps1 -NoTunnel -NoStart -NoAutostart -NonInteractive -WardrobeBinary C:\path\wardrobe.exe -InstallDir "$env:TEMP\WardrobeTest"

Linux/macOS example:

    bash scripts/setup.sh --no-tunnel --no-start --non-interactive --binary /path/to/wardrobe --install-dir /tmp/wardrobe-test

These paths are examples. Use a disposable test directory; automated scripts never need personal Tunnel credentials.

Official references: [Secure MCP Tunnels](https://developers.openai.com/api/docs/guides/secure-mcp-tunnels) · [openai/tunnel-client](https://github.com/openai/tunnel-client).