# 本地订单导入

Wardrobe 公共 Core 支持统一订单商品行模型，以及 CSV、XLSX、JSON 本地文件导入。个人订单存储在本机 SQLite，不需要访问购物网站账号。

`platform` 是有长度限制的来源标识。订单明细使用现有来源、订单号和行标识进行去重，保持原有数据库兼容；订单不自动代表用户已持有商品。

本机 API：`POST /api/orders/import-file`（multipart：`platform` 和 `file`）、`POST /api/orders/import`（规范化 JSON）。详见[可选数据源接口](ORDER_SOURCES.md)。

公开浏览器扩展仅呈现本机衣橱，不监听任何购物网站或读取浏览器会话。
