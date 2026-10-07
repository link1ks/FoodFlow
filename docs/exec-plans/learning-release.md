# 食光学习交付顺序

开始：2026-10-07（Asia/Shanghai）。用户指定顺序：部署与安全 → 备份恢复 → 账号找回与 AI 限额 → 家庭内测 → 公开发布。用户确认还没有云服务器，先完成本机部署与验证。目标为学习、降低支出；真实公开发布保留为最终阶段，不据此立即购买或暴露服务。

## 阶段与完成条件

| 顺序 | 阶段 | 完成条件 | 当前状态 |
| --- | --- | --- | --- |
| 1 | 部署与安全 | 独立可复现部署，端口/代理信任/容器权限/浏览器策略验证，完整 harness 与相关验收通过，GitHub 同步 | 本机通过，Linux 修复及 GitHub CI 已通过；云端主机待有服务器后另验收 |
| 2 | 备份恢复 | 主库与图片共同备份、脱敏清单与校验；在新空数据库和图片卷恢复并核对账号/流水/库存/图片；明确 RPO/RTO 与保管方式 | 本机及 GitHub CI 已通过；异地恢复仍待另验收 |
| 3 | 账号找回与 AI 限额 | 邮箱账号可用的恢复渠道、单次使用/过期/防枚举/限流与会话撤销；模型调用持久化预算预占与并发限制，失败/租约恢复不自动重复收费 | 本机学习版完成：预存恢复码及应用预扣上限通过 full/acceptance；真实账单封顶仍未验收，收费模式关闭，本轮 GitHub 同步待收尾 |
| 4 | 家庭内测 | 获得成员同意，完成入库→规划→采购→做饭闭环和至少一周实际观察；记录使用负担、回访、失败与实际 AI 消耗，修复重现的问题 | 等前三阶段；自动测试不替代成员体验 |
| 5 | 公开发布 | 用户确定实际发布目标与费用，验证 HTTPS/主机访问/更新/恢复/监控、账号与模型成本控制；处理隐私/删除/许可与公开支持范围 | 最后评估；目前无云服务器、域名或公开发布操作 |

每个实现阶段记录独立 harness task，执行 full 与相关 integration/acceptance/diagnose，把真实证据、失败和限制写在这里，再同步 GitHub。常规测试只使用确定性模型，不发送真实短信，不删除开发数据。现在有 ¥5/月应用预扣硬上限；它不等同于服务商实际账单硬封顶，真实计费仍需验证。

## 第一阶段记录

