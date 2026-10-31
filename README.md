# Wardrobe

[简体中文](README_ZH.md) · [Documentation](docs/SETUP.md) · [MCP](docs/MCP.md) · [Security](SECURITY.md)

A local-first wardrobe and outfit assistant built with Go, SQLite, a Web UI and MCP. Own your inventory, outfit combinations, hairstyles and body measurements while keeping private records local.

- Single-user localhost Web UI and Edge/Chrome side panel.
- SQLite clothing inventory, outfits, product images, hairstyle references, skill configuration.
- MCP tools for controlled local queries, writes and AI-assisted outfit planning.
- Generic order-item model with local CSV/XLSX/JSON imports, deduplication and wardrobe classification.
- Optional source adapters can be added in independent, user-controlled builds. **The public source and releases do not contain retailer-page automation or retailer-specific capture code.**
- Core can be built and tested alone; **no private repositories, tokens or account cookies are required**.

## Install

Use [Releases](https://github.com/ThinkerQAQ/wardrobe/releases) for public builds or run `go run .` locally. See [setup](docs/SETUP.md).

## Privacy and legal use

Personal SQLite files, images, credentials and settings stay in `Data/` on your computer. Connecting cloud AI through MCP may send only the fields or images you explicitly request. Never publish personal order data, photos, credentials or backups.

Wardrobe is independent and not affiliated with any retailer. Users must have rights to their accounts, data and media and comply with applicable law and third-party service terms. The [MIT License](LICENSE) covers the original code, **not** retailer services, product data or third-party photos.

## Development

```bash
go test ./...
go vet ./...
go build .
```

For architecture and optional adapter contracts, see [Order source API](docs/ORDER_SOURCES.md). The public build works without optional extensions.
