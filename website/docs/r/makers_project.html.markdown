---
subcategory: "EdgeOne Makers(MAKERS)"
layout: "tencentcloud"
page_title: "TencentCloud: tencentcloud_makers_project"
sidebar_current: "docs-tencentcloud-resource-makers_project"
description: |-
  Provides a resource to create an EdgeOne Makers project whose source is uploaded directly.
---

# tencentcloud_makers_project

Provides a resource to create an EdgeOne Makers project whose source is uploaded directly.

## Example Usage

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

## Argument Reference

The following arguments are supported:

* `name` - (Required, String) Name of the project. It must match `^[a-z0-9][a-z0-9-]{3,61}[a-z0-9]$` and be unique within the zone. It is also used as the prefix of the preset domain.
* `zone_id` - (Required, String) Id of the Makers zone. It can be found with `tencentcloud_teo_zones` filtered by `zone-type` `pages`.
* `area` - (Optional, String) Acceleration area. Valid values: `mainland`, `overseas`, `global`. When it is not set, the zone default is used. Custom domains of `mainland` and `global` projects must have ICP filing. Changing it creates a new project.
* `build_command` - (Optional, String) Build command.
* `framework` - (Optional, String) Framework identifier, for example `next` or `vite`.
* `install_command` - (Optional, String) Dependency install command.
* `nodejs_version` - (Optional, String) Node.js version used to build the project. Valid values: `14.21.3`, `16.20.2`, `18.20.4`, `20.18.0`, `22.11.0`, `22.17.1`, `22.21.1`, `24.5.0`, `24.11.0`, `24.18.0`.
* `output_directory` - (Optional, String) Directory of the build output.
* `root_directory` - (Optional, String) Root directory of the project inside the uploaded source.

## Attributes Reference

In addition to all arguments above, the following attributes are exported:

* `created_on` - Creation time in ISO 8601 format.
* `id` - Id of the resource, in the format `zone_id#project_id`.
* `preset_domain` - Preset domain assigned by the platform. For `mainland` and `global` projects it only serves signed requests.
* `project_id` - Id of the project.
* `status` - Status of the project, for example `Normal` or `Ban`.

## API Reference

This resource interacts with the following TencentCloud APIs:

* Create: [Send](https://www.tencentcloud.com/search/?wd=Send&type=document&lang=en)
* Read: [Send](https://www.tencentcloud.com/search/?wd=Send&type=document&lang=en)
* Update:
  - [ModifyHostsAttribute](https://www.tencentcloud.com/search/?wd=ModifyHostsAttribute&type=document&lang=en)
  - [Send](https://www.tencentcloud.com/search/?wd=Send&type=document&lang=en)
* Delete: [Send](https://www.tencentcloud.com/search/?wd=Send&type=document&lang=en)

## Import

makers project can be imported using the id, e.g.

```
terraform import tencentcloud_makers_project.example zone_id#project_id
```

