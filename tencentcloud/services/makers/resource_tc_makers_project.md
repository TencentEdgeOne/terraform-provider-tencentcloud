Provides a resource to create an EdgeOne Makers project whose source is uploaded directly.

Example Usage

```hcl
data "tencentcloud_teo_zones" "makers" {
  filters {
    name   = "zone-type"
    values = ["pages"]
  }
}

resource "tencentcloud_makers_project" "example" {
  zone_id        = data.tencentcloud_teo_zones.makers.zones[0].zone_id
  name           = "my-makers-site"
  area           = "overseas"
  nodejs_version = "22.11.0"
  build_command  = "npm run build"
}
```

Import

makers project can be imported using the id, e.g.

```
terraform import tencentcloud_makers_project.example zone_id#project_id
```
