# 一个月学习部署

目标是学习部署与运维。当前没有云服务器，本阶段在本机验收；不购买域名、短信、托管数据库或 Redis/Kafka 云服务。

## 本机启动

需要 Docker Compose 与 PowerShell 7。

```powershell
pwsh -File scripts/learning-deploy.ps1 init
pwsh -File scripts/learning-deploy.ps1 up
pwsh -File scripts/learning-deploy.ps1 status
pwsh -File tests/learning-security.ps1
pwsh -File tests/smoke.ps1 -Base http://127.0.0.1:17173
```

访问 <http://127.0.0.1:17173>。这是独立的 `foodflow-learning` 项目、账号、数据库与图片卷，不迁移或清空 5173 的开发数据，也不复用 15173 验收数据。`init` 只运行一次；已有配置会拒绝覆盖。暂停用 `pwsh -File scripts/learning-deploy.ps1 stop`，数据卷保留。

凭证保存在 Git 忽略的 `.cache/learning.env`，仅创建者可读写。脚本使用随机数据库密码和 JWT 密钥，不输出凭证；不要提交此文件，也不要把 `docker compose config`、完整 inspect 或原始日志粘贴到 GitHub。这里的配置是两项生成凭证，不复制原 `.env` 中的 DeepSeek 等密钥。

## 已配置的安全边界

- 只有网页绑定 `127.0.0.1:17173`；API、PostgreSQL、Worker 不发布主机端口。数据库位于内部 Docker 网络，网关无法直接连接数据库。
- 原开发 Compose 的网页 5173 和 API 8080 也改为回环绑定，下次按该配置重新创建服务后生效。此阶段未用学习配置替换原开发卷。
- API/Worker 用 UID 10001，网页用 UID 101；只读根文件系统、去除全部 Linux capabilities、禁止提权。临时目录用有大小上限的 tmpfs；图片用独立可写卷。一次性初始化只更改新学习卷根目录的所有者，不递归修改已有文件。
- PostgreSQL 使用官方镜像原有初始化权限；它不是非 root 应用容器的验收对象。容器日志按每文件 10 MB、最多 3 文件轮转；这不是完整资源容量或磁盘告警方案。
- 网关覆盖调用者传来的 IP/协议头。API 只信任固定网关 IP `172.29.170.10` 的 `X-Real-IP`，默认部署仍不信任代理头；拒绝 CIDR/任意地址信任。网段冲突时需同时调整 Compose 网段、网关 IP 和 `TRUSTED_PROXIES` 后重新验证。
- CSP 禁止执行内联脚本和跨站资源，防嵌入、禁止 MIME 嗅探、隐藏 referrer；React 动态样式仍允许 inline style。账户接口不缓存，带哈希静态资源长缓存。错误响应保留安全头。
- 模型、短信与外部菜价同步在该配置中明确关闭，不因现存密钥而产生费用。当前学习环境使用确定性演示规划；AI 限额将在第三阶段实现后再启用真实模型。

SSH 隧道会让多位成员在网关处显示同一个传输来源，认证预算共享属于此访问方式的限制。不能根据可伪造的请求头绕过限制；公开访问与多成员访问方式在后续阶段重新验收。

## 后续迁到云服务器

先在供应商订单确认试用资格或一个月实际付款、磁盘和流量费用；这份配置不执行购买。云端需先安装 Docker Compose 与 PowerShell 7，在新目录初始化配置，保持相同的独立卷与端口边界。

云主机安全组只开放 SSH 给自己的来源地址；浏览器通过加密 SSH 隧道访问回环端口，不开放 17173、8080、5432 到公网。例如：

```powershell
ssh -N -L 17173:127.0.0.1:17173 your-user@your-server
```

随后在自己的电脑打开 <http://127.0.0.1:17173>。本机与云端不能同时占用同一转发端口。当前没有执行云端访问、SSH 加固或 HTTPS 验收；公开发布还需要实际域名、HTTPS 网关、主机与依赖维护、监控、恢复与权限验收。不能把本机通过写成公网可发布。

## 一个月结束前

按 [数据库与图片备份恢复](BACKUP_RECOVERY.md) 生成完整备份，并在空目标验证恢复，再决定续费。当前支持本机独立演练，异地恢复和实际云端迁移仍需另验收；不要删除带数据的卷，也不要只依赖试用到期的数据保留期。
