Provides a resource to create an EdgeOne Makers project whose source is uploaded directly.

Example Usage

```hcl
data "tencentcloud_teo_makers_zones" "this" {}

resource "tencentcloud_teo_makers_project" "example" {
  zone_id        = data.tencentcloud_teo_makers_zones.this.zone_ids[0]
  name           = "my-makers-site"
  area           = "overseas"
  nodejs_version = "22.11.0"
  build_command  = "npm run build"
}
```

Import

teo makers project can be imported using the id, e.g.

```
terraform import tencentcloud_teo_makers_project.example zone_id#project_id
```
