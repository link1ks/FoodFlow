# 本机学习版发布与回退

本轮交付的是学习发布流程，不是公网部署。家庭内测已按用户要求跳过，真实体验债务仍保留。无需购买服务，收费 AI/SMS 继续关闭。

## 准备一个可追溯版本

先提交所有源码，再在相同源码摘要上完成完整验证与独立验收。文档快照提交允许不同 Git revision，但不能改变验证输入。学习部署脚本把该摘要绑定到实际 API/Web 镜像，Worker 使用同一 API 镜像。

```powershell
go run ./cmd/harness -mode full -out .cache/harness/learning-release-full.json
go run ./cmd/harness -mode acceptance -out .cache/harness/learning-release-acceptance.json
pwsh -File scripts/learning-deploy.ps1 up
pwsh -File tests/learning-security.ps1
pwsh -File scripts/learning-release.ps1 prepare
```

`prepare` 要求 Git 工作区干净、完整 full/acceptance 检查通过且摘要一致、学习服务运行、API/Worker 镜像一致、实际镜像绑定正确。输出位于 `.cache/learning-releases/release-*`：已提交源码 ZIP、部署 Compose 和带 SHA256/长度/版本/验证 run ID/镜像精确 ID/迁移历史摘要的清单。路径白名单拒绝 `.env`（允许模板 `.env.example`）、数据目录、缓存、日志、私钥、路径穿越、重名与链接。包不含数据库、照片卷、凭证或认证日志。

**镜像只记录本机 ID，没有导出镜像 TAR。** 这样避免重复保存大镜像；不要清理需要回退的镜像。源码 ZIP 可以作为版本记录，迁到新主机仍需重新构建、初始化独立凭证并验收，不能把该包当作离线 Docker 镜像包。清单和本机标签并非签名或防篡改证明；接收别人的包时仍需审查来源。

## 检查和切换

将下面路径替换为 `prepare` 返回的实际目录：

```powershell
pwsh -File scripts/learning-release.ps1 check -Release .cache/learning-releases/release-实际版本
pwsh -File scripts/learning-release.ps1 switch -Release .cache/learning-releases/release-实际版本 -ConfirmSwitch
```

`check` 只读检查包、保留镜像、SQL 文件摘要、迁移历史、部署拓扑和数据库镜像。`switch` 必须显式确认，先生成[主库和照片配套备份](BACKUP_RECOVERY.md)，再在共享维护锁内重复检查；存在 queued/running 任务时拒绝切换。它用固定学习项目、精确镜像、`--no-build --no-deps` 只重新创建 API/Worker/Web，检查数据库容器身份与迁移历史未变、网关就绪。它不运行迁移、不替换数据库或图片卷。完成后再运行安全检查并用账号实际登录，确认照片、库存、历史与入库可用。

这适合**相同数据结构和拓扑**的应用更新/回退。SQL 文件、已应用迁移、数据库镜像或 Compose 改变时会拒绝直接回退；不要绕开检查。应在全新私密目标恢复旧版本配套备份并核对账号、库存、照片和幂等，再制定明确的切换与数据取舍方案。数据库恢复不写回当前学习数据库。相同 schema 也不证明任何两版的业务语义兼容，仍须针对具体版本验证。

停止/重新创建期间会短暂不可用。切换不是原子的；失败时保留备份、原镜像与失败状态，检查固定学习服务状态后恢复已知版本，不自动重复切换。并发成员写入与新任务可能出现在检查后；该流程要求操作者在安静的维护窗口操作，不是在线无损部署或任务队列排空协议。

## 验证范围

full 的 `release-regression` 检查私密路径/归档穿越/重复项、文件损坏、摘要漂移、验证缺失及 schema/history/topology/image 变化拒绝。`tests/learning-release.ps1` 在固定学习项目以当前版本进行真实重建，检查配套备份、数据库身份、七类库存/流水/幂等/成本表指纹、照片字节及部署安全。CI 同样执行，仅上传脱敏结果，不上传发布包或私人备份。

当前演练是同版本重新创建，尚未验证旧版业务兼容、异机恢复、HTTPS、域名、公网登录、真实账单和真实成员体验。实际证据见[交付计划](exec-plans/learning-release.md)。
