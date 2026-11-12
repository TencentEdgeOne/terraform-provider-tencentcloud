## 1. Service 层

- [x] 1.1 新增 `tencentcloud/services/teo/service_tencentcloud_teo_makers.go`：转发调用 `call`、项目、环境变量、环境、COS 上传、部署、构建日志、自定义域名的封装，以及源码文件列举和哈希
- [x] 1.2 新增 `teo_makers_validators.go`：取值枚举、正则、首尾空格校验

## 2. 资源

- [x] 2.1 `resource_tc_teo_makers_project.go`：schema、CRUD、导入（`zone_id#project_id`）
- [x] 2.2 `resource_tc_teo_makers_project_env_vars.go`：schema、ValidateConfig（空集合、重叠环境）、按 Key + 环境对齐的增删改、导入
- [x] 2.3 `resource_tc_teo_makers_deployment.go`：schema（含 `timeouts.create`）、ModifyPlan 计算哈希、上传部署轮询、失败带日志、导入（`zone_id#project_id#deployment_id`）
- [x] 2.4 `resource_tc_teo_makers_custom_domain.go`：schema、绑定、切环境、解绑、导入（`zone_id#project_id#domain`）
- [x] 2.5 在 `tencentcloud/framework/registry.go` 注册全部资源和 data source
- [x] 2.7 `data_source_tc_teo_makers_zones.go`
- [x] 2.6 `go mod vendor` 补上 `terraform-plugin-framework-timeouts/resource/timeouts`

## 3. 测试

- [x] 3.1 单元测试 `service_tencentcloud_teo_makers_test.go`：Action 选择、环境变量对齐、读回时环境集合的处理、源码哈希、构建日志格式化
- [x] 3.2 验收测试 `resource_tc_teo_makers_project_test.go`：创建、改名并改环境变量触发重新部署、从 `source_dir` 切到 `source_zip` 重新部署、三个资源的导入校验
## 4. 文档

- [x] 4.1 全部资源和 data source 的 `.md` 示例
- [x] 4.2 `make doc` 生成 `website/docs/{r,d}/teo_makers_*.html.markdown`
- [x] 4.3 `tencentcloud/provider.md`、`website/tencentcloud.erb` 加入全部资源和 data source

## 5. 验证

- [x] 5.1 `go build`、`go vet ./tencentcloud/services/teo/`
- [x] 5.2 `go test ./tencentcloud/services/teo/ -run TestMakers`
- [x] 5.3 `TF_ACC=1` 跑 `TestAccTencentCloudTeoMakersProjectResource_basic`
- [x] 5.4 `make check-mux` 通过；golangci-lint v2 对 teo 包 0 issues（tfproviderlint 未装，只针对 SDKv2）
