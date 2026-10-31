# Optional order-source adapters

Wardrobe Core exposes a normalized order JSON contract, shared SQLite store, and two local import APIs.

```json
{"platform":"example","items":[{"order_id":"demo-1","line_id":"sku-a","name":"Example coat","spec":"M","quantity":1}]}
```

Source adapters are **optional**. Public builds include no retailer automation and do not require access to another repository.

- A browser adapter may emit normalized rows only from a source the user is authorized to access.
- An adapter can be composed at **build time**, not loaded through Go's platform-specific plugin system.
- The Core's optional Go hooks provide source-specific legacy deduplication and bounded image-host rules.
- Public bundles must not contain adapter JavaScript, source-specific host permissions, private URL patterns, credentials or account data.
- Any private bundle is built separately and must not be published into this public repository.

The source adapter is responsible for securing retailer access; the open-source license does not grant access to third-party services.
