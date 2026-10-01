# FoodFlow 家庭试用版完善与 GitHub 同步

开始：2026-10-01（Asia/Shanghai）。用户授权逐步完善项目并同步 GitHub；目标为本地与家庭试用版，明确不调用付费模型。

## 交付顺序

1. 同步已验证的契约、浏览器成功链、质量台账与真实食材照片；确认对应 GitHub CI。
2. 扩充自编家庭菜谱，保持精确 BOM、目录关联、份数、步骤与调味估算一致。
3. 实现金额与浪费月报、历史成本覆盖说明、长图导出，保留流水与成本快照。
4. 完善账号生命周期，使用确定性短信 fixture 验证权限、验证码、撤销会话与确认边界。
5. 补齐可确认的营养换算/分类与剩余 API 契约、生成前端类型。
6. 绑定验收镜像与源码，补家庭规模负载验证与可操作的试用指南；更新质量台账、启动环境与 GitHub。

每个代码阶段单独记录 harness task，执行 full 与相关 integration/acceptance，写入实际结果后提交同步。GitHub CI 结论只能来自对应提交的实际运行，不将本地通过写成云端通过。

## 范围边界

真实 AI 质量、短信送达、S3 和公网生产部署不在本轮实测范围。家庭成员是否持续使用必须通过真实试用获得，不能由自动测试替代。生产高可用、安全与长期 SLO 债务保留，不凭本地配置关闭。

## 阶段记录

