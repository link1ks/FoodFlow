# 业务质量与技术债

更新：2026-10-01。机器可读台账：[domains.json](quality/domains.json)。模块 owner 指代码归属，不代表虚构的人员负责人。

## 如何判断质量

| 业务域 | 必须保持的行为 | 当前验收边界 |
| --- | --- | --- |
| 库存 | 精确计量、FEFO、非负、流水/Outbox 同事务、重放不重复扣库 | PostgreSQL 真实集成、守恒 fuzz；持续混合负载待验证 |
| 权限 | 当前家庭成员和角色由服务端判断；家庭切换不串数据 | 后端越权/角色场景，浏览器切换与降权 |
| 菜单 | 草案不扣库，确认重算，烹饪只扣一次 | API 契约链与桌面/手机 UI 成功链 |
| 采购 | 实购确认后入库，重复请求不重复增加库存 | UI 采购入库、幂等与软删除回归 |
| AI | 结构校验、取消/租约 fencing、人工确认 | 确定性 fixture；真实效果与费用待预算授权 |
| 统计 | 独立数据库、事件去重/隔离、故障不阻塞核心写入 | 真实 Redis/Kafka 本地恢复；生产配置待验收 |
| 营养 | 历史快照、未知覆盖明确、无虚构健康分数 | 真实数据库快照与覆盖规则 |
| 前端 | 交互闭环、错误状态、贴合食材的本地照片 | 桌面/手机 Chromium；真实用户研究待开展 |
| Harness | 可复现约束、契约、映射、完成证据 | full/acceptance 门禁；镜像绑定和剩余契约在台账 |

本表不使用没有基准的完成百分比或质量分数。台账中的 `checks` 是场景定位信息；文件存在不代表测试通过，实际结果必须来自当前源码的 full/acceptance 报告。full、fast 与 acceptance 的 `quality-map` 检查会拒绝丢失的业务域、owner、测试路径/锚点和不完整技术债。

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

普通全量测试执行 fuzz seed；额外 fuzz 命令探索更多输入，不能将有限时长运行当作形式证明。当前交付证据见[执行计划](exec-plans/quality-photos.md)。
