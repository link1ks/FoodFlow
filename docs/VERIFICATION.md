# 验证范围与依据

验证结论属于运行时的源码与环境。当前提交是否通过，查看 [GitHub Actions](https://github.com/link1ks/FoodFlow/actions/workflows/ci.yml)；历史成功不能代替新版本检查。

## 复现检查

准备 Docker、Go 1.26、Node.js 24、pnpm 11、PowerShell 7，以及 Playwright Chromium：

```powershell
pnpm --dir web install --frozen-lockfile
pnpm --dir web exec playwright install chromium
go run ./cmd/harness -mode full -out .cache/harness/full.json
go run ./cmd/harness -mode acceptance -out .cache/harness/acceptance.json
go run ./cmd/harness -mode diagnose -out .cache/harness/diagnostics.json
```

| 范围 | 场景与依据 |
| --- | --- |
| 库存与权限 | PostgreSQL 事务、FEFO、守恒、并发、请求重放、个人/家庭越权；`internal/app` 与 `internal/core` 测试 |
| 菜单与采购 | 配方 BOM、份数、缺口、确认与烹饪扣库；`web/e2e/workflow.spec.ts` 与 `internal/app/family_recipes_test.go` |
| 社区 | 文字及封面初审、修改重审、积分一次发放、周榜与入库权限；后端、模型 fixture 与浏览器场景 |
| 账号与 AI | 恢复码、验证码用途、会话撤销、租约、调用前预扣、并发及不确定性处理；确定性模型/短信 fixture |
| 报表与营养 | 历史快照、金额覆盖、未知值、重量换算与 PNG 导出；后端集成与手机/桌面浏览器 |
| 平台 | Redis 故障回源、Kafka 中断与重复事件、独立投影、网关替换；`tests/platform.ps1` 与 `tests/gateway-recovery.ps1` |
| 部署与恢复 | 隔离环境权限、照片和数据库共同恢复、同版本重建；`tests/learning-security.ps1`、`tests/learning-recovery.ps1`、`tests/learning-release.ps1` |
| 工程约束 | 架构、源码清单、质量映射、sqlc/前端类型一致性、镜像绑定及更新脚本回归 |

`full` 要求 Docker 数据库集成，不能靠跳过获得通过。`acceptance` 使用独立数据和虚构账号，不清空开发数据；输出含源码摘要与镜像身份。详细命令和证据规则见 [Harness](HARNESS.md)。

## 已关闭技术债的依据

[质量台账](quality/domains.json) 中的以下条目引用本页。这里保留关键事实与回归定位，替代已结束的逐次执行日志；关闭状态不扩大实测范围。

| 条目 | 实现及可重复验证 |
| --- | --- |
| `INV-LOAD` | `internal/app/family_load_test.go` 的 `TestFamilyMixedLoadConservesStockLedgerAndOutbox`：10 厨房、20 客户端、30 秒，检查精确库存、幂等、流水与 Outbox 守恒，输出延迟、错误与锁等待采样 |
| `AUTH-ACCOUNT` | `internal/app/account_lifecycle_test.go`：确定性短信证明、账号换绑/合并、回滚、历史保留与会话撤销；`web/e2e/account.spec.ts` 验证不可用状态与确认流程 |
| `SHOP-PRICE` | `internal/app/monthly_report_test.go` 的 `TestMonthlyReportCostsCoverageAndScope`：采购金额、已知消耗/报损、未知覆盖、时区、重放与越权；浏览器验证 PNG 导出 |
| `HARNESS-IMAGES` | `cmd/harness/images_test.go`：拒绝旧摘要、错误项目、缺失或重复服务与未绑定标签；acceptance 核对 API/Worker/Web/Relay/Insights 的不可变镜像 ID |
| `API-COVERAGE` | `internal/app/openapi_test.go`、`internal/app/fixture_contract_test.go` 与 `internal/app/workflow_contract_test.go` 校验实际响应；`tests/generated-freshness.ps1` 拒绝过期生成结果；模型任务的开放 JSON 边界仍单独保留 |

上述家庭完善阶段的最终本地 full `e25c3cff0adc21340781b9a59a6f946e` 与 acceptance `e2e29d71168bd742924d8c96d3a51251` 对应摘要 `6e2c758851867498487c0992da10229a73c0a3b91b617a061a1e7e924f1ebffe`（2026-10-01）。该次负载 32,060 请求、0 错误/守恒失败、P95 9.864ms；仅进程内 API 与真实 PostgreSQL，不是公网容量或长期 SLO。

独立发布/恢复实现 `ec8a680c709456f1ac8f22f1de0b4172896e6111` 的 [CI 37602389753](https://github.com/link1ks/FoodFlow/actions/runs/37602389753) 已成功，含部署安全、配套恢复和同版本重建；不证明旧版本迁移兼容、公网部署或异机恢复。

个人厨房、社区初审与更新脚本实现 `984ac4535cb82f49daa48544a78a990f8bd65283` 的 [PR 验证 38210901917](https://github.com/link1ks/FoodFlow/actions/runs/38210901917) 与 [分支验证 38210898932](https://github.com/link1ks/FoodFlow/actions/runs/38210898932) 均成功。该次本地 full 15 项、acceptance 14 项及 36 个手机/桌面场景通过，使用确定性模型；2026-10-12 经 [PR #1](https://github.com/link1ks/FoodFlow/pull/1) 合并。这里的数量是该次运行记录，不是当前测试数量承诺。

## 未覆盖范围

真实模型和图片审核准确度、短信送达、S3、公网 HTTPS、生产高可用、长期负载与真实用户体验均需专项验收。应用费用预扣不等于服务商账单封顶；本地单次成功不证明生产容量、效率提升或用户收益。持续维护的风险见 [质量台账](QUALITY.md)。
