---
name: wardrobe-aesthetics
description: 基于用户本地 SQLite 的审美依据、历史尺码记录和用户明确的穿着反馈，进行服饰筛选、搭配与风格判断。用于查看个人审美、审美规则、穿搭推荐或讨论服装取舍。
---

# 个人审美和判断依据

**唯一数据源是 Wardrobe Local 的 SQLite。不要从 README、Google Sheets、JSON 快照或本 Skill 猜测或复写历史记录。**

## 数据读取

1. 使用 `get_wardrobe_rules` 读取最新的用户审美规则和原始说明。或调用 `get_wardrobe_workflow(name="wardrobe-aesthetics")`，返回本流程以及同一份 SQLite 的实时规则快照；不依赖前端「审美与说明」页面。
2. 通过 `search_wardrobe` / `get_wardrobe_product` 验证具体款式的颜色、版型、面料、已购规格及持有/合身状态；看图时调用 `get_wardrobe_image`，没有图不能臆测。
3. 人体净尺寸与理想成衣尺寸应通过 `get_wardrobe_body` 单独查询，原笔记数据如未确认当前有效，就标为历史参考。

## 如何理解规则

- 区分三类：用户明确表达的喜好、早期系统/表格总结与未核实历史资料。历史「推荐/建议/统计」不自动升级为用户本人认可偏好。
- 旧资料中静态商品总数、旧平台同步流程、历史尺码统计等仅作出处证据，不作为当前数量或操作要求。当前数据量用 `audit_wardrobe` 或搜索结果统计。
- 穿着适配遵循「颜色与风格 → 轮廓版型 → 尺寸与场景 → 单品组合 → 用户反馈」；先以实际衣橱为依据，不把理论审美当成穿过后的真实体验。
- 用户给出的当前偏好与反馈优先于历史默认分数。不得擅自更改评分、持有、合身、审美结论。

## 数据位置与保密

审美记录保存在本机 SQLite 的 `records(kind=notes)` 中，不存放到 Git 仓库的 Skill 文件。调用 `get_wardrobe_workflow(name="wardrobe-aesthetics")` 时，当前实现会在响应中**附带 SQLite 的全部审美 notes**；若 MCP 连接到云端 ChatGPT，这些记录也会随响应发送至 AI 服务。应仅在用户确实需要个人审美分析时调用，且不得将原始记录上传到 GitHub、公开文档或无关服务。
