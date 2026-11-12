---
subcategory: "TencentCloud EdgeOne(TEO)"
layout: "tencentcloud"
page_title: "TencentCloud: tencentcloud_teo_makers_custom_domain"
sidebar_current: "docs-tencentcloud-resource-teo_makers_custom_domain"
description: |-
  Provides a resource to bind a custom domain to an EdgeOne Makers project.
---

# tencentcloud_teo_makers_custom_domain

Provides a resource to bind a custom domain to an EdgeOne Makers project.

## Example Usage

```hcl
resource "tencentcloud_teo_makers_custom_domain" "example" {
  zone_id    = tencentcloud_teo_makers_project.example.zone_id
  project_id = tencentcloud_teo_makers_project.example.project_id
  domain     = "www.example.com"

  depends_on = [tencentcloud_teo_makers_deployment.example]
}
```

## Argument Reference

The following arguments are supported:

* `domain` - (Required, String) Custom domain to bind. Domains of `mainland` and `global` projects must have ICP filing.
* `project_id` - (Required, String) Id of the project.
* `zone_id` - (Required, String) Id of the Makers zone.
* `environment` - (Optional, String) Slug of the environment the domain serves. Default is `Production`. The environment must already have a successful deployment, both when binding and when changing it in place.

## Attributes Reference

In addition to all arguments above, the following attributes are exported:

    * `content` - Content of the verification file.
    * `path` - URL path of the verification file.
    * `record_type` - Record type, `TXT`.
    * `record_value` - Value of the TXT record.
    * `subdomain` - Host record of the TXT record.
  * `dns` - DNS record verification.
  * `file` - File verification.
* `cname` - CNAME target assigned by EdgeOne. It stays empty until ownership verification passes and the domain is created. Do not change the HTTPS certificate of the domain before it is set.
* `current_cname` - Current public CNAME resolution of the domain. It is not a DNS target.
* `id` - Id of the resource, in the format `zone_id#project_id#domain`.
* `ownership_verification` - Ownership verification required before the domain takes effect. Null when no verification is needed.
* `status` - Status of the domain, for example `process` right after binding, `online` once it serves traffic, or `offline`.

## API Reference

This resource interacts with the following TencentCloud APIs:

* Create: [Send](https://www.tencentcloud.com/search/?wd=Send&type=document&lang=en)
* Read: [Send](https://www.tencentcloud.com/search/?wd=Send&type=document&lang=en)
* Update: [Send](https://www.tencentcloud.com/search/?wd=Send&type=document&lang=en)
* Delete: [Send](https://www.tencentcloud.com/search/?wd=Send&type=document&lang=en)

## Import

teo makers custom domain can be imported using the id, e.g.

```
terraform import tencentcloud_teo_makers_custom_domain.example zone_id#project_id#www.example.com
```

