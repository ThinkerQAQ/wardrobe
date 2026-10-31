[English](MCP.md) · [首页](../README_ZH.md)

# Wardrobe Local MCP：功能与边界（v0.9.1）

Wardrobe 的主数据位于用户本机 SQLite，不能上传到 GitHub。云端 AI 调用 MCP 时，返回的个人数据、图片、以及用户传入的照片都可能进入 AI 服务。

## 已实现：30 个工具

| 模块 | 工具 | 功能 |
| --- | --- | --- |
| 订单与图片 | get_wardrobe_order_summary, organize_wardrobe_orders, reconcile_wardrobe_order_duplicates, cache_wardrobe_order_images | 汇总订单库、自动整理待确认穿搭、去重归档、下载本地商品图（不向 MCP 传私人订单明细或图片字节） |
| 商品 | search_wardrobe, get_wardrobe_product, create_wardrobe_product, update_wardrobe_product, save_wardrobe_image, update_wardrobe_review, audit_wardrobe | 商品读取/核实导入/编辑/归档/评价/审计 |
| 搭配 | list_wardrobe_looks, get_wardrobe_look, save_wardrobe_look, delete_wardrobe_look | 搭配读取、创建、编辑、版本化删除 |
| 发型/身体/审美 | list_wardrobe_hairstyles, get_wardrobe_body, update_wardrobe_body, get_wardrobe_rules, get_wardrobe_image | 本地原始记录和归档图 |
| 真人照片核验 | get_wardrobe_tryon_evidence, confirm_wardrobe_tryon_photo, preflight_wardrobe_tryon | 检查用户声明、照片 SHA-256、真实商品图和配饰白名单 |
| 效果卡 | compose_wardrobe_lookbook | 1200×1800 PNG；Go 确定性合成，右侧真实商品图；不自动换装 |
| Skills | get_wardrobe_workflow, list_wardrobe_workflows, get_wardrobe_workflow_source, update_wardrobe_workflow, reset_wardrobe_workflow | 查询和版本化编辑 SQLite Skill |
| 能力查询 | get_wardrobe_capabilities | 说明什么功能可用、什么尚未接入 |

## 可选订单来源

公开 Core 仅实现规范化订单明细模型、本机 SQLite、文件导入和受限的订单 MCP 工具，不包含电商网站页面监听或私人接口访问代码。

- `get_wardrobe_order_summary` 只返回汇总数量，不传原始订单和凭据。
- `organize_wardrobe_orders` 将可识别穿搭商品整理为待确认的衣橱单品。
- `reconcile_wardrobe_order_duplicates` 允许可选适配器实现来源特定的历史去重，公开 Core 不自行执行电商重复归档。
- `cache_wardrobe_order_images` 需要可选受限图片来源策略，公开 Core 不从电商 CDN 下载商品图。

MCP 中被主动请求的其他个人字段或照片仍可能传送至所连接的 AI 服务。详见[订单来源适配接口](ORDER_SOURCES.md)。

## MCP 生成固定效果卡

1. 读取 Look 和商品，必要时用 get_wardrobe_image 检查原始商品图片。
2. 准备一张**单幅人物全身照片**。优先使用本地归档 Look 的 portraitLookId；或在用户同意后以 portraitImageBase64 传 JPEG/PNG（最大 8 MiB，最多 2400 万像素）。
3. 调用 compose_wardrobe_lookbook，示例参数：
    {"lookId":"LOOK-DEMO","portraitLookId":"LOOK-REF"}
4. Go 从 SQLite 中选出外套、中层（mid 或 inner）、裤子和鞋子的本机归档主图、名称、颜色和尺码，并生成图片（PNG 直接返回 MCP 客户端）。可选 hairId，必须属于已归档发型。
5. 左侧保持用户传入的人物照片；不自动生成、不换脸、不证明是真人实穿。缺商品图、缺人物图、ID 非法均失败，不可让 AI 重绘右侧信息。

本地 Web UI 的 /lookbook.html 同样调用 Go 的 POST /api/lookbook/render。浏览器选中的人物图发送到本机回环服务，不发送到第三方；通过云端 MCP 调用时则会进入对应云端 AI 服务。**不要把远程 MCP 生图说成完全离线。**

## 真正的真人 AI 试穿：尚未接入

preflight_wardrobe_tryon 只进行预检查。POST /api/tryon/render 仍固定返回 HTTP 501，因为没有能够验证图像输入与结果验收的受控渲染器。若只用外部 AI 生成匿名模特图，再交给搭配卡合成，仍不能称为“本人试穿成功”。

confirm_wardrobe_tryon_photo 记录用户**明确声明**某张本地归档真人照是正脸或全身照，以及图片 SHA-256，**不会自动验证身份**；只有用户确实确认才能调用。

## 安全、部署与测试

- MCP 使用已授权 Tunnel；禁止暴露完整本地 Web UI /api/export 管理服务到公网。MCP bearer token、数据库、历史订单和真人照片不能进 Git。
- 所有写接口保持用户明确反馈/授权和版本锁。删除搭配不删除其商品与原照片。
- 运行 go test ./...、go vet ./...、node --check web/lookbook.js 以及 Windows 交叉编译。
- 重建并重启本地 Wardrobe MCP 进程，ChatGPT 插件刷新工具发现后才能看到新增的工具；只合并代码不等于运行进程更新。
