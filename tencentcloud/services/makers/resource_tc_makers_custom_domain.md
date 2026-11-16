Provides a resource to bind a custom domain to an EdgeOne Makers project.

Example Usage

```hcl
resource "tencentcloud_makers_custom_domain" "example" {
  zone_id    = tencentcloud_makers_project.example.zone_id
  project_id = tencentcloud_makers_project.example.project_id
  domain     = "www.example.com"

  depends_on = [tencentcloud_makers_deployment.example]
}
```

Import

makers custom domain can be imported using the id, e.g.

```
terraform import tencentcloud_makers_custom_domain.example zone_id#project_id#www.example.com
```
