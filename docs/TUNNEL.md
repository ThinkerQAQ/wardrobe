# Private Secure MCP Tunnel

[简体中文](TUNNEL.zh-CN.md) · [Setup](SETUP.md) · [Home](../README.md)

Wardrobe exposes a local **stdio MCP server** and a separate loopback Web UI. Use the **official OpenAI Secure MCP Tunnel** to connect the stdio MCP endpoint to your own ChatGPT workspace without exposing the entire local HTTP server.

## Recommended: guided setup

End users should use the packaged one-click installer: **Setup-Wardrobe.cmd** on Windows or **bash scripts/setup.sh** on Linux/macOS. The wizard downloads the official tunnel-client, guides personal credentials, configures the stdio profile, runs doctor checks, and sets up per-user startup when supported. See [Setup](SETUP.md).

## Architecture

~~~text
ChatGPT (approved private plugin)
            |
  OpenAI Secure MCP Tunnel (outbound)
            |
    tunnel-client (local)
            |
  Wardrobe -mcp-stdio
            |
  local SQLite and pictures
~~~

Each user needs their **own** Tunnel ID, a restricted runtime API key and authorization to register a private ChatGPT plugin. Reuse an existing healthy Tunnel instead of creating redundant ones. The installer cannot grant these external permissions.

## Manual installation for advanced users

1. Obtain [openai/tunnel-client](https://github.com/openai/tunnel-client) from its official source and read the locally installed tool's quickstart help.
2. Create a Tunnel in [OpenAI Platform](https://platform.openai.com/settings/organization/tunnels), or reuse one with the correct workspace association. Create a restricted runtime key in [Runtime keys](https://platform.openai.com/settings/organization/api-keys). Do not use an admin key for the runtime.
3. Configure a dedicated stdio command pointing at your absolute Wardrobe executable and actual private Data/config.toml. Do **not** connect to the Wardrobe Web UI port.
4. Run tunnel-client doctor and start a supervised tunnel client. For a standalone foreground session, tunnel-client run is supported; for long-lived managed sessions use the tool's documented runtime supervision or OS user service.
5. In ChatGPT, create a **private** MCP connection using the Tunnel option, approve requested permissions, then verify a read-only search_wardrobe call.

No personal credentials or Tunnel IDs belong in this repository. You can find a non-secret template in [examples/tunnel-client.example.yaml](../examples/tunnel-client.example.yaml).

## Reinitialization after upgrading Wardrobe

The bundled Wardrobe MCP exposes **30 tools**. A connected ChatGPT conversation can retain an older tool catalog after a server upgrade. Restart the tunnel service once and reconnect/refresh the ChatGPT connector's tool discovery; check two successive read-only calls, not just one.

As of v0.9.3, Wardrobe's stdio mode detects repeated `initialize` frames from the same long-lived tunnel process, closes the previous MCP session, and creates a fresh session that advertises the current 30 tools. This fixes `duplicate initialize received` on **sequential** reconnects, without touching the shopping session or private SQLite data. It does **not** isolate simultaneously active multi-client sessions; deployments requiring independent concurrent sessions should use stateless streamable HTTP MCP bound to localhost and authenticated with the existing local token. Never forward the entire Web UI or expose that token over a public interface.

## Different health layers

A healthy Tunnel reports transport connectivity, **not** necessarily that ChatGPT discovered tools or that Wardrobe has the correct database. Check: (1) local Wardrobe and SQLite, (2) tunnel-client doctor/readiness, (3) ChatGPT connector registration and permissions, (4) an actual read-only tool call. Refresh tool discovery after upgrades.

## Privacy

The OpenAI Tunnel forwards only the authorized MCP process. The local HTTP server contains /api/export and Web UI functions and **must not be publicly forwarded**. Uploaded/returned personal fields and images may reach the AI service whenever you actively use the connected ChatGPT plugin; transport does not provide automatic redaction.

See [One-click setup](SETUP.md), [Security](../SECURITY.md), and [official Secure MCP Tunnel documentation](https://developers.openai.com/api/docs/guides/secure-mcp-tunnels).