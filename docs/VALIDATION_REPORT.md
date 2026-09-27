# 工程验证记录

验证日期：2026-09-27。所有数字仅描述本机和对应测试输入，不代表生产容量。

## 环境与复现

- Windows amd64，Intel Core i5-14600KF，Go 1.26，GOMAXPROCS=20。
- PostgreSQL 16；集成测试使用 Testcontainers 独立数据库。
- 本地部署采用 `compose.acceptance.yaml`，隔离数据卷与端口，不使用真实模型密钥。

```powershell
$env:REQUIRE_INTEGRATION='true'
go test ./... -count=1
go vet ./...
go test ./internal/engine/... -run '^$' -bench . -benchmem -count=3
pnpm --dir web test
pnpm --dir web build
```

## 已验证结果

- Go 全量测试通过，app 包耗时 63.883 秒；Docker 不可用时强制失败，未将跳过当作通过。
- `go vet ./...` 通过。
- 前端 3 个测试文件、8 个用例通过；TypeScript 与 Vite 生产构建通过。
- `cmd/loadcheck` 编译通过；负载工具只允许本机验收端口，每次创建独立测试家庭。

### 纯内存基准（3 次运行）

| 内核与输入 | 耗时 ns/op | B/op | allocs/op |
| --- | --- | --- | --- |
| Matcher：128 个食材、100 个菜谱、选择 64 个食材 | 730.9 / 732.8 / 724.8 | 0 | 0 |
| 多餐规划：1 菜谱、1 批次、3 餐 | 1898 / 1905 / 1904 | 0 | 0 |
| 工序排程：4 个步骤 | 162.5 / 159.7 / 177.0 | 0 | 0 |
| 多菜组合：12 菜谱、12 批次、60 分钟预算 | 15517 / 16416 / 19653 | 6040 | 56 |

上述数据不含查询、JSON、鉴权或网络耗时；多菜组合当前仍有堆分配。微秒级匹配不等于微秒级 API 响应，小输入结果不能外推到大规模搜索。

## 故障与一致性边界

数据库集成测试覆盖并发扣库、幂等键重放、跨家庭访问、租约过期与旧 Worker 写入拦截、取消后阻止写入、事务回滚。租约测试使用可控数据库状态；它与真实进程强杀、主机断电、网络分区是不同层次的验证，不能混为一谈。

模型外部调用没有 exactly-once 承诺，网络超时可能产生费用但没有收到结果。业务写入通过事务、幂等记录及租约校验保护。

## 部署验收状态

- 独立 Compose 镜像构建成功，27 个迁移完成，API/Worker/Web/数据库正常运行。
- Smoke 两次通过：注册、家庭隔离、邀请、幂等入库、Worker 菜单、采购入库、烹饪扣库。
- 重启 API/Worker/Web 后 ready 返回 200，随后再次通过主流程。
- PostgreSQL 自定义格式备份成功，恢复到全新数据库；不可变流水逐行 JSON 聚合指纹、用户数、迁移版本与源库一致。验收恢复过程暂停 API/Worker，避免并发写入影响对比。
- 图片卷 tar 备份已生成；本轮没有图片对象恢复与 S3 验证。
- `tests/worker-recovery.ps1` 通过：SIGKILL 独立 Worker，停机期间创建任务保持 queued，重启后恢复为待确认草稿。运行中任务的租约 fencing 由数据库集成测试单独验证；未模拟模型调用途中强杀。
- 完整源码已同步 GitHub；[CI #1](https://github.com/link1ks/FoodFlow/actions/runs/36311723228) 三项全部成功：backend 6 分 9 秒、frontend 32 秒、images 1 分 53 秒。后端执行 `go vet`、`go test -race`、强制数据库集成测试与 sqlc 生成一致性检查，并上传覆盖率产物。对应源码提交 `74c0c89`。

### HTTP 负载冒烟

同一台主机运行客户端及 Docker Desktop 服务。每组新建 1 个家庭、20 种食材与 20 个批次，10 次预热，500 次带鉴权库存 GET；闭环并发模型。

| 并发 | 请求/秒 | P50 ms | P95 ms | P99 ms | 最大 ms | 失败 |
| --- | --- | --- | --- | --- | --- | --- |
| 1 | 663.99 | 1.554 | 2.092 | 2.573 | 3.328 | 0/500 |
| 8 | 3355.32 | 2.102 | 3.202 | 9.524 | 14.037 | 0/500 |
| 32 | 4855.49 | 5.772 | 11.549 | 21.063 | 25.211 | 0/500 |

```powershell
go run ./cmd/loadcheck -requests 500 -concurrency 1 -out .cache/load-c1.json
go run ./cmd/loadcheck -requests 500 -concurrency 8 -out .cache/load-c8.json
go run ./cmd/loadcheck -requests 500 -concurrency 32 -out .cache/load-c32.json
pwsh -File tests/worker-recovery.ps1
```

每组持续约 0.10–0.75 秒，仅用于验证负载工具与 API 路径；数据量小且缓存热，不覆盖持续压力、资源饱和、多家庭混合读写或真实网络。不能将瞬时吞吐当作稳定 QPS、SLO 或生产性能承诺。原始脱敏结果保存于 `docs/validation/`。

复现操作见 [本地部署验收](LOCAL_DEPLOYMENT.md)。公网部署、真实短信送达、S3 与生产流量尚未验证。