- 任务 `learning-deploy-security-20261007` 已记录 start。原 main 工作区干净；没有更改或输出现有 `.env`。Docker 引擎初始未运行，启动既有 Docker Desktop 后可读取 ServerVersion 29.8.0，未移除容器或卷。
- 新增 [学习部署说明](../LEARNING_DEPLOYMENT.md) 与独立 `compose.learning.yaml`；固定本机端口 17173，禁用收费集成、单独数据库与图片卷。修复网关 IP 信任边界和错误响应安全头。
- 验证结果待填；不得把已写配置当作已通过验收。公开 TLS、云主机设置、异地备份、真实家庭体验和真实 AI 质量不在本阶段通过范围。
- 初次独立部署安全检查与厨房 smoke 通过。临时照片脚本误发送 `low=0`，且缺少遇错即停，输出不能作为通过证据；原任务按 fixture 记录失败。修正输入后重新上传/读取 SHA256 一致，并转入固定项目的 `learning-security.ps1` 回归，脚本遇错即停。
- 首次 full 为 11/12；Go 集成失败，64 KB 尾部丢失早先失败名称，尚不能归因。r1 按 unknown 记录失败，开始 r2，并通过 `go test -json` 提取真实失败事件。没有放宽数据库、家庭权限或认证预算。
- JSON 重现定位 `TestIngredientCatalogSelection`：固定 `expires_on=2026-10-01` 在 10 月 7 日已经过期，实际库存可用量正确为 0。将此“可用库存幂等入库” fixture 改为运行日后 7 天，保留 300 g、一次入库、同键不同请求 409 等断言；没有修改过期分配规则。64 KB 日志尾部可能丢失早先失败仍是诊断限制，失败时使用 Go JSON 提取定位。
- 最终 r2 full 12/12（`99c6a1e8168265a4b7526d1bf7aa2e6d`），重建 acceptance 13/13（`d0e28ab72fbf5754ec0eceabe57a544b`），同摘要 `9e3607cb2575d93473fc037e4c0bfd61efe5cc8ea5415370ceecf9b2bdb1d8e9`。桌面/手机浏览器 24/24，0 失败/跳过/flaky；包含注入脚本被阻止以及登录、图片、PNG 导出可用。Redis/Kafka 恢复、网关替换与脱敏 diagnose 通过。完整验证再跑 10 家庭/20 客户端/30 秒混合负载，32,256 请求、0 错误/守恒失败，P95 9.057 ms；仅进程内 API+真实 PostgreSQL，不据此宣称公网容量或用户效率。
- 独立学习项目 17173 的容器权限/端口/响应头检查、实际照片上传回读 SHA256 与厨房 smoke 通过；API/Worker/Web 为非 root，只读根目录且去除全部 capabilities。学习数据库和图片卷保留，未改原开发数据；原 Compose 回环端口调整只在后续按该配置重新创建时生效。受保护配置初始化拒绝覆盖现存凭证，模型与短信明确禁用。没有购买服务器、发送短信或调用真实模型。
- `learning-deploy-security-r2-20261007` 已记录 passed；便携证据：[learning-deploy-security](../validation/learning-deploy-security-20261007.json)。CI 加入独立学习环境的实际安全/照片/厨房验收；对应提交的云端结果待 GitHub 运行后记录。本地通过不替代云端 CI，也不关闭 PLATFORM-OPS、AUTH-SMS-DELIVERY、AI-EVAL、UX-TRIAL 等剩余债务。
- 已正常推送代码提交 `ba7510d936d414fdb157cca68be817be96dfd785` 到 main；提交后源码摘要仍与本机 full/acceptance 一致。[CI 37566459044](https://github.com/link1ks/FoodFlow/actions/runs/37566459044) 最近读取为 in_progress：frontend completed/success，backend/images/platform 仍在运行，无已报告失败步骤；尚未宣称全部云端通过。此后仅记录文档状态，不修改上述验证输入。
- 本轮复查该 CI 已 completed/failure：backend/frontend/images 成功，platform 在学习部署安全检查失败。选择性读取报错确定 Linux 清理进程变量留下空 `POSTGRES_PASSWORD`，覆盖 Compose env-file。将原值缺失时的清理改为删除 Env 项；恢复验收在同一 PowerShell 进程检查缺失变量仍不存在。上条是当时快照，不能视作最终云端成功。

## 第二阶段记录

- `learning-backup-recovery-20261007` start：此前工具仅覆盖数据库，未形成照片共同静止点。新增 [配套备份恢复说明](../BACKUP_RECOVERY.md)、原生二进制 Docker 流工具、只读内容指纹与全新私密恢复项目；未读取/输出原 `.env`，未删开发数据，真实 AI/SMS 关闭。
- 原任务按 unknown 记录 failed：Compose `up --wait db image-init` 把一次性初始化退出 0 判为失败。改为先等待数据库、启动 API 时按 completed 依赖运行初始化；失败目标保留停止。r1 按 product 记录 failed：Windows 重复设置受保护目录的新 ACL 触发权限错误。改为保留现有 ACL 元数据，仅修改必要访问规则，已私密时跳过；加入重复设置回归。r2 已 start，完整证据待验证后填写。
- 新工具暂停 API/Worker、逐表/序号/照片只读前后快照一致后才写完整清单；dump/TAR 文件长度及 SHA256 校验、引用完整性、文件路径/链接/重名/超限/空目标保护通过。全新目标使用备份镜像精确 ID、单事务 restore、全新 JWT，无主机端口或 Worker，完成后停止并保留目标。
- 最终版本虚构恢复验收：45 张 public 表与所有自增序号状态一致，4 张照片的路径与字节一致，恢复用时 10.529 秒；原写入服务暂停 5.490 秒。恢复后密码登录、300 g 库存、番茄照片 SHA256、备份前幂等请求不重复写入及新 1 g 入库成功，匿名访问拒绝、旧令牌拒绝。原账号令牌、300 g 与照片仍可用；新私密损坏副本在创建恢复配置/目标之前拒绝。私密备份、真实账号信息、令牌和原始业务行不加入 Git/CI artifact。
- RPO 24 小时为每天手动成功备份的目标，目前没有调度；RTO 10 分钟为目标，以上计时只是同机小数据集演练。尚未验证异机/异地/磁盘灾难、加密备份、自动切换、保留期清理或独立 insights/Kafka 恢复，不关闭 PLATFORM-OPS 总体债务。没有购买服务或产生模型/SMS 费用。
- 实现任务 r2 的 full 12/12（`7ac095953adb7d539fc01d071355722e`）与重建 acceptance 13/13 通过，task 已 passed。复查发现 CI 重复调用/上传恢复验收，另起 `learning-backup-ci-dedup-20261007` maintenance，去重后重新在最终源码验证，不复用旧摘要。
- 最终 full 12/12（`7f54a4ec3033fe2f5fdb25c53fcb26e4`）、重建 acceptance 13/13（`07413b200673d12285651792e96e62fc`），共同源码摘要 `8a883e8db5c7506c4ff1602efbc4402c42819e78b1ee4046fc2ce381a1ae2232`；桌面/手机 Chromium 24/24，0 失败/跳过/flaky，脱敏 diagnose、网关替换、Redis/Kafka 恢复与厨房闭环通过。最终维护 task 已 passed。便携脱敏证据：[learning-backup-recovery](../validation/learning-backup-recovery-20261007.json)。真实恢复集成在 CI 去重前已通过，应用/恢复脚本未随后更改；最终 full/acceptance 对应去重后的全部提交输入。
- 正常推送代码提交 `016f726e64a7b1824f1d549ba26e4910d1d41e3d` 到 main；[CI 37569143684](https://github.com/link1ks/FoodFlow/actions/runs/37569143684) 最近读取为 in_progress：frontend/images completed/success，backend/platform 正在完整 Go/race 检查，0 已报告失败步骤。学习恢复的 Linux 云端步骤尚未执行完，不宣称云端全部成功；下次继续先复查。此后仅追加文档快照，使用 `[skip ci]` 不重复触发同一源码的云端构建。
- 第三阶段初查：已有真实模型 worker 租约失效后转 failed、人工重试的保护；仍缺持久化成本预占/家庭硬限额和不依赖真实短信的账号恢复。后续优先采用学习环境可用的零服务费方案，模型调用继续以确定性 fixtures 验证后再启用。
- 本轮复查 [CI 37569143684](https://github.com/link1ks/FoodFlow/actions/runs/37569143684) 已 completed/success，backend/frontend/images/platform 均成功；证明上轮 Linux 空环境变量清理、独立学习安全与真实备份恢复已在该提交通过。上条 in_progress 是当时的快照，现由此条补上最终结果。

## 第三阶段与清理记录

- 用户明确要求先清理再进行下一步。按 Windows 存储治理的安全分类，仅清理当前项目明确可重建的旧 `.cache` 根目录日志/重复云端 ZIP（49 文件，17,948,585 bytes）；保留全部凭证、备份、SQL/dump、数据库/图片卷、harness run/task 证据、依赖和浏览器运行时。单文件删除前检查绝对父路径与非链接，不枚举个人资料或系统目录，不移除数据卷。
- 停止固定 `foodflow-acceptance` 标签下的 9 个运行容器；它们原容器内存统计合计约 597 MiB，学习项目仍就绪。用 Docker 官方 Buildx prune，只匹配 24 小时未使用的缓存，保留至少 2 GB、设 4 GB 目标，不删除镜像或卷。操作回收 6.439 GB，缓存由 15.28 GB 降至 8.841 GB；本轮重新构建后为 11.01 GB，净减少约 4.27 GB。验收结束再次停止临时环境，最终仅 4 个学习服务运行。没有清空 Windows RAM、浏览器账号、系统缓存或压缩 Docker VHDX，不能声称宿主物理磁盘文件已相同比例缩小。
- `learning-account-ai-20261007` feature start 后，后端确定性恢复码/AI 额度集成通过；UI 首次 build 发现 AppShell 引用不存在的 `house`，按 product 记录 failed。改用该组件传入的 `household.id`，转 `learning-account-ai-r1-20261007` regression start，完整前端 build 与浏览器实际个人主页/找回流程成为回归门禁。
- 新增 [恢复码与 AI 额度指南](../ACCOUNT_RECOVERY_AI.md)、迁移 032、OpenAPI/前端类型、生成数据库模型和知识清单。恢复码需当前密码再验证及显式确认，五个随机 128-bit 码仅显示一次，保存哈希/180 天有效期/凭证版本；重置共享事务与用户锁，撤销全部会话和剩余码，错误/不存在/过期信息一致，持久化认证尝试限制。手机号重置/换号/合并的凭证版本也使旧码失效。邮箱账号无需邮件服务费，但未预先保存码且没有短信证明时无法自助找回。
- 收费模式显式关闭为默认；启用后发送前提交唯一任务预扣，金额按 milli-CNY 整数，服务与家庭共同月上限 ¥5、默认文字 ¥0.10/图片 ¥0.50 全额预扣、每日 20 次、并发 1；检查角色/租约/取消/请求大小，禁止模型 HTTP 重定向。重启、同任务 retry、月份切换均不能清掉原预扣；网络/进程不确定保留额度和并发。固定学习项目维护命令须操作者确认服务商已结束，再只释放老的终态请求并发，不退款、不派发重试。真实 token 账单、图像计费、价格变化未验证，AI-INVOICE 债务保持 open，不把应用额度当成实际费用封顶。
- 最终 full 12/12（`cacc12b524d992c744c64360793e4a3c`）、重建 acceptance 13/13（`6bcb725dbd616a58c39debe5aeb6c205`），共同源码摘要 `62c0cefee0e8a72a060f7359f64befc3aa355f9ce8943240f97342b11fe7987e`。桌面/手机 Chromium 26/26，0 失败/跳过/flaky，含恢复码生成、明确确认、邮箱密码找回及新密码登录；厨房闭环、Redis/Kafka、网关替换和脱敏 diagnose 通过。r1 task 已 passed。证据：[learning-account-ai](../validation/learning-account-ai-20261007.json)。
- 本机 17173 已更新，迁移新增表后学习安全检查通过；配套恢复核对 48 张 public 表、自增序号与 6 张照片，恢复 10.303 秒、暂停写入 5.282 秒，原令牌/300 g/照片保留，损坏副本仍在创建目标前被拒绝。所有备份和恢复目标保留停止，维护 status 只读通过。全程使用虚构账号与本地模型 fixtures，未购买服务、发送真实短信或调用付费模型。收费开关仍关闭，真实家庭内测需后续成员参与，不用自动检查代替一周体验。
