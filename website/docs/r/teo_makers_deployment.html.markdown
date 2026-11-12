---
subcategory: "TencentCloud EdgeOne(TEO)"
layout: "tencentcloud"
page_title: "TencentCloud: tencentcloud_teo_makers_deployment"
sidebar_current: "docs-tencentcloud-resource-teo_makers_deployment"
description: |-
  Provides a resource to deploy a local directory or zip archive to an EdgeOne Makers project.
---

# tencentcloud_teo_makers_deployment

Provides a resource to deploy a local directory or zip archive to an EdgeOne Makers project.

~> **NOTE:** `source_zip` is recommended. A zip archive is uploaded as a single object, while `source_dir` uploads files one by one with temporary credentials that expire after 30 minutes, so very large directories may fail. A zip archive is also the exact artifact built by `archive_file` or CI.

## Example Usage

### Deploy a zip archive (recommended)

```hcl
data "archive_file" "site" {
  type        = "zip"
  source_dir  = "${path.module}/site"
  output_path = "${path.module}/build/site.zip"
}

resource "tencentcloud_teo_makers_deployment" "example" {
  zone_id    = tencentcloud_teo_makers_project.example.zone_id
  project_id = tencentcloud_teo_makers_project.example.project_id
  source_zip = data.archive_file.site.output_path

  timeouts = {
    create = "30m"
  }

  lifecycle {
    replace_triggered_by = [tencentcloud_teo_makers_project_env_vars.example]
  }
}
```

### Deploy a directory

```hcl
resource "tencentcloud_teo_makers_deployment" "example" {
  zone_id     = tencentcloud_teo_makers_project.example.zone_id
  project_id  = tencentcloud_teo_makers_project.example.project_id
  environment = "Production"
  source_dir  = "${path.module}/site"
}
```

## Argument Reference

The following arguments are supported:

* `project_id` - (Required, String) Id of the project.
* `zone_id` - (Required, String) Id of the Makers zone.
* `environment` - (Optional, String) Slug of the target environment: `Production`, `Preview`, or the slug of a custom environment of the project. Default is `Production`.
* `source_dir` - (Optional, String) Local directory to upload. `node_modules`, `.edgeone`, `.env`, `.git` and `.DS_Store` are skipped. Relative paths are resolved against the working directory, so `path.module` is recommended. Exactly one of `source_dir` and `source_zip` must be set.
* `source_hash` - (Optional, String) Hash of the uploaded source. When it is not set, the provider computes it during plan from the files of `source_dir`, or from the content of `source_zip`. A different value creates a new deployment.
* `source_zip` - (Optional, String) Local zip archive to upload, for example the `output_path` of an `archive_file` data source or an artifact built by CI. The archive is uploaded as is. Recommended over `source_dir`. Exactly one of `source_dir` and `source_zip` must be set.
* `timeouts` - (Optional, Object) Framework attribute.
* `triggers` - (Optional, Map) Arbitrary values that create a new deployment when they change, for example the id of the environment variables resource.

The `timeouts` object supports the following:

* `create` - (Optional, String) A string that can be [parsed as a duration](https://pkg.go.dev/time#ParseDuration) consisting of numbers and unit suffixes, such as "30s" or "2h45m". Valid time units are "s" (seconds), "m" (minutes), "h" (hours).

## Attributes Reference

In addition to all arguments above, the following attributes are exported:

* `created_on` - Creation time in ISO 8601 format.
* `deployment_id` - Id of the deployment.
* `id` - Id of the resource, in the format `zone_id#project_id#deployment_id`.
* `preview_url` - Access URL of this deployment.
* `project_url` - Default URL of the project.
* `status` - Status of the deployment: `Success`, `Failed`, `Timeout`, `Cancelled` or `Invalid`.

## API Reference

This resource interacts with the following TencentCloud APIs:

* Create: [Send](https://www.tencentcloud.com/search/?wd=Send&type=document&lang=en)
* Read: [Send](https://www.tencentcloud.com/search/?wd=Send&type=document&lang=en)

## Import

teo makers deployment can be imported using the id, e.g.

```
terraform import tencentcloud_teo_makers_deployment.example zone_id#project_id#deployment_id
```

