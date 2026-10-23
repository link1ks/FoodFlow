# README 渐进展示与阅读顺序

任务：`readme-progressive-layout-20261022`，maintenance，baseline unknown。
已绑定契约见 [验收绑定](../quality/tasks/readme-progressive-layout-20261022.json)。
本任务独立于已完成的 `ux-real-showcase-20261022`，不改写旧契约或旧完成证据。

## 范围与审查标准

用户希望展示分散、逐步展开，功能说明放在阅读中段。本次只调整
[README](../../README.md) 与 [操作实录说明](../SHOWCASE.md) 的信息顺序和
呈现方式，复用现有真实浏览器素材，不修改应用、测试、模型配置或数据库。

先让读者理解项目并找到启动入口，再在阅读中段按模块了解功能。
每段操作展示紧邻对应说明，通过逐步展开控制阅读长度，避免连续大图占据前部。
标题、展开入口及步骤文字应清楚说明将看到的模块和结果；收起时仍能理解功能。
完整录制步骤、预置数据与限制继续由 SHOWCASE 提供，链接应能直接到达对应位置。

审查时同时检查桌面与窄屏实际 Markdown 渲染：初始阅读顺序、每个展开入口、
展开后的素材宽度、图文对应、锚点和相对链接、SHOWCASE 的返回路径。
发布后检查 GitHub 实际渲染，不把本地预览与 GitHub 渲染视为同一份证据。
现有命名测试只验证功能说明的业务依据，不能证明 Markdown 视觉布局合适。
源码检查和业务回归也不能代替对素材来源、既有哈希及显示位置的人工核对。

保持已有演示的事实边界：隔离虚构账号、预置数据、确定性模型 fixture、
明确确认后才写入，以及历史未知金额等限制不因排版被隐藏或改写。
本任务不新增付费调用、不宣称物理设备或真实模型质量通过，也不扩大历史稳定版验收。

## 已发生的任务准备

- `2026-10-22T10:55:47.3482311Z` 已通过 harness 记录 maintenance 开始，baseline 为 unknown。
- 已阅读 AGENTS、HARNESS、ENGINEERING_FEEDBACK、既有契约及当前 README/SHOWCASE。
- 已执行工程经验索引与 stage=start 查询。输入显式设置 `error_code=""`、
  `applicable_version="0.3.0"`、`business="UI"`、`verified_only=true`。
- 查询于 `2026-10-22T10:55:54.3124014Z` 返回空结果；没有采用或推荐历史修复。
  无命中不表示没有排版问题。私有输入与结果分别为
  `.cache/harness/readme-progressive-layout-query.json` 和
  `.cache/harness/readme-progressive-layout-retrieval.json`。
- 契约已语义审查并于 `2026-10-22T11:01:08.7565895Z` 绑定；没有复用旧任务的成功报告。

契约仅使用已有业务绑定，不新增测试。两次独立源码审查确认文案与现有操作一致，
包括生菜50g补100g、蒜10g、家庭库存隔离、未知金额和确定性社区初审。
业务语义未变，没有仅因排版调整而刷新 business review digest。

## 已完成的布局审查与失败保留

README 首部保留简介与启动入口，中段分成食材、菜单、采购与今天、记录、家庭、社区，
之后才是更新与技术内容。七个独立 details 默认收起，各自跟在对应功能文字后。
旧功能锚点保留，SHOWCASE 增加返回功能介绍的链接；社区候选文字明确为最多三道。
七段公开GIF逐一与原素材清单SHA256核对一致，本任务未改素材或回写旧录制来源。

本地 Markdig 预览经浏览器目视检查：桌面视口1265×720、正文上限900像素；
另用390像素正文栏检查换行及展开后的图像宽度。后者是窄栏预览，不是物理手机或
GitHub手机页面验收。菜单实录单独展开，其他入口保持收起；文字与图像均未越出正文。
截图为 `.cache/harness/readme-layout-local-desktop.png` 和
`.cache/harness/readme-layout-local-narrow-expanded.png`。本地预览用于布局审查，
GitHub 的最终锚点、相对链接与实际渲染须在发布后另行核验。

