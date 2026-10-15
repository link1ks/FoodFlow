# 业务质量与技术债

[一百次语义理解试验](INTAKE_SEMANTIC_EVAL.md)：模型完全符合由规则的 45/100 提升至 75/100，错误草稿仍七条，否定/歧义子类变差；不接入产品。JSON/数量来源合法不能替代意图评测，相关安全债务仍开放。

2026-10-15 已补充 [真实 OSS 与百炼任务链路](OSS_MODEL_CHAIN.md)，修复明确否定/跨句撤回与改口的数量预填，正常音频清理通过；代表性质量、麦克风与中断音频治理仍未验收。

更新：2026-10-15。机器可读台账：[domains.json](quality/domains.json)。模块 owner 指代码归属，不代表虚构的人员负责人。

百炼已完成明确授权的五次图片、五次合成语音调用，证据、费用折算与修复边界见 [真实模型验证](BAILIAN_VALIDATION.md)。代表性图片、噪声/麦克风与实际账单仍未验收，相关台账保持开放。

## 如何判断质量

| 业务域 | 必须保持的行为 | 当前验收边界 |
| --- | --- | --- |
| 库存 | 精确计量、FEFO、非负、流水/Outbox 同事务、重放不重复扣库 | PostgreSQL 真实集成、守恒 fuzz；10 家庭/20 客户端/30 秒混合负载，范围与 SLO 见试用指南 |
| 权限 | 当前家庭成员和角色由服务端判断；家庭切换不串数据 | 后端越权/角色场景，浏览器切换与降权 |
| 菜单 | 草案不扣库，确认重算，烹饪只扣一次 | API 契约链与桌面/手机 UI 成功链 |
| 社区（归菜单域） | 配方及封面初审通过才公开、作者修改重审、全站入库审核权限、周榜固定、积分只发一次且兑换不透支 | PostgreSQL 并发/权限/回放及派发恢复；本地确定性模型，桌面/手机封面重审；真实模型准确度、多账号刷票和举报运营待完善 |
| 采购 | 实购确认后入库，重复请求不重复增加库存 | UI 采购入库、幂等与软删除回归 |
| AI | 结构校验、取消/租约 fencing、人工确认、收费发送前持久化预扣与并发限制 | 确定性 fixture；应用预扣上限已验证，实际账单封顶与模型效果待真实验收 |
| 语音录入（归库存与 AI 域） | 用户同意后转写；包装数量不猜重量；草稿确认前不入库；整批幂等确认 | 本地 Qwen 协议 fixture、权限/取消/事务回滚与桌面/手机 UI；真实麦克风、噪声及百炼识别效果待验收，见 [语音指南](VOICE_INTAKE.md) |
| 统计 | 独立数据库、事件去重/隔离、故障不阻塞核心写入 | 真实 Redis/Kafka 本地恢复；生产配置待验收 |
| 营养 | 历史快照、未知覆盖明确、无虚构健康分数 | 真实数据库快照与覆盖规则 |
| 前端 | 交互闭环、错误状态、贴合食材的本地照片 | 桌面/手机 Chromium；真实用户研究待开展 |
| Harness | 可复现约束、契约、映射、完成证据 | full/acceptance 门禁与镜像绑定；动态任务契约仍在台账 |

Redis 是 API 必需依赖：启动和就绪检查拒绝不可用的 Redis，公共目录、菜谱和官方价格使用最多 60 秒的可重建缓存，鉴权与家庭权限仍先检查。回归覆盖跨 API 命中、筛选隔离、上海日期键、过期/损坏回源和不完整用料禁止缓存；平台验收覆盖实际 Redis 停止与恢复。所有独立部署和发布检查均检查此依赖。此范围不证明 Redis 生产高可用、TLS/ACL 或持续容量。

本表不使用没有基准的完成百分比或质量分数。台账中的 `checks` 是场景定位信息；文件存在不代表测试通过，实际结果必须来自当前源码的 full/acceptance 报告。full、fast 与 acceptance 的 `quality-map` 检查会拒绝丢失的业务域、owner、测试路径/锚点和不完整技术债。

数量回归覆盖整数/小数部分只能使用 ASCII 数字，以及非法请求不得创建食材、批次、流水或 Outbox；合法三位小数与幂等重放仍保持精确。清冰箱回归构造超过 400 条用料的目录，验证候选菜的完整 BOM：缺少末项时不推荐，补齐后分配全部 30 项用料。修复状态与依赖安全复核见 [测试审计](TEST_AUDIT.md)。

## 维护与关闭规则

新增功能同步补 acceptance、checks 和未覆盖风险；移除场景必须先明确替代验证。每条技术债维护唯一 ID、P1/P2/P3、open/closed 和可验证的 completion。关闭时填写 `evidence`，指向仓库中的验证报告或执行计划；门禁拒绝空证据和不存在的文件。不因文档修改或单次绿色运行推断用户收益。

任务失败用 `-failure-category product|fixture|infrastructure|unknown` 分类，历史未分类记录保持 unknown。按类别统计用于定位工程反馈短板；墙钟时间包括等待，不是人工投入或 AI 效率提升指标。

## 持续验证

```powershell
go run ./cmd/harness -mode full
go run ./cmd/harness -mode acceptance
go test ./internal/core -run '^$' -fuzz FuzzQuantityRoundTrip -fuzztime 10s
go test ./internal/core -run '^$' -fuzz FuzzConversionAndServingConservation -fuzztime 10s
go test ./internal/engine/scheduler -run '^$' -fuzz FuzzScheduleStockConservation -fuzztime 10s
go run ./cmd/harness -mode summary -out .cache/harness/summary.json
```

普通全量测试执行 fuzz seed；额外 fuzz 命令探索更多输入，不能将有限时长运行当作形式证明。场景定位与已关闭技术债的依据见 [验证说明](VERIFICATION.md)，当前提交以其 CI 与新运行结果为准。
