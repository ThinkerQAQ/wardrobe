# Third-party software / 第三方软件

The MIT license covers Wardrobe's original code, not its dependencies, retailer services, order data or product photography. Third-party software retains its own license and copyright.

Official release builds run pinned upstream `github.com/google/go-licenses/v2@v2.0.1` against the exact target package with the same GOOS/GOARCH and CGO setting as the binary. Each archive includes `THIRD_PARTY_LICENSES/`: complete upstream license artifacts (including any source required by a dependency), `DEPENDENCIES.csv` and the Go runtime license. A collector error fails release packaging. This inventory excludes optional private adapters, which must independently preserve their dependencies' terms.

Dependency versions are pinned in go.mod and go.sum. Core includes the MCP Go SDK, TOML parser, Go image packages, modernc SQLite and their transitive dependencies. The generated report and full texts describe the actual release; this summary does not replace those texts.

官方发行包按实际目标平台生成完整依赖许可、版本清单及 Go 运行时许可，位于 THIRD_PARTY_LICENSES。收集失败会阻止打包。MIT 仅覆盖原创代码，不授权第三方服务、订单数据或商品图片；私有适配器须独立处理依赖授权。数据库、个人照片和运行配置不属于发行包。
