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
