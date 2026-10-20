# Python Agent 迁移与上手步骤

任务：`python-agent-migration`，2026-10-20，feature，baseline unknown。
状态：最终本地 full/acceptance/diagnose/main-site 已通过，默认部署已
切换到 Python Agent；准备发布并等待当前提交的云端完成证据。

2026-10-20 最新指示：用户将后续命令执行交由助手，取代下面历史步骤
中的手动分工。助手继续解释关键操作，负责绑定、完整验收、配套备份、
默认部署更新、发布及完成门禁，不再要求用户操作命令。

## 分工

用户已选择：助手完成代码与自动化测试；用户亲手初始化 Python 环境、
运行选定的关键测试、执行数据库与照片备份、切换部署和发布。
每次交付一个可理解、可验证的操作阶段，检查实际输出后继续。

## 迁移前已核实的现状

- `internal/agent` 是 Go 实现，文本模型接口依赖 Eino。
- 菜单推荐与营养建议的提示构造还位于 `internal/app`。
- 图片识别、语音转写、社区初审也属于迁移范围。
- PostgreSQL 任务领取、租约、权限及收费调用保护由 Go 应用拥有。
- 模型返回后仍需校验候选菜谱、目录匹配、当前权限及取消状态。
- 本机 Python 为 `E:\ANACONDA\python.exe`，版本 3.13.11，
  可导入 venv 和 ensurepip；`py` 启动器未发现已注册解释器。
- 工作分支为 main；开始检查时工作区干净。

## 迁移边界

Python 拥有五类 Agent 的提示构造、模型请求、响应解析及结构化输出。
演示模式也要经过 Python，避免默认验收只验证旧 Go 路径。
迁移后移除 Eino 依赖。

Go 保留 API、授权、数据库事务、任务队列、确定性数量计算、
收费额度和最终业务校验。Python 不获得数据库凭据或直接写库存能力。
保留 Go 桥接与业务校验，不保留旧模型实现作为静默回退。

优先验证同容器 Python 子进程方案，以有界 JSON stdin/stdout 通信，
每任务一个子进程，Go 管理超时及取消；子进程环境采用允许列表。
凭据不能通过命令行参数或日志传递。

收费调用握手：Python 准备确切请求并报告字节数，Go 完成持久化
额度及租约检查后才允许派发。没有允许不得调用模型。
进程死亡、协议损坏及网络歧义时保守保留不确定占额，禁止自动重试。
该协议已实现并通过本地定向故障测试，仍需完整部署验收。

## 第一个手动步骤

在 PowerShell 执行：

```powershell
Set-Location E:\FoodFlow
powershell -NoProfile -ExecutionPolicy Bypass -File .\scripts\prepare-python-agent.ps1
```

脚本使用现有 Python 3.13，在 Git 忽略的 `.cache/python-agent-venv`
中创建独立环境。不安装第三方依赖、不读取 `.env`、不访问数据库、
不调用模型。重复执行检查并复用已有环境，不删除目录。

预期输出包括项目解释器路径、3.13 版本、pip 版本和
`Python agent environment ready`。失败时保留错误输出交回诊断。
学习重点：全局解释器提供运行时，虚拟环境隔离项目依赖。
以后用 `.cache\python-agent-venv\Scripts\python.exe` 指定项目环境，
无需改变 PowerShell 激活策略。

## 后续实现和验收

1. 实现 Python Agent、跨语言协议和 Go 桥接，迁移五类能力。
2. 覆盖确定性模型夹具、未知字段、越界候选、提示注入作为数据、
   超时、取消、额度拒绝、结果不确定、重复任务和旧租约等场景。
3. 将 Python 测试接入 harness/CI；将源码与依赖纳入 source digest、
   业务所有权和知识清单。通过命名桥接集成测试绑定迁移需求。
4. 更新 Docker、默认/验收/学习部署及恢复与发布验证，确保真正执行
   Python Agent，而非仅增加一个没有被调用的 Python 文件。
5. 语义复核业务文档与绑定；绑定不可变任务合同后串行运行 full、
   acceptance，再完成 diagnose，保留失败证据。
6. 用户执行准备好的数据库/照片备份及默认部署切换；助手核对数据、
   配置保留情况与 localhost:5173 构建身份。
7. 用户执行已审查的 Git 发布；助手检查当前提交云端结果，生成
   version-2 完成证据并记录 task outcome。

未经另行明确授权，不做真实付费模型验证。

## 当前证据与限制

用户已手动创建 Python 3.13.11 环境，返回 pip 25.3 及
`Python agent environment ready`。harness task start 已记录。

五类模型能力及菜单/营养演示模式已接入 Python；Eino 依赖已移除。
Python 仅使用标准库，无 pip 依赖；源码嵌入 Go 二进制。Docker
运行镜像和 CI 已配置 Python 3.13，部署尚待验证。

本地 `.cache/harness/python-agent-targeted.jsonl` 记录定向回归通过：
Agent 测试包含七个 Python unittest、中文输出、授权握手及取消；
厨房测试覆盖额度、社区审核/撤下/重启、图片确认、菜单不变量、
营养建议及语音授权/确认/租约。使用临时 PostgreSQL 容器及本地模型
夹具，没有访问默认数据库或收费模型。这不是最终完成 bundle。

