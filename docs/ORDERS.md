# Local order imports

The public Wardrobe Core accepts normalized order line items and local CSV/XLSX/JSON files. All records are stored in the local SQLite database; no remote shopping account is required.

The source identifier (`platform`) is a non-empty, bounded string. Order item identity uses the existing `platform/order_id/line_id` scheme for backward compatibility. Source data is sanitized, purchase history is not automatically proof of ownership, and duplicate rows are reconciled conservatively.

To import files, use the localhost `POST /api/orders/import-file` endpoint with `platform` and `file` multipart form fields. The normalized JSON endpoint is `POST /api/orders/import`. See [optional source integrations](ORDER_SOURCES.md).

The public browser extension only displays the local wardrobe and never reads retailer sites or captures sessions.
