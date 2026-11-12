## Context

Makers 接口不在 SDK 里，通过 teo 的 4 个转发 Action 调用。请求体顶层是 `ZoneId`、`Interface`（`pages:<接口名>`）、`Payload`（业务参数的 JSON 字符串），结果是 `Response.Result` 里的 JSON 字符串。Action 动词必须和接口名一致，比如 `pages:CreatePagesProject` 走 `CreatePagesResources`。转发 Action 对任意 `pages:*` 接口通用，新增接口不用新的 Action。

## Decisions

### 转发调用

`TeoMakersService.call` 按接口名前缀选 Action，用 `tchttp.NewCommonRequest("teo", "2022-09-01", action)` 发到 `UseTeoClient()`。响应类型嵌入 `tchttp.BaseResponse`，错误按 SDK 错误返回，`ResourceNotFound` 视为不存在。以后 SDK 有了类型化 Action，只换 service 层，schema 和 state 不变。

### 项目

- `CreatePagesProject` 不幂等，失败不重试。后端在站点内校验重名。
- `ModifyPagesProject` 只传变化的字段：后端拿 `Name` 查重时会命中项目自己，原样传当前名字会报重名。
- `Provider` 固定 `Upload`、`Channel` 固定 `Custom`、`Source` 为 `terraform`。

### 环境变量

后端模型：同一 Key 可以有多行，各行的生效环境（`Env`）两两不相交；不传 `Env` 时写入为项目当前全部环境。`DescribePagesProjectEnvs` 返回明文，能检测漂移。

- schema 是 set，每项 `key`、`value`（sensitive）、`comment`、`environments`。`environments` 不填表示全部环境。
- apply 时按 (Key, 展开后的环境集合) 对齐远端行：值或备注变化的带 `Id` 更新，远端多余的按 `Id` 删除，缺的新建。先删后写，避免环境从一行挪到另一行时触发重叠校验。按 Key 删会删掉该 Key 的所有行，所以只按 `Id` 删。
- 读回时，覆盖全部环境的行写成 `environments = null`，除非原 state 显式列出了同样的集合。
- 后端会去掉首尾空格，配置里带空格会永远有 diff，在校验阶段拒绝。空的 `comment`、`environments` 读回会变成 null，也在校验阶段拒绝。

### 部署

- 源码来自 `source_dir` 或 `source_zip`，二选一，文档推荐 zip。
- `ModifyPlan` 计算 `source_hash`：目录按排序后的 `<相对路径> <sha256>` 行拼接再做 sha256，跳过 `node_modules`、`.edgeone`、`.env`、`.git`、`.DS_Store`；zip 先校验扩展名和文件头，再取整个文件的 sha256。哈希变化标记替换。
- state 里没有哈希（导入的部署）时不替换，只补上哈希，便于从脚本方案迁移。
- Create：取 COS 临时凭据。目录 8 路并发上传，`CreatePagesDeployment` 传 `DistType=Folder`、`TempBucketPath` 为目标前缀；zip 上传到 `<TargetPath>/<文件名>`，传 `DistType=Zip`、`TempBucketPath` 为该对象 key。之后每 5 秒轮询到结束，受 `timeouts.create` 控制（默认 20 分钟）。拿到 `DeploymentId` 后立即写 state，失败时资源被标记为 tainted。失败时取 `DescribePagesDeploymentLog` 最后 50 行放进报错，去掉 ANSI 颜色码。
- Delete 只移出 state：部署是可回滚的历史，正在服务生产的部署也删不掉。
- 环境变量变化后要重新部署才生效，交给用户写 `replace_triggered_by`，资源之间不做隐式联动。

### 自定义域名

- `CreatePagesZoneCustomDomain` 带 `Env` 绑定，`ModifyPagesZoneCustomDomainEnv` 原地切环境。两者都要求目标环境有成功部署。
- 解绑失败时再查一次，域名确实不在了就当成功，不靠匹配错误消息。
- 归属验证、证书不在本资源内做：验证用 `tencentcloud_teo_ownership_verify`，证书要等 `cname` 写入后再配。

### 命名

资源、文件、Go 标识符、描述都用 Makers。保留后端原名的只有 Action（`*PagesResources`）、Interface（`pages:*`）、响应字段（如 `PagesDomains`）和 `DescribeZones` 的 `zone-type` 取值 `pages`，`tencentcloud_teo_makers_zones` 把后者封装掉。

## Risks

- 转发 Action 的 `Payload` 是不透明字符串，接口参数变更不会体现在 SDK 版本上。缓解：service 层用结构体收敛参数，验收测试覆盖主流程。
- 上传期间临时凭据 30 分钟过期，超大目录可能失败。缓解：推荐 `source_zip`，只上传一个对象。
