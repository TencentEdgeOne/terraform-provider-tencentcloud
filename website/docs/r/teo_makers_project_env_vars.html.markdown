---
subcategory: "TencentCloud EdgeOne(TEO)"
layout: "tencentcloud"
page_title: "TencentCloud: tencentcloud_teo_makers_project_env_vars"
sidebar_current: "docs-tencentcloud-resource-teo_makers_project_env_vars"
description: |-
  Provides a resource to manage the complete set of environment variables of an EdgeOne Makers project.
---

# tencentcloud_teo_makers_project_env_vars

Provides a resource to manage the complete set of environment variables of an EdgeOne Makers project.

## Example Usage

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

## Argument Reference

The following arguments are supported:

* `env_vars` - (Required, Set) Environment variables. The same key may appear more than once when the `environments` of those entries do not overlap.
* `project_id` - (Required, String) Id of the project.
* `zone_id` - (Required, String) Id of the Makers zone.

The `env_vars` object supports the following:

* `key` - (Required, String) Name of the variable, at most 255 characters.
* `value` - (Required, String) Value of the variable, at most 1000 characters.
* `comment` - (Optional, String) Comment of the variable, at most 255 characters.
* `environments` - (Optional, Set) Slugs of the environments the variable applies to, for example `Production`, `Preview` or a custom environment slug. When it is not set, the variable applies to every environment that exists on the project when it is written.

## Attributes Reference

In addition to all arguments above, the following attributes are exported:

* `id` - Id of the resource, in the format `zone_id#project_id`.

## API Reference

This resource interacts with the following TencentCloud APIs:

* Create: [Send](https://www.tencentcloud.com/search/?wd=Send&type=document&lang=en)
* Read: [Send](https://www.tencentcloud.com/search/?wd=Send&type=document&lang=en)
* Update: [Send](https://www.tencentcloud.com/search/?wd=Send&type=document&lang=en)
* Delete: [Send](https://www.tencentcloud.com/search/?wd=Send&type=document&lang=en)

## Import

teo makers project env vars can be imported using the id, e.g.

```
terraform import tencentcloud_teo_makers_project_env_vars.example zone_id#project_id
```

