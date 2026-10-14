# FoodFlow 全面测试与问题清单

## 真实百炼验证补充（2026-10-14）

五张公开食材图与五段合成中文录音的真实调用均生成待确认草稿；五个语音同键重放没有重复收费发送，库存/流水/Outbox 均未写入。输入、返回用量、约 0.0052 元的折算与账单边界见 [百炼报告](BAILIAN_VALIDATION.md)。下文原始审计中“未配置/未调用”仅代表当时状态。

新复现并修复两处：图片草稿返回英文，已加入简体中文要求与请求契约回归；“一点五千克”吞掉单位中的“千”，已限制中文小数部分仅含数字，回归精确 1500.000 g。新提示词未追加收费复测。苹果个数不能换算目录重量而留空是预期保护。外部 S3、物理麦克风/噪声、代表性准确率和实际账单仍未验收。
## 最新修复状态（2026-10-14）

本次已按用户选择修复数量校验、完整食谱用料读取，并完成依赖安全升级。以下状态对应当前修复工作树，源码摘要 `6c34cf812c50204b7be4f09a42a39a692f8072b9cf2b9e82a5c822e27f849146`；下文原始审计保留其原版本和未修复时的复现事实。

| 问题 | 当前状态 | 修复与验证 |
| --- | --- | --- |
| QA-QTY-01 | 已修复 | 整数/小数部分均只接受 ASCII 数字；非法输入返回 400，食材/批次/流水/Outbox 均不写入；有效三位小数与幂等重放精度保持。正式解析/API 回归及任意字符串 fuzz 已加入 |
| QA-RECIPE-02 | 已修复 | 先限 20 个 recipe IDs，再完整读取用料，不再对联表行设 400 上限。超过 400 条用料的回归证明缺末项时不推荐，补齐后分配全部 30 项 |
| QA-SEC-01 | 已升级并完成当前公告复核 | Go/Docker 构建最低 1.26.9；pgx 5.9.2、S3 1.97.3、EventStream 1.7.8、compress 1.18.7、jsonparser 1.1.2、x/net 0.60.0、x/crypto 0.57.0、x/text 0.42.0。当前源码扫描受影响导入包/可达调用均为 0，实际 API 二进制受影响符号为 0 |
| QA-SEC-02 | 已升级，公告告警为 0 | Vitest 4.1.11 移除旧 tinypool 依赖链，source-map-js 锁定 1.2.2；生产及开发依赖合并扫描为 0 告警 |
| QA-VOICE-01 / QA-RECIPE-01 / QA-VOICE-02 | 仍开放 | 语音否定、候选相关性过滤顺序、过期录音保留不在本轮所选修复范围，保留原回归探针 |

最终 full `f338f9cbf068bd2be3c40f9fccc8b435` 15/15、acceptance `43e9ced12fb58542f1adeae5adb6b5bc` 14/14、38/38 桌面/移动端 Edge 场景通过，0 跳过/重试。额外数量/事务/存储/算法包竞态检测通过，最终 diagnose 5/5 通过。新数量语法 fuzz 15 秒、2 worker、877,121 次执行通过。正式回归在 `internal/core/core_test.go`、`internal/core/properties_test.go`、[API/完整 BOM 回归](../internal/app/quantity_bom_regression_test.go)、[本地 S3 SDK 协议回归](../internal/storage/sdk_upgrade_test.go)；它们在正常 full/CI 测试中执行，原始审计的未纳入 CI 描述只适用于旧 `.cache/audit` 探针。

最终证据为 `.cache/harness/quantity-bom-deps-final-{full,acceptance,race,diagnose}.json`、`quantity-bom-deps-npm.json`、`quantity-bom-deps-final-govuln-summary.json`、`quantity-bom-deps-runtime-vuln-summary.json`。Go 模块层仍提示 GO-2026-5932：x/crypto 中已停维护的 OpenPGP 包；项目没有导入该包，未发现受影响调用/二进制符号，该公告也无修复版本。扫描 0 可达漏洞不等于模块公告记录全为空，也不证明未知漏洞不存在。

修复已应用本机 5173；更新前生成新的私密数据库/照片配套备份，暂停写入后前后业务表、序列与照片指纹一致，schema 仍为 36、无新增迁移。网关/健康均 HTTP 200，实际 API 编译版本与上述依赖一致。未改 `.env`、未自动修正历史库存，也未调用真实模型、短信或 S3。新修复的云端 CI 尚未建立，原始审计中的旧提交 CI 不能证明本次工作树。

## 原始审计（修复前）

审计日期：2026-10-14（北京时间）。代码版本：`64958e8932d895b8c72358ea6b080c57ca2bd0bd`；业务源码摘要：`e8e923f076a55a1c5cf802b5451b8c6ebd5a2a754300cdb8a24bad778bee4c9f`。本次是测试与审计，未修复下列业务问题，也未修改依赖、用户配置或默认部署数据。

