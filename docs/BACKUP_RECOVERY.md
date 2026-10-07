# 学习环境备份与恢复

适用范围：独立 `foodflow-learning` 项目的 PostgreSQL 16 与图片卷。先按 [学习部署](LEARNING_DEPLOYMENT.md) 启动 17173；现有开发环境、Kafka 与 insights 数据库不在这组命令内。工具不调用收费模型或短信服务，不清空原数据。

```powershell
pwsh -File scripts/learning-recovery.ps1 -Action backup
# 用上一命令实际返回的完整目录替换下面路径
pwsh -File scripts/learning-recovery.ps1 -Action verify -Backup D:\FoodFlow\.cache\learning-backups\backup-实际时间与编号
pwsh -File scripts/learning-recovery.ps1 -Action restore-check -Backup D:\FoodFlow\.cache\learning-backups\backup-实际时间与编号
```

备份短暂停止原先正在运行的 API/Worker，形成数据库和照片共同的静止点；完成或失败后恢复这些服务。期间页面可能显示服务暂不可用，请等待恢复。不要同时通过 SQL 或文件系统直接写数据。前后只读指纹必须一致，否则不生成完整清单，部分文件仍保留用于排查。锁文件防止同一工作区同时运行恢复工具；异常终止留下锁时，先确认没有维护进程，再由操作者处理，不自动移除旧锁。

完整备份目录含 `database.dump`（PostgreSQL custom 格式）、`images.tar` 与 `manifest.json`。原生二进制流避免 PowerShell 文本管道破坏数据。清单记录精确容器镜像 ID、文件长度/SHA256、所有 public 表行数/内容指纹、自增序号状态及照片目录内容指纹，不写账号、密码、令牌或原始业务行。活动图片任务引用的文件也必须存在。只有最后完成验证的目录才有清单。

备份文件**包含真实家庭数据和密码散列**，只允许当前本机用户读取；Linux 为目录 0700、文件 0600，Windows 使用当前用户专属 ACL。清单用于发现损坏，不提供签名或防恶意篡改认证。仅使用自己生成并可信保管的备份。备份没有另做加密，`.cache` 也不是异地存储；不要上传到 GitHub、聊天、普通共享目录或公开对象存储。学习配置及恢复配置同样保持私密。

恢复演练先检查两个文件的完整性和照片归档边界，再创建全新随机项目 `foodflow-restore-*`。数据库必须为空；[pg_restore 的单事务选项](https://www.postgresql.org/docs/16/app-pgrestore.html) 避免半套数据库提交，图片解包拒绝路径穿越、链接、重复路径、超限和覆盖已有文件。随后比较全部表内容、序号、图片路径/字节与有效图片引用。失败的目标也保留；演练结束停止目标，保留容器、卷和受保护的 `.cache/learning-restores/*.env`。

恢复目标没有主机端口、Worker 或网关，使用全新 JWT 密钥；原登录令牌在该目标失效，数据库里的账号密码仍能登录。工具不自动切换网站到恢复目标。当前演练依赖同机缓存的精确数据库/API 镜像，未验证新机器重建、磁盘损坏后的恢复或跨 PostgreSQL 大版本迁移；保留镜像源码/版本也是以后异地恢复方案的一部分。

运行实际账号/照片/幂等验收：

```powershell
pwsh -File tests/learning-recovery.ps1
```

验收创建虚构账号和番茄实拍照片，验证恢复后密码登录、300 g 库存、照片 SHA256、备份前请求重放不重复入库、新入库可写和匿名请求被拒绝；同时检查原账号/库存/照片仍可用。损坏文件只在新的私密副本内构造，必须在创建恢复目标前拒绝。虚构记录及停止的恢复目标保留，不删除用户数据。CI 仅上传汇总 `.cache/harness/learning-recovery.json`，不上传备份文件、清单或私密配置。

学习阶段目标：RPO 24 小时需每天手动完成一次备份，目前没有定时任务，实际可恢复点以最后一份成功备份为准。RTO 目标 10 分钟；本机小数据集的计时仅测演练，不代表故障机器或更大数据量的恢复时间。建议重大修改前额外备份，并在已有另一块受保护磁盘保管完整目录；当前尚未完成异机/异地恢复验收。自动保留期与清理尚未配置，工具不会删除旧备份和卷，需自行留意磁盘空间。

真实运行结果见 [学习交付记录](exec-plans/learning-release.md)。[平台运维](PLATFORM.md) 的 insights/Kafka 与投影重放恢复仍需单独验证。