- 基线同步：任务 `completion-baseline-20261001` 已开始；此前完整本地交付见 [quality-photos](quality-photos.md)。当前无远端新增提交，未覆盖既有工作区或数据。
- 首次基线 full 失败：Docker Desktop 引擎未运行，所有必需数据库场景明确失败；已将任务记为 infrastructure 失败。通过已安装的 Docker Desktop 恢复引擎，不删除容器或卷；重试任务 `completion-baseline-r1-20261001` 已开始。
- 配置只读核对确认本地保留 DeepSeek `deepseek-flash` 密钥配置；密钥未打印或提交，未发起模型请求。费用咨询不改变本轮禁用付费调用的授权边界。
- Docker 恢复后重试 full：9/9 通过，运行 `e7fd6cd441666625ee217c66c2a1af3c`，源码摘要 `da54d131bfd20e1f7a1213dedc2805ef238f8b2ea4779cfdf54d5e29324c9a2e`；重试任务已记录 passed。此阶段仅补架构说明与执行计划；运行时验收依据此前 quality-photos 记录，未把文档复核冒充新浏览器验收。
- 用户确认 Gemini 是 Google AI Pro 网页会员；未新增 API 密钥或启用模型调用。
- 基线已正常提交并推送 main：`321524b373055f2e79b911bdbe72a6088e59ebbd`。[GitHub CI](https://github.com/link1ks/FoodFlow/actions/runs/36821811211) 已启动；最近核对 images/frontend 成功，backend/platform 仍运行，未声明全部通过。

### 家庭菜谱

- 任务 `family-recipes-20261001`：增加 029 迁移的 12 道自编两人份参考菜谱，合计 16 道。步骤使用目前可运行的案板/灶台，份量和计时均明确为估计；不以计时结束证明熟透。南瓜米粥不虚构油盐需求。
- 手工选菜新增原始份量、BOM、步骤与来源预览；人数换算仍由服务端执行。新增浏览器场景检查 3 人份菠菜 450 g、蒜 15 g 的采购缺口和确认前后库存不扣减。
- 定向集成 `TestFamilyRecipesBOMAndScheduling` 12/12 通过，验证目录与单位关联、正数量、步骤耗时一致、调味估算、真实菜单确认/管线与无库存写入。fixture 初次缺少确认幂等键并复用餐次，已修正为保留实际业务保护的请求。
- 首次 full 8/9，旧 `TestWorkflowAndInvariants` 假定 10 分钟内排除生菜后没有菜可做；新增菠菜让合法替代存在。按 fixture 类别记录失败并开始 `family-recipes-r1-20261001`。回归改为分别验证替代不含排除食材，以及排除全部可选菜时拒绝，未修改业务授权或排除逻辑。
- 第二次 full 8/9，`TestIngredientPhotosAndRecipeOptions` 把所有番茄匹配数量固定为 1；新增番茄豆腐汤正确产生另一个匹配。记录 r1 为 fixture 失败，开始 r2；将原场景定位到番茄炒蛋 ID，保留原有缺口、份数和单位冲突断言。上述两条定向回归通过后再次 full。
- 最终 full 9/9：运行 `c5f78cc83c5ae746dbe954d89fb62e0f`；重建验收镜像后 acceptance 11/11，浏览器 7 场景 × 桌面/手机 = 14 通过，0 跳过、unexpected、flaky；full/acceptance 同摘要 `c1fa906a9ce40f7b967c0d50d3f03cf1e0c4fcc91cfa0b7269029a53672e643f`。`family-recipes-r2-20261001` 已记录 passed。便携证据：[family-recipes](../validation/family-recipes-20261001.json)。
- 基线提交 `321524b` 的 [CI 36821811211](https://github.com/link1ks/FoodFlow/actions/runs/36821811211) 已 completed/success，backend、frontend、images、platform 四项均成功且无失败步骤。
- 本阶段关闭 MENU-RECIPES 的扩充债务；真实烹饪体验、模型效果和家庭使用率仍需实际试用。金额月报、账号生命周期、营养覆盖、契约类型、镜像绑定和持续负载尚未在本阶段交付。

### 金额月报

- 任务 `family-monthly-report-20261001`。金额仍由厨房主库拥有，不向独立 Insights 数据库开放厨房表。单条 SQL 一致读取家庭时区月窗内库存事实与不可变成本快照；金额采用 PostgreSQL numeric 聚合后的两位小数字符串，数量按历史名称/单位分组。
- 分别显示“本月记录采购金额”（按成本录入日期）、已知消耗和报损成本、已定价/未知成本出库条数与估算扣减。补记成本不重算之前的出库，不用当前参考价代替实付，不虚构节省金额。接口明细上限 200 组，总计包括全部月内事实。
- Records 新增金额月报与 PNG 下载。导出保留已知小计/未知覆盖/截至时间/时区/明细限制，每页最多 40 组，用户可选择导出页；全部金额总计在每页展示。数量用 BigInt 精确格式化，补充超过 JavaScript safe integer 的回归。
- `TestMonthlyReportCostsCoverageAndScope` 通过：未知历史、¥5 采购基数、已知消耗 ¥1、已知报损 ¥1.50、3 条出库其中 1 条未知、重复报损无新增、历史改名不改变旧事实、实际响应标准 OpenAPI 校验、家庭越权拒绝、上海时区月窗两端与校准不计成本。初次测试编译使用了 Reader 而契约验证器要求 ReadCloser，已改为 NopCloser；没有运行或消耗真实模型。
- 初次 acceptance 10/11（14 浏览器通过、2 失败）：月报 fixture 已有库存，不显示空厨房指南，而测试误点该按钮。记录任务为 fixture 失败并开始 r1；修正后定向桌面/手机 PNG 导出 2/2 通过，查看实图确认中文、金额、覆盖与单位没有截断。
- r1 full 9/9；重复 acceptance 浏览器后段注册 429（12 通过、4 失败）：同一验收代理 IP 的 15 分钟认证预算累积。记录 r1 为 fixture 失败，开始 r2；新增 `prepare-browser-fixture.ps1`，固定 project 并核对数据库容器 Compose project/service 标签，单独过期虚构测试的认证预算窗口；下次请求仍执行真实应用计数。未放宽生产限制，未删除开发/验收账户、库存、流水、会话或数据卷。
- 最终 r2 full 9/9（13 前端单测）、acceptance 11/11（skip-build 无构建检查，新增 browser-fixture），浏览器 16/16、0 跳过/失败/flaky；运行 `9ea1fc69f21e82d85bc0a09bfff32b0f` / `390275e9871320b896b9d068bbd3d9ef`，同摘要 `33950b0e788d214fa7f5149e9901b5c6541d55cb267860c9a5a3ba71f5b29019`。业务镜像已在初次验收重建，后续仅改测试/验收准备脚本，复用该镜像；自动镜像绑定债务仍保留。`family-monthly-report-r2-20261001` 已 passed，便携证据：[family-monthly](../validation/family-monthly-20261001.json)。
- 菜谱提交 `6d6fa67` 的 [CI 36823185891](https://github.com/link1ks/FoodFlow/actions/runs/36823185891) 四任务均 completed/success，无失败步骤。
- 重现预算耗尽后的重复性验证：仅在固定 acceptance 数据库将虚构认证计数设为 61，运行准备脚本后再跑全部浏览器 16/16，通过而未关闭限流。这是对已复现 fixture 失败的回归，不据两次通过推断用户收益或生产容量。

### 账号生命周期

- 任务 `family-account-lifecycle-20261001`：已验证手机号短信找回、原/新双号码换绑、互补邮箱/手机号账号合并。只允许一边邮箱、一边已验证手机号，避免静默覆盖冲突登录标识。保留当前账号 ID/称呼/密码，合并已证明属于两账号的家庭权限；来源用户保留停用标记，库存与历史流水 actor 不改写。
- 会话加入 auth_version；密码找回、换绑和合并提升版本并撤销会话。过期身份校验即使延迟插入 session 也不能绕过版本检查。来源 queued/running 任务取消并撤销租约；之后人工重试记录当前请求者。
- 后端定向两项与 OpenAPI 校验通过，覆盖缺少确认、错误密码/用途、错误新码不消耗有效旧码、重放、旧会话/旧密码拒绝、所有权转移、库存/流水保留、不可变合并审计。短信使用 recording fixture，不发送真实消息。
- 检查曾被账号用量限制阻止，命令未执行；用户恢复额度后重跑成功。该等待计入真实任务墙钟时间，不记录虚构测试结果。
- 页面补找回、换绑、合并及显式确认；短信未配置时显示不可用且不能提交。邮箱专用账号暂没有自助找回；两个同类账号/已同时绑定两种方式的账号不支持合并。真实短信送达保留 AUTH-SMS-DELIVERY 债务。030 Down 明确拒绝自动撤回身份转移；回滚需审查恢复方案，未执行破坏性 Down。
- 月报提交 `150a80b` 的 [CI 36825163622](https://github.com/link1ks/FoodFlow/actions/runs/36825163622) 四任务全部 completed/success。
- 最终 full 9/9，重建 acceptance 12/12，浏览器 20 通过、0 失败、0 跳过；运行 `fb42e67366569a91fb812029cd96d3f6` / `99c5022c560b3f1f9341e22cba207950`，源码摘要 `2a710463d4a3728ba47a450f87bb2f4907f154b5f4f9313002ea5b01b2d0eb73`。便携证据：[family-account](../validation/family-account-20261001.json)。

### 营养依据、接口类型与生成回归

- 任务 `family-nutrition-contracts-20261001`。031 迁移保存按批次、版本、确认者与时间的不可变称重依据：原单位样本对应可食克重，以及保留目录/深色/非深色/未知分类。不改变单位、库存余量和过去记录；用户称量/分类不等于食材检测或实际摄入。g/kg 可食净重不能超过样本原料重量，未确认时仍明确为原料重量估算。
- 鸡蛋 2 个→100g 可食部、牛奶 250ml→260g fixture：旧早餐始终未知；新午餐 206.44 kcal；后来鸡蛋改为 80g 不重算午餐，晚餐加入后日合计 384.28 kcal。验证幂等重放、旧版本拒绝、不可变依据、家庭越权/只读成员、深色改未知不改旧快照，以及可食重量上限。未补造缺失的 USDA 品种/营养值；NUTRITION-REFERENCES 保留。
- 所有成功 API 响应补 JSON/图片/SSE 媒体和结构。共用真实 API fixture 校验实际响应（包括预期业务错误）；有意无效的请求仍进入真实 handler。标准 openapi-typescript 7.13.0 生成库存、菜谱、账号、营养、月报等前端使用的类型；full/CI 添加生成类型过期检查。模型任务 payload/result 明确为可扩展 JSON，API-JOB-SCHEMAS 保留，不称为每类模型结果都已严格校验。
- 初次接入响应检查的批量编辑误改测试里的 image/png 字面量，编译失败；定位到具体条件并修复。新的全 app 集成响应校验通过，没有为了迁就 fixture 放宽业务条件。
- 账号提交 `785fd53` 的 [CI 36845481497](https://github.com/link1ks/FoodFlow/actions/runs/36845481497)：frontend/images/platform 成功，backend 的 sqlc 一致性步骤失败；030 新增模型未随账号阶段生成。开始 `family-dbgen-regression-20261001`（baseline reproduced）；已重新生成 030/031 类型。新增 full 必需的独立 sqlc 检查，在 SQL/config 临时副本生成并比较，不改工作区与数据；回归拒绝旧内容、缺/多文件，允许 CRLF 等价。上一阶段本地通过不等于该提交云端通过。
- 最终 full 12/12，重建 acceptance 12/12，浏览器 22 通过、0 失败/跳过/flaky；运行 `28993004b9ca1d245fff4a819aee0d30` / `a8488e5293b071a76b5b081bb0e570cc`，同摘要 `3dd29675ddec62a52803311e0e3c45b49a610660e27d3c410f7bcf22bc8531c2`。浏览器确认 2 个鸡蛋→100g 依据保存后库存仍 4 个、流水仍一条；查看手机截图确认布局与状态。便携证据：[family-nutrition](../validation/family-nutrition-20261001.json)。

### 家庭负载、镜像绑定与试用

- 任务 `family-trial-harness-20261001` 首次负载失败，按 unknown 记录：测试把 Outbox event_id 误写为 id，10 个守恒检查失败；另有 1 次未分类请求错误，首次报告没有保留状态分类，不能断言其原因或已修复。保留 `.cache/harness/family-load-initial.json`，开始 r1 回归任务。仅修正 fixture 字段并加入脱敏错误分类、最长耗时，没有放宽业务库存规则。
- r1 定向负载：10 家庭、20 并发客户端、30 秒，32,095 请求、4,585 完整循环、0 错误/守恒失败，P50 3.72ms、P95 9.745ms、P99 13.147ms、最长 81.932ms，100ms 采样最高 5 个锁等待者。目标 0 错误与 P95<2000ms；在独立真实 PostgreSQL 与进程内 HTTP API 上测试，不覆盖网关/公网/Kafka/真实模型，不能称作生产容量或长期稳定性证明。后续 full 会再次执行该场景。
- 验收构建记录源码摘要与不可变 API/web 镜像 ID；运行容器必须属于固定 acceptance 项目，API/Worker/Web 以及平台 Relay/Insights 匹配同一构建。旧环境缺少清单的真实负例已经被 images 模式拒绝；单元负例覆盖旧源码/镜像、错误项目、缺/重复服务和 unbound 标签。源码/标签报告为本地证据，不是防篡改证明。
- 扩展标准响应校验至列表 fixtures；拆分业务 JS 与共享依赖（169.73KB / 330.93KB），静态哈希资源长期缓存、入口 no-cache。构建无大 chunk 提示，但未将构建大小或缓存配置当作实际用户加载速度提升。
- [家庭试用指南](../FAMILY_TRIAL.md) 给出协作、三天业务闭环、匿名观察指标和本地备份；真实家庭回访仍保留 UX-TRIAL。新增显式 Compose 试用覆盖，禁用模型/短信/菜价同步并使用本地存储，保存的 API 密钥和数据卷不改写。
- 营养与契约提交 `dd522ba` 的 [CI 36848745945](https://github.com/link1ks/FoodFlow/actions/runs/36848745945) backend/frontend/images/platform 全部 completed/success，先前账号生成一致性云端问题已在此提交验证修复。
- 最终 full 12/12（`e25c3cff0adc21340781b9a59a6f946e`）、重建 acceptance 13/13（`e2e29d71168bd742924d8c96d3a51251`），同摘要 `6e2c758851867498487c0992da10229a73c0a3b91b617a061a1e7e924f1ebffe`。22 浏览器测试通过，0 失败/跳过/flaky；绑定 API/Worker/Web/Relay/Insights 的镜像 ID 与摘要。full 再次负载 32,060 请求、4,580 循环，0 错误/守恒失败，P95 9.864ms、最长 75.244ms，采样峰值 6 个锁等待者。便携证据：[family-trial](../validation/family-trial-20261001.json)。
- 本地旧库与照片先备份到受保护的 `.cache/trial-backups`，然后试用 overlay 升级启动 5173。初次只读核对误用 inventory_batches 表名，另一次脚本变量与 PowerShell HOME 冲突；修正为真实 batches 与专用变量后核对账号/家庭/批次/流水数保持一致。没有删除或重建原卷；实际环境模型/短信配置为空、菜价同步关闭、本地存储，健康 HTTP 200，入口 no-cache、哈希资源 immutable。备份未做恢复演练，不把成功导出等同于已证明可恢复。
- 家庭试用代码已完成本地验证，下一步提交并推送；最终云端状态以对应 GitHub Actions 运行结果为准。真实家庭回访、模型质量、短信送达、未覆盖营养资料及生产运维仍保留。
- 交付结果：`family-trial-harness-r1-20261001` 已记录 passed。代码提交 `a5c9ade428bd93e77dce88168d4bfb6a8359b619` 已推送 main；[CI 36851834888](https://github.com/link1ks/FoodFlow/actions/runs/36851834888) completed/success，backend/frontend/images/platform 四项均成功且没有失败步骤。提交后源码摘要仍与本地 full/acceptance 一致；本地试用状态再次 HTTP 200、原卷保留。
- 此后仅补本执行计划与便携报告的云端结果，不改业务/构建/契约输入。文档记录提交使用 [skip ci] 避免重复运行同一源码的 CI；上述云端通过结论严格对应 a5c9ade 代码提交，未把文档提交称作新一次测试。
