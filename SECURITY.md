# Security and privacy

[简体中文](SECURITY_ZH.md) · [Home](README.md)

Wardrobe is a **single-user local application**. It is not a secure multi-tenant service that should be deployed directly to the public internet.

## Responsible disclosure

Do not post cookies, API keys, MCP bearer tokens, orders, delivery addresses, face photos, measurements or local debug logs with secrets in public issues or pull requests. For reproducible security issues, privately contact **contact@thinkerqaq.com** with a sanitized reproduction, affected version and likely impact.

## Operational safety

- The local Web UI binds to **127.0.0.1**. Do not port-forward or proxy the full Web UI or /api/export publicly.
- Dedicated stdio MCP access can be connected through the official per-user Secure MCP Tunnel. Tunnel is **not** a general-purpose public reverse proxy.
- Local MCP tokens in Data/mcp-token, runtime API keys and user-specific Tunnel IDs must never be committed or shared.
- Web UI and MCP have different access controls. HTTP MCP may require bearer authentication; the Web UI is protected primarily by the loopback access boundary.
- SQLite and images live locally, but **any records or images you explicitly request through cloud-connected MCP may be transmitted to that AI service**.
- Back up the entire Data directory only after stopping database access; live SQLite files may have WAL data not present in the main database file.
- Browser order import uses a per-user pairing key in `Data/order-import.key` with a scoped Chrome extension origin; do not expose either import endpoint or the key publicly. The extension reads only matching order API responses while the user has enabled capture. Review its browser permissions before installing; a malicious shopping page could still interfere with content received by its extension.
- Shopping-platform exports and order snapshots are private. Log into retailer services only through their official websites and do not install unknown account-export extensions.
- Original product photographs, retailer content and third-party binaries do not inherit the MIT source-code license.

## Installer security

The one-click installer downloads only from **ThinkerQAQ/wardrobe** and **openai/tunnel-client** official GitHub Releases and checks their SHA256SUMS.txt manifests. A checksum verifies file integrity against the release publisher; it is **not independent protection against a compromised upstream release**.

Each user needs their own restricted runtime key and Tunnel ID. The Windows wizard stores runtime keys using **user-bound Windows DPAPI**. Linux/macOS store the key in a user-owned file with mode **0600** under a private installation directory. Filesystem access by the same OS user still constitutes access to these credentials.

No user database, purchase history, photos, token, personal tunnel configuration or runtime secret belongs in a GitHub release. **Never upload user-specific data to this repository.**
## Dependency updates and CI

Dependencies are monitored with GitHub Dependabot security alerts and updates. CI runs `govulncheck` against reachable Go call paths. Protected `main` requires passing `test` and `windows-installer`, linear history, resolved PR discussions and admin enforcement; merge mode is squash-only and merged feature branches are automatically removed. Keep Go at a supported patched release when building distributable binaries.

## Extension side panel (same-origin local application)

The Edge/Chrome side panel embeds an explicit loopback page (`/?sidepanel=1`), which accesses SQLite through same-origin Go APIs. No manual pairing is required. Only that iframe entry allows `chrome-extension:` ancestors; all other routes retain frame denial. The extension parent cannot directly read the iframe DOM due to the browser's same-origin policy. Order capture explicitly forwards only normalized whitelisted item fields to that local frame; no cookie values, raw order bodies, or remote uploads are part of the collector. Do not expose local HTTP services to a public network. The older paired import endpoint is retained for compatibility but is not used by the current UI.
