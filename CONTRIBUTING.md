# 开发与交付

FoodFlow 的模块归属见 [仓库导航](AGENTS.md) 和 [服务边界](docs/PLATFORM.md)。修改库存写入时，必须保留家庭权限、精确数量、请求幂等以及库存、不可变流水、Outbox 的单事务边界。

## 本地工作流

使用 Go 1.26、Node.js 24、pnpm 11、PowerShell 7 和 Docker Compose。启动与配置见 [README](README.md)，独立部署见 [部署说明](docs/LEARNING_DEPLOYMENT.md)。

```powershell
pnpm --dir web install --frozen-lockfile
pnpm --dir web exec playwright install chromium
go run ./cmd/harness -mode task -task example-change -action start -kind maintenance -baseline unknown
# 修改源码；接口、SQL 或配置变化后刷新并审查生成结果。
go run ./cmd/harness -mode knowledge -update
go run ./cmd/harness -mode full -out .cache/harness/example-full.json
go run ./cmd/harness -mode acceptance -out .cache/harness/example-acceptance.json
go run ./cmd/harness -mode task -task example-change -action finish -outcome passed -evidence .cache/harness/example-full.json
```

任务名称必须唯一。失败也应记录真实结果；完整规则见 [Harness](docs/HARNESS.md)。验证过程中保持源码不变，执行计划记录命令、结果与限制。数据库测试和浏览器场景使用隔离环境；默认使用确定性模型，不调用付费服务。

## 提交范围

提交源码、锁文件、接口契约、数据库迁移、必要的测试夹具、授权资产、部署脚本和可复现的脱敏证据。生成代码必须与源契约一致。历史迁移不改写，新增迁移向前演进。

环境配置、密钥、数据库备份、家庭照片、数据卷、构建产物、依赖目录、IDE 配置、临时下载和原始认证日志留在 Git 忽略的本地目录。公共参考照片必须保留 [来源与许可](web/public/ingredient-photos/credits.html)。仓库文件检查拒绝常见私人配置和生成产物路径；它不替代内容审查或密钥检测。

提交前检查 `git diff --check`、`git status --short` 和待提交差异。变更说明写明问题、最终行为、验证与剩余风险。安全问题按 [安全说明](SECURITY.md) 私下报告。
