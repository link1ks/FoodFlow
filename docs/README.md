# 文档导航

## 设计与开发

- [产品与启动](../README.md)、[开发与交付](../CONTRIBUTING.md)、[安全说明](../SECURITY.md)
- [业务架构](../ARCHITECTURE.md)、[模块导航](../AGENTS.md)、[服务归属](PLATFORM.md)
- [HTTP 契约](../openapi.yaml)、[生成契约与配置清单](generated/contracts.md)
- [Harness 工作流](HARNESS.md)、[质量与未完成项](QUALITY.md)

## 部署与运行

- [开发与隔离验收部署](LOCAL_DEPLOYMENT.md)
- [本机学习部署](LEARNING_DEPLOYMENT.md)
- [数据库与图片备份恢复](BACKUP_RECOVERY.md)
- [账号找回与 AI 限额](ACCOUNT_RECOVERY_AI.md)
- [发布包、更新与回退](LEARNING_RELEASE.md)
- [家庭试用场景](FAMILY_TRIAL.md)

## 验证与变更记录

- [更新日志](../CHANGELOG.md)
- [交付阶段与实际验证](exec-plans/learning-release.md)
- [仓库整理记录](exec-plans/repository-hygiene.md)
- [平台实施计划](exec-plans/platform.md)、[Harness 加固记录](exec-plans/harness-hardening.md)
- [历史性能测量](VALIDATION_REPORT.md)、[历史平台故障验收](PLATFORM_VALIDATION.md)

`exec-plans` 保存有来源的实现决策、失败和完成证据，`validation` 保存便携脱敏摘要。历史报告中的测试数量和性能仅属于其标注版本；当前结论以对应提交的 CI 和新运行报告为准。私人配置、备份、截图及原始运行产物保存在 Git 忽略的本地目录。
