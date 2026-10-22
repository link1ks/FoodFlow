# 项目文档

## 使用与部署

| 文档 | 内容 |
| --- | --- |
| [项目首页](../README.md) | 功能、快速启动、更新与开发入口 |
| [模块操作实录](SHOWCASE.md) | 循环 GIF、关键状态变化与动图维护 |
| [个人厨房](PERSONAL_KITCHENS.md) | 个人默认厨房与可选家庭共享 |
| [配方社区](COMMUNITY.md) | 发布、封面初审、每周精选、积分与管理员设置 |
| [一键更新](UPDATING.md) | 首次下载、增量更新与 Docker 重建 |
| [家庭试用](FAMILY_TRIAL.md) | 协作、厨房闭环与试用检查 |
| [独立本机部署](LEARNING_DEPLOYMENT.md) | 独立数据库、端口与安全配置 |
| [备份恢复](BACKUP_RECOVERY.md) | 数据库与图片共同备份、空目标恢复 |
| [发布与回退](LEARNING_RELEASE.md) | 交付包检查、重建及数据库兼容边界 |
| [账号与 AI 额度](ACCOUNT_RECOVERY_AI.md) | 找回、会话撤销与收费调用保护 |

## 开发与维护

| 文档 | 内容 |
| --- | --- |
| [架构](ARCHITECTURE.md) / [服务归属](PLATFORM.md) | 业务约束、事务与事件边界 |
| [模块导航](../AGENTS.md) | 源码地图与维护规则 |
| [API 契约](../api/openapi.yaml) / [生成清单](generated/contracts.md) | 接口、依赖与配置事实 |
| [开发与贡献](CONTRIBUTING.md) / [安全](SECURITY.md) | 修改、交付与安全问题报告 |
| [Harness](HARNESS.md) / [验收部署](LOCAL_DEPLOYMENT.md) | 可执行检查与隔离环境 |
| [验证说明](VERIFICATION.md) / [质量台账](QUALITY.md) | 可复现场景、验证范围与未完成项 |
| [平台维护计划](exec-plans/platform.md) / [Harness 约束](exec-plans/harness-hardening.md) | 当前维护与交付证据 |
| [应用图标](assets/README.md) | 品牌资产、生成方式与使用位置 |
| [更新日志](CHANGELOG.md) | 用户可见变更 |

仓库保留源代码、契约、部署工具、测试、授权资产和持续维护说明。已结束的逐次执行记录与旧运行摘要不随源码发布；历史内容可从 Git 历史查阅，新的本地运行证据保存在 Git 忽略的 `.cache/harness`，云端结果绑定到 GitHub Actions 的具体提交。
