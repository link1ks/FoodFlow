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
