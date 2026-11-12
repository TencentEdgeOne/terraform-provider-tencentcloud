Provides a resource to deploy a local directory or zip archive to an EdgeOne Makers project.

~> **NOTE:** `source_zip` is recommended. A zip archive is uploaded as a single object, while `source_dir` uploads files one by one with temporary credentials that expire after 30 minutes, so very large directories may fail. A zip archive is also the exact artifact built by `archive_file` or CI.

Example Usage

Deploy a zip archive (recommended)

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

Deploy a directory

```hcl
resource "tencentcloud_teo_makers_deployment" "example" {
  zone_id     = tencentcloud_teo_makers_project.example.zone_id
  project_id  = tencentcloud_teo_makers_project.example.project_id
  environment = "Production"
  source_dir  = "${path.module}/site"
}
```

Import

teo makers deployment can be imported using the id, e.g.

```
terraform import tencentcloud_teo_makers_deployment.example zone_id#project_id#deployment_id
```
