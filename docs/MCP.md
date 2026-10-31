# Wardrobe MCP: exposed tools and guarantees

[简体中文](MCP.zh-CN.md) · [Home](../README.md)

Wardrobe contains **30 tools** across inventory, outfit planning, user profiles, image composition, editable skills, and source-photo preflight. The SQLite data and image files remain on the user's computer until the user chooses to expose selected data through an authorized MCP call.

## Tool overview

| Category | MCP tools |
| --- | --- |
| Orders | get_wardrobe_order_summary, organize_wardrobe_orders, reconcile_wardrobe_order_duplicates, cache_wardrobe_order_images |
| Inventory | search_wardrobe, get_wardrobe_product, create_wardrobe_product, update_wardrobe_product, save_wardrobe_image, update_wardrobe_review, audit_wardrobe |
| Outfits | list_wardrobe_looks, get_wardrobe_look, save_wardrobe_look, delete_wardrobe_look |
| Personal data & images | list_wardrobe_hairstyles, get_wardrobe_body, update_wardrobe_body, get_wardrobe_rules, get_wardrobe_image |
| Try-on evidence and preflight | get_wardrobe_tryon_evidence, confirm_wardrobe_tryon_photo, preflight_wardrobe_tryon |
| Fixed lookbook | compose_wardrobe_lookbook |
| Skills | get_wardrobe_workflow, list_wardrobe_workflows, get_wardrobe_workflow_source, update_wardrobe_workflow, reset_wardrobe_workflow |
| Capabilities | get_wardrobe_capabilities |

Writes require explicit user instructions and respect version checks. The evidence-confirmation call records a **user assertion**, not verified biometric identity. It must never be inferred from an AI-generated face.

## Optional order sources

The public Wardrobe Core stores normalized order records locally and exposes aggregate-only order MCP tools. It does not include source-specific browser collectors, private shopping-site APIs or account access.

- `get_wardrobe_order_summary` reports aggregate counts and does not return raw rows or account credentials.
- `organize_wardrobe_orders` links eligible clothing/accessory purchases as pending-confirmation wardrobe items.
- `reconcile_wardrobe_order_duplicates` delegates to an optional source adapter; public Core has no source-specific reconciliation.
- `cache_wardrobe_order_images` requires an optional trusted image-host policy. The public build does not fetch from retailer CDNs.

Only explicitly authorized MCP calls may expose selected local fields or images to the connected AI service. See [optional order-source contracts](ORDER_SOURCES.md).

## Producing a fixed lookbook card

1. Read a saved Look and actual SKU records. Inspect original archived product images when needed.
2. Provide a single full-body subject image using a locally archived portraitLookId, or a user-authorized JPEG/PNG portraitImageBase64 (limited to 8 MiB and safe pixel dimensions).
3. Call compose_wardrobe_lookbook with lookId and exactly one portrait source. A minimal input uses lookId and portraitLookId; an optional hairId must refer to an actual hairstyle.
4. The Go renderer outputs **one 1200 × 1800 PNG**. The left side contains only the supplied image; the right side is composed deterministically from archived SQLite SKU photos and names. No default saving or fake product generation.
5. A missing image, invalid ID or incomplete required clothing slot causes an explicit error. A generated preview must **not** be treated as evidence of an actual fitting session.

The local Web UI at /lookbook.html calls the same Go compositor via POST /api/lookbook/render. A browser-uploaded portrait stays on the local host. When you pass an image through the cloud MCP plugin, that data enters the connected AI service.

## What is not implemented

preflight_wardrobe_tryon validates required images, user-declared face/body references, accessories allowlists and other structural requirements. It does **not** render a convincing identity-preserving try-on. The protected POST /api/tryon/render endpoint returns HTTP 501 until a verified image-input renderer and output checks are implemented. External AI generation of anonymous models plus a fixed card cannot be described as a successful verified user try-on.

## Operational checklist

- Do not expose the full Wardrobe HTTP API on the public internet; only route the dedicated stdio MCP via your authorized Tunnel.
- Never upload SQLite, real purchase records, reference images, MCP tokens or runtime keys to GitHub.
- Run Go tests/vet, frontend syntax checks, Windows cross-compilation, and read-only MCP discovery after upgrades.
- Restarts or source-code merges do not automatically refresh the ChatGPT plugin tool snapshot.

See [Secure Tunnel](TUNNEL.md), [Lookbook](LOOKBOOK.md), and [Try-on safeguards](TRYON.md).