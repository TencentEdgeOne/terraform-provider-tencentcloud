## Why

EdgeOne Makers 的项目、部署、环境变量、自定义域名目前没有 Terraform 资源。用户只能用 `terraform_data` 加 `local-exec` 调脚本，这样 Terraform 感知不到控制台上的改动，destroy 逻辑靠 provisioner 拼，也没法 `import`。Makers 接口已通过 API 3.0 公开（teo `2022-09-01`，CAM 鉴权），具备接入条件。

## What Changes

- 新增 4 个 framework 资源和 1 个 data source：
  - `tencentcloud_teo_makers_project`：上传类项目。`name`、`nodejs_version`、构建配置原地更新，`zone_id`、`area` 变化替换
  - `tencentcloud_teo_makers_project_env_vars`：项目环境变量的权威集合，支持按环境设置不同值
  - `tencentcloud_teo_makers_deployment`：上传本地目录或 zip 包并等待构建结束。plan 阶段计算源码哈希，内容变化才重新部署；构建失败时报错带构建日志，资源标记为 tainted
  - `tencentcloud_teo_makers_custom_domain`：绑定自定义域名，`environment` 原地切换，导出 `cname`、`status`、归属验证信息
  - data source `tencentcloud_teo_makers_zones`：列出账号下的 Makers 站点，给其他资源取 `zone_id`
- 新增 service 层 `service_tencentcloud_teo_makers.go`，统一封装转发调用。
- 用户可见的名称一律用 Makers；只有接口名（`*Pages*` Action、`pages:` Interface）和 `zone-type` 取值 `pages` 保留后端原名。
- vendor 补上 `terraform-plugin-framework-timeouts/resource/timeouts` 包（模块已在 `go.mod` 中，不改版本）。

非破坏性：只新增资源，不改已有资源的 schema 和 state。

## Capabilities

### New Capabilities
- `teo-makers-resources`: Makers 项目、环境变量、部署、自定义域名，以及 Makers 站点查询。

## Impact

- 代码：`tencentcloud/services/teo/` 新增 service、4 个资源、1 个 data source、校验器和测试；`tencentcloud/framework/registry.go` 注册
- 依赖：`tencentcloud-sdk-go` 没有 Makers 的类型化 Action，经 `tchttp.NewCommonRequest` 调 4 个转发 Action（`Create/Describe/Modify/DeletePagesResources`），客户端复用 `UseTeoClient()`，endpoint、凭据、日志与其他 teo 资源一致。上传制品用已 vendor 的 `cos-go-sdk-v5`
- 文档：framework 资源暂不支持 `make doc`，`website/docs/{r,d}/teo_makers_*.html.markdown` 手写；`provider.md`、`website/tencentcloud.erb` 同步
