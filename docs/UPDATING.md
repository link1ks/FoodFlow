# 一次下载，后续一键更新

适用于每台电脑独立保存 FoodFlow 项目的使用方式。Windows 需要安装 [Git](https://git-scm.com/downloads)；更新源码不需要 Go、Node.js 或 Docker。

## 首次使用

用 Git 下载一次（文件夹可以改为自己的路径）：

```powershell
git clone https://github.com/link1ks/FoodFlow.git E:\FoodFlow
```

已有通过 Git 下载的项目可以继续使用。GitHub 的 **Download ZIP** 不带 `.git` 更新记录，需要先用 `git clone` 建立可更新的项目，再复制自己的 `.env` 等本地配置；不要覆盖已有目录或删除原来的数据。

## 以后更新

- 双击根目录的 `check-update.cmd`：查看远端新提交数，不修改项目源码。
- 双击根目录的 `update.cmd`：只获取当前分支上游的新提交，快进更新，无需重新下载整个项目。

两者都需要联网。窗口显示当前与远端提交编号；失败时显示原因。启动器支持 Windows 自带的 Windows PowerShell 5.1，路径含空格也可以运行。

命令行也可以运行：

```powershell
cd E:\FoodFlow
powershell -NoProfile -ExecutionPolicy Bypass -File scripts/update.ps1 -Action check
powershell -NoProfile -ExecutionPolicy Bypass -File scripts/update.ps1
```

更新跟随当前分支配置的远端上游（首次 clone 默认是 `main` → `origin/main`）。本地有修改、未跟踪文件、本地独有提交或分支没有上游时，更新会停止。先自行提交或移走修改、处理分支差异，再重新更新。脚本不会自动 stash、reset、rebase 或覆盖你的修改。检查更新允许存在本地修改。

`.env`、`.cache`、本地 `data` 与 Docker 数据卷不属于源码更新内容。远端若引入这些私有路径，会拒绝更新。同一项目同时运行第二个更新器也会停止。

## 更新运行中的应用

源码更新后，正在运行的应用仍使用旧构建，需要重新构建。使用默认 `compose.yaml` 部署时，先备份该部署的数据库和照片，再执行（[本地验收备份](LOCAL_DEPLOYMENT.md) 的脚本仅备份独立验收环境，不会备份默认部署）：

```powershell
powershell -NoProfile -ExecutionPolicy Bypass -File scripts/update.ps1 -Restart
```

此命令先检查 `.env` 和 Docker，再更新源码，最后执行默认 Compose 的构建和启动，并等待健康检查。即使源码已经最新，也会重试构建启动。它使用原项目目录和已有数据卷；迁移可能升级数据库结构，Git 回退不等于数据库回退。构建失败会返回失败，保留已更新源码和数据，修复后可重试。

如果使用 `compose.trial.yaml`、学习部署或其他自定义 Compose 配置，先双击 `update.cmd`，然后用原部署方式重建，例如 `pwsh -File scripts/local-trial.ps1 up`。学习部署应遵循 [发布流程](LEARNING_RELEASE.md)。`-Restart` 只适用于默认 Compose，不切换这些部署。

## 多电脑与发布新代码

每台电脑首次 clone 一次，以后双击更新即可。新功能必须先提交并推送到 GitHub，其他电脑才能获取；未推送的本地修改无法通过更新同步。

此功能同步**代码**。每台独立部署的账号、库存和照片保存在各自数据库/数据卷中，不会自动跨电脑同步。若需要多台电脑共享同一家庭数据，应访问同一个部署，或按 [配套备份恢复](BACKUP_RECOVERY.md) 迁移数据库与照片。
