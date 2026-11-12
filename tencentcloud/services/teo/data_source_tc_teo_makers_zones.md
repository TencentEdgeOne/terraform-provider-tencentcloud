Use this data source to query the EdgeOne Makers zones of the account. Every Makers resource needs the `zone_id` of one of them.

Example Usage

```hcl
data "tencentcloud_teo_makers_zones" "this" {}

output "zone_id" {
  value = data.tencentcloud_teo_makers_zones.this.zone_ids[0]
}
```
