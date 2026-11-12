Provides a resource to manage the complete set of environment variables of an EdgeOne Makers project.

Example Usage

```hcl
resource "tencentcloud_teo_makers_project_env_vars" "example" {
  zone_id    = tencentcloud_teo_makers_project.example.zone_id
  project_id = tencentcloud_teo_makers_project.example.project_id

  env_vars = [
    { key = "NODE_NAME", value = "edge" },
    { key = "API_BASE", value = "https://api.example.com", environments = ["Production"] },
    { key = "API_BASE", value = "https://api-preview.example.com", environments = ["Preview"], comment = "preview backend" },
  ]
}
```

Import

teo makers project env vars can be imported using the id, e.g.

```
terraform import tencentcloud_teo_makers_project_env_vars.example zone_id#project_id
```
