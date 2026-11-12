---
subcategory: "TencentCloud EdgeOne(TEO)"
layout: "tencentcloud"
page_title: "TencentCloud: tencentcloud_teo_makers_zones"
sidebar_current: "docs-tencentcloud-data_source-teo_makers_zones"
description: |-
  Use this data source to query the EdgeOne Makers zones of the account. Every Makers resource needs the `zone_id` of one of them.
---

# tencentcloud_teo_makers_zones

Use this data source to query the EdgeOne Makers zones of the account. Every Makers resource needs the `zone_id` of one of them.

## Example Usage

```hcl
data "tencentcloud_teo_makers_zones" "this" {}

output "zone_id" {
  value = data.tencentcloud_teo_makers_zones.this.zone_ids[0]
}
```

## Argument Reference

The following arguments are supported:

* `zone_name` - (Optional, String) Exact name of the zone to look up.

## Attributes Reference

In addition to all arguments above, the following attributes are exported:

  * `area` - Acceleration area of the zone.
  * `created_on` - Creation time in ISO 8601 format.
  * `status` - Status of the zone.
  * `zone_id` - Id of the zone.
  * `zone_name` - Name of the zone.
* `id` - Id of the data source.
* `zone_ids` - Ids of the matching zones.
* `zones` - Matching zones.


