package teo

import (
	"context"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
	teov20220901 "github.com/tencentcloud/tencentcloud-sdk-go/tencentcloud/teo/v20220901"

	tccommon "github.com/tencentcloudstack/terraform-provider-tencentcloud/tencentcloud/common"
	"github.com/tencentcloudstack/terraform-provider-tencentcloud/tencentcloud/framework/fw"
	fwhelper "github.com/tencentcloudstack/terraform-provider-tencentcloud/tencentcloud/framework/helper"
	"github.com/tencentcloudstack/terraform-provider-tencentcloud/tencentcloud/internal/helper"
	"github.com/tencentcloudstack/terraform-provider-tencentcloud/tencentcloud/ratelimit"
)

var _ datasource.DataSourceWithConfigure = &TeoMakersZonesDataSource{}

// makersZoneType is the DescribeZones zone-type of Makers zones.
const makersZoneType = "pages"

// NewTeoMakersZonesDataSource is the factory referenced by
// tencentcloud/framework/registry.go to register this data source.
func NewTeoMakersZonesDataSource() datasource.DataSource {
	return &TeoMakersZonesDataSource{}
}

// TeoMakersZonesDataSource implements tencentcloud_teo_makers_zones.
type TeoMakersZonesDataSource struct {
	fw.DataSourceWithConfigure
}

type TeoMakersZonesDataSourceModel struct {
	Id       types.String         `tfsdk:"id"`
	ZoneName types.String         `tfsdk:"zone_name"`
	ZoneIds  []types.String       `tfsdk:"zone_ids"`
	Zones    []TeoMakersZoneModel `tfsdk:"zones"`
}

type TeoMakersZoneModel struct {
	ZoneId    types.String `tfsdk:"zone_id"`
	ZoneName  types.String `tfsdk:"zone_name"`
	Status    types.String `tfsdk:"status"`
	Area      types.String `tfsdk:"area"`
	CreatedOn types.String `tfsdk:"created_on"`
}

func (d *TeoMakersZonesDataSource) Metadata(_ context.Context, _ datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = "tencentcloud_teo_makers_zones"
}

func (d *TeoMakersZonesDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Use this data source to query the EdgeOne Makers zones of the account. Every Makers resource " +
			"needs the `zone_id` of one of them.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed:    true,
				Description: "Id of the data source.",
			},
			"zone_name": schema.StringAttribute{
				Optional:    true,
				Description: "Exact name of the zone to look up.",
			},
			"zone_ids": schema.ListAttribute{
				Computed:    true,
				ElementType: types.StringType,
				Description: "Ids of the matching zones.",
			},
			"zones": schema.ListNestedAttribute{
				Computed:    true,
				Description: "Matching zones.",
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						"zone_id":    schema.StringAttribute{Computed: true, Description: "Id of the zone."},
						"zone_name":  schema.StringAttribute{Computed: true, Description: "Name of the zone."},
						"status":     schema.StringAttribute{Computed: true, Description: "Status of the zone."},
						"area":       schema.StringAttribute{Computed: true, Description: "Acceleration area of the zone."},
						"created_on": schema.StringAttribute{Computed: true, Description: "Creation time in ISO 8601 format."},
					},
				},
			},
		},
	}
}

func (d *TeoMakersZonesDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	defer tccommon.LogElapsed("data_source.tencentcloud_teo_makers_zones.read")()

	var data TeoMakersZonesDataSourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	filters := []*teov20220901.AdvancedFilter{{
		Name:   helper.String("zone-type"),
		Values: []*string{helper.String(makersZoneType)},
	}}
	if !data.ZoneName.IsNull() && data.ZoneName.ValueString() != "" {
		filters = append(filters, &teov20220901.AdvancedFilter{
			Name:   helper.String("zone-name"),
			Values: []*string{data.ZoneName.ValueStringPointer()},
		})
	}

	var zones []*teov20220901.Zone
	var offset, limit int64 = 0, 100
	for {
		request := teov20220901.NewDescribeZonesRequest()
		request.Filters = filters
		request.Offset = &offset
		request.Limit = &limit
		ratelimit.Check(request.GetAction())
		response, err := d.Client().UseTeoClient().DescribeZonesWithContext(ctx, request)
		if err != nil {
			resp.Diagnostics.Append(fwhelper.WrapSDKError("Error reading Makers zones", err))
			return
		}
		if response.Response == nil {
			break
		}
		zones = append(zones, response.Response.Zones...)
		if len(response.Response.Zones) < int(limit) {
			break
		}
		offset += limit
	}

	data.ZoneIds = make([]types.String, 0, len(zones))
	data.Zones = make([]TeoMakersZoneModel, 0, len(zones))
	for _, z := range zones {
		data.ZoneIds = append(data.ZoneIds, types.StringPointerValue(z.ZoneId))
		data.Zones = append(data.Zones, TeoMakersZoneModel{
			ZoneId:    types.StringPointerValue(z.ZoneId),
			ZoneName:  types.StringPointerValue(z.ZoneName),
			Status:    types.StringPointerValue(z.Status),
			Area:      types.StringPointerValue(z.Area),
			CreatedOn: types.StringPointerValue(z.CreatedOn),
		})
	}
	data.Id = types.StringValue(makersZoneType + tccommon.FILED_SP + data.ZoneName.ValueString())
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}
