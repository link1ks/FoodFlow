# 阿里云 OSS 真实验证

2026-10-14，用户授权开通按量付费 OSS、创建专用私有测试 Bucket，以及生成仅限该 Bucket 对象上传、读取、删除的 RAM 凭据。北京地域、标准存储、本地冗余、阻止公共访问、OSS 托管 AES256 加密；未购买资源包。专用 RAM 用户没有控制台登录，策略仅含 `oss:PutObject`、`oss:GetObject`、`oss:DeleteObject`，资源为一个测试 Bucket 的对象。密钥仅保存本地 `.env`，一次性下载副本已删除。

## 真实结果

FoodFlow `storage.S3` 对文本、仓库公开番茄图片和合成音频格式字节分别执行 PUT、GET、DELETE、删除后 GET。三个对象均逐字节及 SHA256 一致；读取 MIME 正确，删除后均为 `NoSuchKey` / HTTP 404。修复后十二次请求全部符合预期，每个请求最多一次发送、禁止重定向。无数据库写入，测试对象已删除。

首次 PUT 复现 HTTP 403 `SecondLevelDomainForbidden`：原实现强制路径式地址，OSS 要求 Bucket 域名式地址。已仅对官方 S3 兼容主机启用域名式寻址，其他端点继续路径式；本地回归检查三种操作的主机、路径、字节和 MIME，以及伪装域名不会被当成 OSS。依据：[阿里云 AWS SDK 接入文档](https://help.aliyun.com/zh/oss/developer-reference/use-aws-sdks-to-access-oss)。第二次构建被本机 Go 缓存权限阻止，旧验证程序又发送了一次同类失败 PUT；重新编译成功后复测通过。验证程序共十四次对象请求，两个失败请求未写入对象，没有 SDK 自动重试。控制台操作产生的请求不在该程序计数中。

脱敏证据：[oss-smoke-20261014.json](evidence/oss-smoke-20261014.json)。不含 Bucket 名、对象键、账号 ID、凭据、认证头或原始日志。

## 配置与费用边界

服务端 S3 配置示例：

```dotenv
STORAGE_BACKEND=s3
S3_ENDPOINT=https://s3.oss-cn-beijing.aliyuncs.com
S3_REGION=cn-beijing
S3_BUCKET=your-private-test-bucket
S3_ACCESS_KEY=your-dedicated-ram-access-key
S3_SECRET_KEY=your-dedicated-ram-secret
```

实际默认站点仍用 `STORAGE_BACKEND=local`，现有图片无需迁移且继续可读。远程凭据已准备，但本次只验收存储适配器；未验收网站上传→OSS→模型 Worker 的组合链路，也未迁移旧对象。音频是存储字节夹具，不是可用于识别评估的有效录音。不得将本次结果解释为整个网站已改用 OSS。

对象合计 12,674 字节，成功读回一次后即删除。费用涉及请求、短暂存储与外网流出，未查实最终账单；开通页面的 ¥0 不代表后续调用免费。阿里云说明不足一分的小时费用可能累计后出账，空 Bucket 无存储容量费，因此不能凭当前未出账宣称消费为零。[OSS 计费说明](https://help.aliyun.com/zh/oss/billing/)。官方请求规则明确 4xx/5xx 不计请求费，故本轮两个 403 和三个删除后 404 不产生请求费；九次成功对象请求及控制台请求应按实际适用额度/单价对账。[请求费用规则](https://help.aliyun.com/zh/oss/api-operation-calling-fees)

本次没有授予列举 Bucket、其他 Bucket、ACL 修改或账号管理权限。私有 Bucket 和专用 RAM 用户保留供后续测试；仅清理本轮合成对象及过时验证程序，保留必要证据。