发现并修复：Windows 默认输出编码损坏中文，改为明确 UTF-8 stdout；
通用错误覆盖原有语音提示，恢复安全的中文状态信息。原有相关断言
保留并通过。首次沙箱运行受 socket 权限 10013 限制；允许本地夹具
访问后才暴露编码回归。一次业务更新因生成清单过期返回失败，
business-map 本身通过；清单更新后重新检查。

后续 architecture、repository-hygiene、knowledge、business-map 检查均通过；
`go test ./cmd/harness` 及变更包 `go vet` 通过。工作区 diff 空白检查通过。

完整 full、acceptance、diagnose、主站备份/切换/只读验证、用户发布
及对应 cloud 证据均待完成，不得声称 localhost:5173 已使用 Python。

## 第二个手动步骤：运行并理解回归测试

```powershell
Set-Location E:\FoodFlow
.\.cache\python-agent-venv\Scripts\python.exe -I -B .\internal\agent\python\test_runtime.py
go test -count=1 -v ./internal/agent
```

第一条直接运行七个 Python 离线测试，预期 `Ran 7 tests` 和 `OK`。
`-I` 隔离用户 Python 配置，`-B` 禁止生成源码目录字节码缓存。
第二条检查 Go 真正启动 Python、中文结果、调用前额度拒绝、
重定向不重发和取消后保留不确定占额。`-count=1` 禁用测试结果缓存。
预期 `PASS` 和 `ok foodflow/internal/agent`。无需配置真实模型密钥。
检查用户实际输出后再进入完整验收。

用户已报告以上两条命令均通过（Python OK、Go PASS）。这是用户反馈，
不是新生成的 harness 完成证据。已准备
[不可变任务合同](../quality/tasks/python-agent-migration.json)，由用户在
最终验证前执行 task bind。随后串行运行 full、acceptance、diagnose，
报告使用时间戳避免覆盖先前尝试。Windows 浏览器选择本机已安装的 Edge，
保留独立桌面/移动上下文。期间保持源码不变；任何修复后须重新生成
匹配源码的证据。后续主站切换、main-site、发布、cloud 和完成 bundle
仍按用户亲手操作的分工继续。

## 最终本地验证与部署记录

后续操作已按用户最新指示全部由助手执行。任务合同在最终验证前绑定。
最终源码摘要为
`1ca4ddc8bcd5f43cf10b958c8e701612c52ec19106a96d90a36ff372b8d5f877`。

- full `11906b855cb1fbc05c028d8f62dcfb5b`：18/18 检查、155 个命名结果通过。
  10 家庭、20 客户端、30 秒完成 3,645 周期和 25,515 请求，错误及不变量
  失败为零，p95 17.371 ms。该负载不包含模型调用，不代表生产容量。
- acceptance `a5a0cd1d50e05faa5191fb1b066f9581`：17/17 检查、
  42 个桌面/手机浏览器结果通过，无跳过或重试；使用本机 Edge。
- diagnose `e40cdfc3442446e9ca70bf40e620d298`：5/5 通过。
- main-site `8a0ca8578a809f4db0b13c21e39630ea`：5/5 通过，新的无登录
  浏览器仅执行 GET/HEAD，确认首页、就绪状态及资源哈希匹配 web/dist。

第一次 acceptance 为 41 通过、1 失败：库存用例在元素截图后，营养浮层
关闭按钮被固定导航拦截，继而发生鼠标离开和浮层消失。单独复跑及三次
重复均通过，提示截图滚动与瞬时悬停状态存在偶发干扰。改为整页截图，
避免元素截图重定位；保留延迟显示、内容及正常点击关闭的全部断言。
桌面/手机各三次重复通过后，重新运行完整 full 和 acceptance，
未用旧源码的 full 或局部通过代替最终证据。初次报告及失败截图保留本地。

主站应用镜像重建后，在私密
`.cache/default-backups/pre-python-agent-20261020T110852Z` 完成并验证
数据库 custom dump 和照片归档。暂停写入期间两次完整快照一致，归档
目录、文件数量、字节和摘要校验通过；切换前后数据库容器身份、全部既有
API/Worker/Web 环境值、挂载和数据库/照片快照一致，`.env` 未改写。
新 Worker 运行 Python 3.13.10，本机开发环境为 3.13.11。
主站首页和 readiness 返回 200。没有默认站点合成写入或付费模型探测。

部署前置失败也保留：原容器引用的旧镜像 ID/部分内容已不在本地镜像库，
导致恢复工具无法启动；直接镜像快照也失败。随后从原容器导出文件系统，
导入仅本地回退镜像，不继承环境变量值；配置保存在私密文件中。照片与
数据库另行配套备份，文件系统导出不替代数据卷备份。首次切换使用了当前
Compose create 不支持的 --no-deps，自动回退后主站 readiness 200；
核实 up --no-start --no-deps 支持后重试成功。没有删除数据库或照片卷，
所有回退镜像、私密配置和备份只留本机，不发布。

最终本地报告为 `.cache/harness/python-agent-full-2.json`、
`python-agent-acceptance-2.json`、`python-agent-diagnose.json`、
`python-agent-main.json` 和 `python-agent-deployment.json`。
云端检查必须绑定随后发布的当前提交。只有 cloud 成功且 version-2
bundle 通过 task finish 才能结束任务；本段不预先声称云端成功。
