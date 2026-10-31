---
name: wardrobe-profile
description: 查询或维护本地 Wardrobe 身体尺寸、成衣参考及发型库；用于尺码核对、发型搭配和用户明确提供的新测量值。旧笔记不是当前测量，只有必要时才向云端 AI 返回个人数据。
---

# 身体尺寸、发型资料

- 首先 `get_wardrobe_body` 获取 `BODY` 的 `measurements`（人体净围）、`garmentTargets`（成衣参考）及来源时间。来源文件修改时间与实际测量时间不一定相同；实际测量时间未知时必须明确说明。对测量方法含糊的字段保留原称并注明待核实。
- 不要混淆身体净胸围与合身短袖的成衣胸围，也不要自动从历史尺码推断新购服装尺码；以商品尺码表及用户反馈为准。
- 仅用户明确报告重新测量值时，通过 `update_wardrobe_body` 更新指定测量项目，必须传当前 `version`；服务端记录变化历史，不能覆盖原笔记来源。
- 发型库通过 `list_wardrobe_hairstyles` 检索，通过 `get_wardrobe_image(kind="hairstyles",id="HAIR....")` 看原图。发型是本地 JPG 参考，不属于商品；不可将其当作真人身份识别。
- 尺寸、发型和图片的**主数据**保存在本机 SQLite / `Data/hairstyles`，不得提交到 Git。只有用户授权、确实需要时才通过 MCP 返回相关尺寸或照片；若使用云端 ChatGPT，返回的数据会发送给 AI 服务，不能声称完全不出本机。照片不能代替真正的测量结果。
