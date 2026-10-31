[English](CONTRIBUTING.md) · [首页](README_ZH.md)

# 贡献与开发约定

- 保持 `main` 随时可以发布；日常改动使用独立功能/修复分支与 Pull Request。
- 合并前执行 `go test ./...`、`go vet ./...`、`node --check web/app.js`；条件允许时运行 Windows 跨平台构建。
- 使用 Squash Merge 合并 PR，并在合并后删除**已合并**的分支；不删除未合并的分支。
- 禁止强推或删除 `main`。仓库公开后使用服务端分支保护；仅靠本地 Git hook 不够。
- 只提交源码、测试和彻底脱敏的示例。禁止提交 `Data/`、真实订单、数据库、照片、密钥、令牌或完整导出文件。
- 配置采用 TOML，业务主数据采用 SQLite，图片/大文件保持在本地并由 SQLite 引用。
- 未来投资和摄影模块（尚未实现）应使用独立的数据库和明确隔离的 MCP 权限。

## 分支保护与 CI

- `scripts/protect-main.sh` 用于为**公开仓库**的 `main` 配置 PR 合并、线性历史、禁止强推与删除。
- 2026-10-30：GitHub Actions 因账号账单设置无法正常运行；在恢复前**不要把 CI 设为必需检查**。
- CI 恢复并确认检查名称为 `test` 后，可通过 `REQUIRE_CI=1 sh scripts/protect-main.sh` 启用必需检查。
- 本机可使用 `git config core.hooksPath .githooks` 禁止直接推送 main；它不能替代 GitHub 服务端规则。

## 隐私与示例

个人身体数据、发型参考图、购买记录、交易快照、朋友圈照片、SQLite 与本地工作流覆盖数据只允许留在 `Data/`（已忽略）。

新增演示数据必须完全虚构，不应从本人的真实订单或用户画像抽样。Skill 文件只说明操作流程，不包含真实用户资料。

依赖升级必须通过 Go 漏洞扫描和必需 CI，不得通过修改分支保护跳过检查。