## 结果与证据

| 项目 | 实测结果 | 范围与本地证据 |
| --- | --- | --- |
| 完整 Harness | 15/15 通过 | `full`，运行 `8459cbeaefe0815b72eaa4f140ae9401`，`.cache/harness/comprehensive-audit-full.json`；真实隔离 PostgreSQL 集成、格式、vet、架构、质量映射、sqlc、API 类型、前端 13 项单元测试和生产构建 |
| 平台及浏览器 | 13/13 通过；38/38 浏览器场景通过，0 跳过、0 重试 | `acceptance -skip-build`，运行 `45844944fef94cc2b82ed9d1503dbffa`，`.cache/harness/comprehensive-audit-acceptance.json`；复用当前摘要绑定镜像，包含 Redis/Kafka 故障恢复、投影去重、网关替换；桌面及移动端使用 Edge 和虚构账号 |
| 运行诊断 | 5/5 通过 | 运行 `b08caa2240adca7af21c8926b6d5c4d2`，`.cache/harness/comprehensive-audit-diagnose.json`；脱敏健康、任务、Outbox、投影、Kafka 延迟快照 |
| 家庭混合负载 | 23,562 请求；0 错误、0 守恒失败；P95 18.916ms、P99 70.177ms | `.cache/harness/family-load.json`，10 厨房、20 客户端、30 秒；进程内 API + PostgreSQL，不含公网网关、模型或长期负载 |
| 数量往返 fuzz | 15 秒通过，3,204,737 次执行 | 仅探索该属性生成的正数输入，不涵盖所有字符串语法 |
| 换算与份数守恒 fuzz | 15 秒通过，3,897,230 次执行 | 覆盖兼容/不兼容单位与舍入属性 |
| 排程库存守恒 fuzz | 降低并行度后 15 秒通过，574,234 次执行 | 默认 20 worker 首次运行完成 2,859,328 次执行后，在结束阶段报 `context deadline exceeded`，无失败输入或守恒断言；`-parallel 2` 复核通过，保留为测试环境稳定性记录，不能据此宣称算法缺陷 |
| 生产前端依赖审计 | 公告告警 0 条 | `pnpm --dir web audit --prod --json`，`.cache/harness/comprehensive-audit-npm.json`，仅代表扫描时数据库已知公告 |
| 完整前端依赖审计 | 5 个包/公告条目：2 critical、1 high、2 moderate | `.cache/harness/comprehensive-audit-npm-all.json`，涉及 4 个独立公告，全部在构建/测试依赖链 |
| Go 源码安全扫描 | 33 个公告有静态调用路径 | `.cache/harness/comprehensive-audit-govuln-compact.json`；本机 Go 1.26.0，30 个属于旧本机标准库，不能套用到已使用 Go 1.26.9 的当前容器 |
| 实际 API 二进制安全扫描 | 15 个版本匹配的公告，其中 8 个存在受影响符号 | `.cache/harness/comprehensive-audit-live-api-vuln-summary.json`；二进制扫描不提供完整调用链或攻击可达性证明。当前 API 编译版本确认为 Go 1.26.9 |
| 当前提交云端 CI | 4 个 job 全部成功 | [CI 38379664177](https://github.com/link1ks/FoodFlow/actions/runs/38379664177)，覆盖 Linux 竞态检测、容器构建、平台与学习环境恢复/同版本发布；与本机实测分开计证据 |

额外本机全量 `go test -race -count=1 -json ./...` 通过：17 个有测试包、104 个顶层测试通过，包含子测试共 177 个 pass 事件；0 测试跳过、0 失败、未报告竞态。环境清除了真实数据库/服务/模型覆盖，强制隔离 PostgreSQL 集成，使用 MinGW、`CGO_ENABLED=1`；证据 `.cache/harness/comprehensive-audit-race.json`。空测试包的 package-level skip 不计为业务场景跳过。

本机独立学习部署另行通过安全与配套恢复验收：`.cache/harness/learning-security.json`、`.cache/harness/learning-recovery.json`。恢复检查 58 张表、2 个图片文件，包含密码登录、旧 token 拒绝、300g 库存、照片 SHA256、原幂等重放、新写入与损坏备份拒绝；恢复目标保留并停止。初始化了新的受保护学习配置，未覆盖默认 `.env`；真实用户数据不进入该环境。学习镜像首次不存在时，Compose 的 registry pull 返回 403，随后本地构建与启动成功，属于非致命环境反馈。当前提交的同版本发布已由云端 CI 验证，本次不把它计为新增本机发布演练。

## 已复现的业务问题

P1 表示优先处理，P2 表示下一轮应处理；这是修复优先级，不是漏洞 CVSS 分数。以下五个问题全部在确定性输入或一次性数据库中复现。

| ID | 优先级 | 复现与实际结果 | 影响及边界 | 定位与建议 |
| --- | --- | --- | --- | --- |
| QA-QTY-01 | P2 | 输入 `1.-1` 得到 `0.990`；`1.+1` 得到 `1.010`；`0.+1` 得到 `0.010`。通过 `POST /catalog-stock` 提交 `quantity: "1.-1"` 返回 HTTP 200 并创建库存 | 非法数量语法进入精确库存基础层。不是浮点误差；是整数/小数片段分别用 `ParseInt`，允许小数部分出现符号。常规 round-trip fuzz 只产生合法正数，未覆盖本例 | [core.Quantity](../internal/core/core.go)，第 24 行起。先严格验证十进制语法，再进行定点解析；回归覆盖所有入库、出库、语音确认等复用入口 |
| QA-VOICE-01 | P2 | `别加番茄500克`、`番茄500克去掉`、`番茄500克取消` 都产生 `500.000 g`，没有条目级警告 | 否定/撤回表达被正向预填；用户仍须勾选核对并确认，识别本身不会改库存。全局核对提示不能替代本项否定警告 | [intake.Parse](../internal/engine/intake/intake.go)，第 125 行只覆盖部分关键词。补充明确否定、撤回表达并把不确定项数量留空；复杂多轮口语仍作为产品边界 |
| QA-RECIPE-01 | P2 | 一道有库存的番茄菜原本可以推荐；增加 20 道更快、但与番茄无关的菜后，同一请求推荐变空，`truncated=false` | 先取 20 道再按所选食材过滤，导致用户有食材、有菜谱却得不到推荐。当前小种子库不一定触发，社区收录扩充食谱后即可出现 | [clearFridge](../internal/app/clear_fridge.go)，第 78 行。按选中食材筛选、校验后再限制完整候选；明确说明候选上限 |
| QA-RECIPE-02 | P2 | 13 道各 30 项用料的不可行菜排在前面，第 14 道菜也是 30 项用料且缺少其中 1 项；接口仍推荐第 14 道，只返回 10 项批次分配，`truncated=false` | SQL `LIMIT 400` 限的是联表行，会截断一道菜的用料。当前社区输入允许最多 30 项，规模处于允许范围。后续菜单确认/烹饪会重算完整用料，因此本例证明错误的可行性推荐，不证明实际扣库越界 | [clearFridge SQL](../internal/app/clear_fridge.go)，第 61 行。先限制 recipe IDs，再完整读取各菜用料；不允许部分 BOM 进入排程 |
| QA-VOICE-02 | P2 | 模拟 Worker 中断、语音任务租约过期后，`claim` 把任务置为 `failed`，原暂存音频仍存在 | 原始录音缺少自动保留期/清理机制；失败任务积累存储和隐私保留风险。这是语音指南已有的恢复边界，本次增加了确定性复现。没有重新发送收费请求 | [claim](../internal/app/jobs.go)，第 209 行；[语音清理](../internal/app/voice.go)，第 101 行起。按终态/年龄/活动引用建立有界清理，清理失败需可观测，不删排队/运行任务的对象 |

回归探针位于本机忽略目录 `.cache/audit/known_issue_regressions_test.go`，Go overlay 为 `.cache/audit/overlay.json`；脱敏结果 `.cache/harness/comprehensive-audit-regressions.json`。六个顶层探针失败对应上述五个业务问题，其中数量问题同时验证了纯解析器和入库 API。

复测命令（不得设置为真实开发数据库）：

```powershell
$env:REQUIRE_INTEGRATION='true'
Remove-Item Env:TEST_DATABASE_URL -ErrorAction SilentlyContinue
go test -overlay .cache/audit/overlay.json ./internal/app -run '^TestAudit' -count=1
```

当前这些探针有意断言期望的正确行为，所以会失败；已保存用于后续修复，但尚未纳入正常 CI。正常 full/acceptance 通过不代表这些问题已解决。

## 安全维护问题：已匹配公告，尚未复现攻击

### QA-SEC-01：后端依赖需升级（P1 维护优先级）

源码调用图、实际二进制与公告三者交叉核对，存在旧版依赖及受影响符号。没有攻击实际用户数据，也没有把符号存在等同于当前部署可被利用。

| 依赖 | 当前版本 | 公告/修复范围 | 当前适用边界 |
| --- | --- | --- | --- |
| pgx/v5 | 5.7.2 | [GO-2026-5004](https://pkg.go.dev/vuln/GO-2026-5004)，修复 5.9.2；另有二进制公告 GO-2026-4771 | SQL 注入公告要求非默认 simple protocol、美元引号和可控占位参数；本仓库未找到启用 simple protocol 的代码，不报告为已复现 SQL 注入 |
| AWS S3 / EventStream | S3 1.84.0，EventStream 1.6.11 | [GO-2026-5764](https://pkg.go.dev/vuln/GO-2026-5764)，S3 1.97.3 / EventStream 1.7.8 | 涉及 EventStream 解码；默认本机使用 local storage，真实 S3 路径未验收 |
| x/text | 0.40.0 | [GO-2026-6629](https://pkg.go.dev/vuln/GO-2026-6629)，修复 0.41.0 | secure/precis 受影响符号可达；特定恶意输入 panic 未在当前认证/数据库连接条件下复现 |
| x/net | 0.57.0 | GO-2026-6603 / 6611 / 6612 / 6617，修复 0.60.0 | 二进制有 HTTP/2 受影响符号；当前 API 为明文 HTTP/1，网关另行终止连接，不能据此断言外部用户可触发 HTTP/2 漏洞 |

本机开发 Go 1.26.0 还应升级补丁；当前容器使用 1.26.9，扫描中的旧标准库告警不属于该容器。依赖升级需要兼容性回归，本次不执行升级。

### QA-SEC-02：前端构建/测试依赖告警（P1 维护优先级）

| 依赖链 | 公告严重度 | 官方公告与建议 |
| --- | --- | --- |
| Vitest 及 @vitest/mocker | moderate，两个包条目对应同一公告 | [GHSA-82fw-gwwq-j7x9](https://github.com/advisories/GHSA-82fw-gwwq-j7x9)，路径穿越/文件读取；公告修复范围 >=4.1.11 |
| Vite/PostCSS/Tailwind → source-map-js | high | [GHSA-68fv-2mgg-jv7q](https://github.com/advisories/GHSA-68fv-2mgg-jv7q)，特定 indexed source map 导致事件循环拒绝服务；修复 >=1.2.2 |
| Vitest → tinypool | critical，两个不同公告 | [GHSA-5gmw-xhrv-c9v3](https://github.com/advisories/GHSA-5gmw-xhrv-c9v3)、[GHSA-85c8-ppgw-ccpr](https://github.com/advisories/GHSA-85c8-ppgw-ccpr)，原型污染 gadget 到代码执行；均覆盖的修复范围 >=2.1.2 |

这些属于开发/测试/构建工具，当前 Nginx 静态生产站点不运行 Vitest。`pnpm audit --prod` 为 0，不能把开发依赖的 critical 自动描述成“网页已存在远程执行漏洞”。需要复核真实污染入口/工具暴露方式后判定可利用性，并升级依赖后重跑测试。

## 配置情况及已知验收缺口

本机语音入口现已部署，但独立百炼 endpoint/key 未配置，`AI_ALLOWANCE_ENABLED` 关闭。真实语音识别不可用是当前配置状态，不是本次新增代码缺陷；本次不读取/输出密钥，不启用收费调用。

[质量台账](quality/domains.json) 仍有十条 open 技术债，不能因测试通过而关闭：

| 台账 ID | 仍需验证或建设 |
| --- | --- |
| INV-VOICE-REAL | 实际百炼识别、硬件麦克风、噪声/方言及录音恢复；录音保留问题本次已额外复现 |
| AUTH-SMS-DELIVERY | 真实短信送达、邮箱所有权验证；离线恢复码已存在 |
| COMMUNITY-ABUSE | 多账号刷票、举报流程、真实社区规模负载 |
| AI-EVAL | 图片识别准确率、菜单可用性的代表性评估 |
| AI-INVOICE | 应用预扣额度与供应商实际账单/价格变化的对账 |
| PLATFORM-OPS | 生产告警、传输安全及投影重建；本地故障恢复通过不替代生产验收 |
| NUT-COVERAGE | 可食质量换算、深色蔬菜分类覆盖 |
| NUTRITION-REFERENCES | 部分品种缺少核实营养资料，未称重的质量库存仍是估算 |
| UX-TRIAL | 真实家庭首次使用、录入负担及回访留存，没有真实用户数据 |
| API-JOB-SCHEMAS | 不同模型 job payload/result 仍是开放 JSON，契约精细化不足 |

没有实测 Safari/Firefox、真实手机设备、外部 S3、真实收费模型/短信、公网 HTTPS、异机恢复或长期生产压力。桌面/手机是 Edge 的视口与设备模拟。默认 5173 服务和真实数据不用于测试写入，所有新增账号/库存/音频都属于一次性数据库或隔离验收/学习部署。

优先修复顺序：依赖安全维护与严格数量解析、完整 BOM 读取、候选筛选顺序、语音否定/撤回处理、录音保留治理。真实 AI 质量和用户留存应安排独立验收，不能由工程测试代替。
