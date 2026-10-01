# 保存的 DeepSeek 额度接入

2026-10-01，用户明确要求“先用我 deepseek 的额度吧”。本任务授权使用已保存的官方 DeepSeek API，取代此前本地试用只用演示模型的运行设置；不授权 Google、其他模型服务、充值或真实短信。harness task：`enable-deepseek-20261001`（feature，baseline unknown）。

## 实施与证据

- 只读核对已保存配置：HTTPS 官方 `api.deepseek.com`、`deepseek-flash`，密钥存在，未显示或提交。查询余额确认账户可用，具体余额只保存在本机 `.cache`，不发布 GitHub。
- `cmd/modelcheck` 三次真实合成请求：menu、nutrition、vision 全部 PASS；使用虚构菜谱、生菜及程序绘制的番茄图，没有传输用户厨房记录或照片。两次余额读数在供应商返回的金额精度内相同，不能宣称调用免费或准确费用为零。没有重复收费测试。
- 新增 `compose.trial-deepseek.yaml`，启动脚本须显式 `-UseDeepSeek`，默认仍为确定性模式。文字与视觉都使用相同 DeepSeek 配置；启动前限定官方 HTTPS 主机、默认端口、路径与 Flash 名称，避免把保存的密钥发送给其他服务商。短信、菜价同步和本地存储覆盖保留。
- 实际 Compose 合并核对：默认六个模型字段全为空；DeepSeek 模式配置完整且文字/视觉凭据范围一致；两种模式短信关闭、菜价同步关闭、存储为 local。启动前确认没有 queued/running 模型任务积压，不删除历史任务或数据。
- 批量补丁与替换曾部分应用：发现剩余知识清单尚未更新，逐文件核对后补齐；没有把失败命令记作通过验证。
- 启动、完整 harness、重建隔离验收和 GitHub 同步结果待实际运行后补录。完整检查与验收仍使用确定性 fixtures，不读取模型密钥；CI 不承担真实模型费用。

## 限制

这三次检查只证明对应合成输入的连接与解析通过，不建立真实图片识别准确率、菜单接受率、长期可用性或成本分布。AI-EVAL 继续保留。启用后用户发起模型功能会消耗 DeepSeek 额度；库存与采购仍需明确确认。短信和 Google 云额度未启用。
- 最终本地 full 12/12（`7d45152c85b8b37d4102523ae5c6252c`）、重建 acceptance 13/13（`2b6b7efacd5881f559e19469b0fd8556`），同摘要 `f1973c90f9041dc49f3997e621a70559e7e3769e8040ca9013f50e3a247d7a72`；浏览器 22 通过、0 失败/跳过/flaky。六种虚构 HTTP/仿冒主机/非默认端口/错误路径/userinfo/非视觉模型配置均在 up 前拒绝，没有发出供应商请求。实际运行 API/Worker 文字与视觉模型均开启且同源，短信关闭、本地存储、健康 HTTP 200；原有账号/家庭/批次/流水记录数未变。便携证据：[deepseek-enable](../validation/deepseek-enable-20261001.json)。
