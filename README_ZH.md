# Wardrobe · 我的衣橱

[English](README.md) · [安装](docs/SETUP.zh-CN.md) · [MCP](docs/MCP.zh-CN.md) · [安全](SECURITY_ZH.md)

基于 **Go + SQLite + Web UI + MCP** 的本地优先个人衣橱与穿搭管理应用。

- 衣物、穿搭、发型、身体数据、图片和 Skill 保存在本机。
- Web UI 与 Edge/Chrome 侧边栏复用同一套本地数据。
- MCP 允许按需授权 AI 查询与操作本机衣橱。
- 支持统一订单明细模型、CSV/XLSX/JSON 本地文件导入、去重、分类和穿搭候选整理。
- 可选数据源适配器在独立环境组合构建。**公开 Core 不包含电商订单页面监听、专用解析或翻页采集代码。**
- 公开版本无需访问私人仓库即可独立构建、测试和使用。

## 安装

使用[公开 Releases](https://github.com/ThinkerQAQ/wardrobe/releases) 或在本地执行 `go run .`，详见[安装指南](docs/SETUP.zh-CN.md)。

## 隐私与合法使用

个人 SQLite、商品图片、照片、令牌及配置存储在本机 `Data/`，不得上传公开仓库。连接云端 AI 的 MCP 调用可能传输用户明确授权查询的字段或图片。

本项目与任何电商平台均无官方关联。用户应有权使用相关账号、数据与图片，并遵守适用法律和第三方服务条款。源码采用 [MIT License](LICENSE)，不代表获得第三方平台、商品数据及照片的授权。

## 开发

```bash
go test ./...
go vet ./...
go build .
```

订单标准数据接口及可选适配器说明见 [docs/ORDER_SOURCES.md](docs/ORDER_SOURCES.md)。

发行包附带第三方完整许可及版本清单，详见[第三方声明](THIRD_PARTY_NOTICES.md)。
