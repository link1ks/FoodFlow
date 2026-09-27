# 本地部署与恢复验收

需要PowerShell 7、Docker Desktop、Go及前端依赖。此环境只绑定本机，不是公网部署；没有购买服务器或调用付费模型。

```powershell
pwsh -File scripts/local-acceptance.ps1 up
pwsh -File tests/smoke.ps1 -Base http://127.0.0.1:18080
pwsh -File scripts/local-acceptance.ps1 restart
pwsh -File scripts/local-acceptance.ps1 backup
pwsh -File scripts/local-acceptance.ps1 restore-check
pwsh -File tests/worker-recovery.ps1
go run ./cmd/loadcheck -requests 500 -concurrency 8 -out .cache/loadcheck.json
```

访问 http://127.0.0.1:15173 。API http://127.0.0.1:18080 。配置自动生成在被Git忽略的 `.cache/acceptance.env`，不要提交或覆盖已有文件，否则数据库密码与签名密钥可能不一致。

使用独立的foodflow-acceptance项目与专属数据卷，不连接日常开发数据库。Smoke创建专用测试账号和家庭，不清空已有数据；压测每次新建20种测试食材，仅允许18080本机端口。

备份分别保存PostgreSQL自定义格式归档与图片tar。恢复检查暂停验收 API/Worker，创建新的restore_check时间戳数据库，并断言流水指纹、用户数、迁移版本一致，最后恢复服务。图片恢复应在新的空目录/测试卷执行并核对文件哈希；正式恢复还须配置一致的图片存储与环境密钥。本脚本不自动删除恢复库或数据卷。

重启后检查ready和主流程；API收到SIGTERM停止接收新连接并等待最多10秒，长连接超时后关闭。生产部署还需要HTTPS、网络访问控制、外部密钥管理、监控告警及异地备份。

## 常见阻塞

- Docker镜像下载失败并提到127.0.0.1代理：检查Docker Desktop代理设置与代理进程，不能据此判断代码镜像构建失败。
- 端口已占用：先确认占用者，不终止未知服务。此验收环境默认使用15173/18080。
- Go数据库测试被跳过：使用REQUIRE_INTEGRATION=true，CI会将Docker不可用视为失败。
- 完整证据见 `docs/VALIDATION_REPORT.md`，未运行的项目不标为通过。