首轮 full 在 Go 编译阶段因 Windows `VirtualAlloc errno=1455` 内存提交空间不足退出，
没有得到完整命名测试和新鲜负载证据。原报告保留为
`.cache/harness/readme-progressive-layout-full-first-failed.json`，未宣称通过。
退出后内存快照不是故障峰值测量；已人工脱敏登记
`readme-20261022-windows-commit-limit`，显式错误码 `WINDOWS_COMMIT_LIMIT_1455`、
适用版本0.3.0，并执行 failure/repair 经验查询。重跑只将 Go 包构建并发限制为
`GOFLAGS=-p=1`、`GOMAXPROCS=2`，不改业务代码、不删测试、不降低20客户端负载。

## 验证与交付证据

以下区分已核验结果和发布后仍需完成的步骤。所有正式报告带
`-task readme-progressive-layout-20261022`，最终报告须晚于契约绑定并匹配当前源码。
保留失败报告，不用新成功结果覆盖失败历史。

| 证据 | 实际结果与位置 |
| --- | --- |
| 契约绑定 | 已于11:01:08Z绑定3条要求、7个现有命名测试；旧契约不变 |
| 仓库检查（repoGate） | knowledge 4项通过；最终full的business-map、quality-map、知识、架构和仓库卫生均通过，公开diff范围已审查 |
| 文档视觉与素材检查 | 独立语义审查、桌面与390像素窄栏预览通过，7个素材哈希匹配；GitHub实际页面另行核验 |
| full | 低并发重跑18项检查、172个命名测试全部通过；`.cache/harness/readme-progressive-layout-full.json`，首轮失败另行保留 |
| 有界负载 | 10家庭、20客户端、30秒；3167周期／22169请求，错误及不变量失败均0，p95 26.425ms（上限2000ms） |
| acceptance | 17项检查、24桌面＋24手机命名场景全部通过，无跳过或重试；`.cache/harness/readme-progressive-layout-acceptance.json` |
| diagnose | 5项检查通过；`.cache/harness/readme-progressive-layout-diagnose.json`，隔离服务健康与脱敏诊断正常 |
| main-site | 5项检查通过；`.cache/harness/readme-progressive-layout-main.json`，新鲜无登录浏览器只读核验默认5173及既有前端构建身份 |
| 发布与 cloud | 提交发布后按当前HEAD检查backend、frontend、images、platform四项；实际结果保存 `.cache/harness/readme-progressive-layout-cloud.json`，不在提交前预写成功 |
| GitHub 文档复核 | 发布后逐项展开7段实录，检查README／SHOWCASE锚点和链接；实际截图及观察保留在 `.cache/harness/readme-progressive-layout-github-review.json` |
| feedback 与 metrics | 在发布后完成cloud及故障解决修订，再从完整保留历史重算 `.cache/harness/readme-progressive-layout-metrics.json`，生成同任务feedback报告 |
| 版本2完成包及 task finish | 全部门禁通过后使用 `.cache/harness/readme-progressive-layout-completion.json`，关联五类验证和feedback，再由task finish记录真实结果 |

full 与 acceptance 不并发。即使没有修改应用，README 和新契约仍影响仓库源码摘要，
所以正式报告需重新生成；隔离验收镜像也须满足当前摘要绑定，不能把旧镜像伪标为新验证。

本次没有应用更新，不需要为 Markdown 排版重建默认数据库或重建默认应用部署。
默认5173仅做只读核验，不登录、不写合成数据，不触及配置、凭据、照片或数据卷。
如果范围后续扩大为应用修改，应另行确认契约范围并按仓库约定备份及更新默认部署。

2026-10-23 续接时默认容器已停止，因此按原Compose依赖链启动了现有服务，
包括既有migrate容器；没有重建。五个核心服务的容器ID、镜像、配置哈希和数据卷
前后相同，证据为 `.cache/harness/readme-progressive-layout-default-start.json`。
这次启动不作为新应用部署或数据库备份／恢复演练声明。此前已提交的存储维护
计划保留；其变化未影响本任务源码摘要。最终本地报告摘要均为
`b031f95eab2c38ef18a00536dd25cf26c9733f9ff919121d9c235e8b7d6ff972`。

feedback 明确 `evaluations=[]`、`device_required=false`、`rag_required=false`，
理由已写入契约。若执行中发现问题，先保留证据并脱敏分类入库；适用的故障必须有
真实修复及对应回归，不能只把状态改成 resolved。最终指标包含完整历史，不能仅统计本任务
或删除旧失败；task finish 本身不新增 verification run。
