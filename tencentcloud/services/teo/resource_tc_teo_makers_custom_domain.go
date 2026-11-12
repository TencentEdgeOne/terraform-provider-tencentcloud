package teo

import (
	"context"
	"fmt"
	"strings"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringdefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"

	tccommon "github.com/tencentcloudstack/terraform-provider-tencentcloud/tencentcloud/common"
	"github.com/tencentcloudstack/terraform-provider-tencentcloud/tencentcloud/framework/fw"
	fwhelper "github.com/tencentcloudstack/terraform-provider-tencentcloud/tencentcloud/framework/helper"
)

var (
	_ resource.ResourceWithConfigure   = &TeoMakersCustomDomainResource{}
	_ resource.ResourceWithImportState = &TeoMakersCustomDomainResource{}
)

// NewTeoMakersCustomDomainResource is the factory referenced by
// tencentcloud/framework/registry.go to register this resource.
func NewTeoMakersCustomDomainResource() resource.Resource {
	return &TeoMakersCustomDomainResource{}
}

// TeoMakersCustomDomainResource implements tencentcloud_teo_makers_custom_domain.
type TeoMakersCustomDomainResource struct {
	fw.ResourceWithConfigure
}

type TeoMakersCustomDomainResourceModel struct {
	Id                    types.String `tfsdk:"id"`
	ZoneId                types.String `tfsdk:"zone_id"`
	ProjectId             types.String `tfsdk:"project_id"`
	Domain                types.String `tfsdk:"domain"`
	Environment           types.String `tfsdk:"environment"`
	Status                types.String `tfsdk:"status"`
	Cname                 types.String `tfsdk:"cname"`
	CurrentCname          types.String `tfsdk:"current_cname"`
	OwnershipVerification types.Object `tfsdk:"ownership_verification"`
}

var (
	makersDnsVerificationAttrTypes = map[string]attr.Type{
		"record_type":  types.StringType,
		"subdomain":    types.StringType,
		"record_value": types.StringType,
	}
	makersFileVerificationAttrTypes = map[string]attr.Type{
		"path":    types.StringType,
		"content": types.StringType,
	}
	makersOwnershipAttrTypes = map[string]attr.Type{
		"dns":  types.ObjectType{AttrTypes: makersDnsVerificationAttrTypes},
		"file": types.ObjectType{AttrTypes: makersFileVerificationAttrTypes},
	}
)

func (r *TeoMakersCustomDomainResource) Metadata(_ context.Context, _ resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = "tencentcloud_teo_makers_custom_domain"
}

func (r *TeoMakersCustomDomainResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	replace := []planmodifier.String{stringplanmodifier.RequiresReplace()}
	resp.Schema = schema.Schema{
		Description: "Provides a resource to bind a custom domain to an EdgeOne Makers project.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed:    true,
				Description: "Id of the resource, in the format `zone_id#project_id#domain`.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"zone_id": schema.StringAttribute{
				Required:      true,
				Description:   "Id of the Makers zone.",
				PlanModifiers: replace,
			},
			"project_id": schema.StringAttribute{
				Required:      true,
				Description:   "Id of the project.",
				PlanModifiers: replace,
			},
			"domain": schema.StringAttribute{
				Required:      true,
				Description:   "Custom domain to bind. Domains of `mainland` and `global` projects must have ICP filing.",
				PlanModifiers: replace,
			},
			"environment": schema.StringAttribute{
				Optional: true,
				Computed: true,
				Default:  stringdefault.StaticString("Production"),
				Description: "Slug of the environment the domain serves. Default is `Production`. The environment must " +
					"already have a successful deployment, both when binding and when changing it in place.",
			},
			"status": schema.StringAttribute{
				Computed:    true,
				Description: "Status of the domain, for example `process` right after binding, `online` once it serves traffic, or `offline`.",
			},
			"cname": schema.StringAttribute{
				Computed: true,
				Description: "CNAME target assigned by EdgeOne. It stays empty until ownership verification passes and " +
					"the domain is created. Do not change the HTTPS certificate of the domain before it is set.",
			},
			"current_cname": schema.StringAttribute{
				Computed:    true,
				Description: "Current public CNAME resolution of the domain. It is not a DNS target.",
			},
			"ownership_verification": schema.SingleNestedAttribute{
				Computed:    true,
				Description: "Ownership verification required before the domain takes effect. Null when no verification is needed.",
				Attributes: map[string]schema.Attribute{
					"dns": schema.SingleNestedAttribute{
						Computed:    true,
						Description: "DNS record verification.",
						Attributes: map[string]schema.Attribute{
							"record_type":  schema.StringAttribute{Computed: true, Description: "Record type, `TXT`."},
							"subdomain":    schema.StringAttribute{Computed: true, Description: "Host record of the TXT record."},
							"record_value": schema.StringAttribute{Computed: true, Description: "Value of the TXT record."},
						},
					},
					"file": schema.SingleNestedAttribute{
						Computed:    true,
						Description: "File verification.",
						Attributes: map[string]schema.Attribute{
							"path":    schema.StringAttribute{Computed: true, Description: "URL path of the verification file."},
							"content": schema.StringAttribute{Computed: true, Description: "Content of the verification file."},
						},
					},
				},
			},
		},
	}
}

func (r *TeoMakersCustomDomainResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	defer tccommon.LogElapsed("resource.tencentcloud_teo_makers_custom_domain.create")()

	var plan TeoMakersCustomDomainResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	zoneId, projectId, domain := plan.ZoneId.ValueString(), plan.ProjectId.ValueString(), plan.Domain.ValueString()
	service := NewTeoMakersService(r.Client())
	if err := service.CreateCustomDomain(ctx, zoneId, projectId, domain, plan.Environment.ValueString()); err != nil {
		resp.Diagnostics.Append(fwhelper.WrapSDKError("Error binding Makers custom domain", err))
		return
	}
	plan.Id = types.StringValue(strings.Join([]string{zoneId, projectId, domain}, tccommon.FILED_SP))
	resp.Diagnostics.Append(r.refresh(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *TeoMakersCustomDomainResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	defer tccommon.LogElapsed("resource.tencentcloud_teo_makers_custom_domain.read")()

	var state TeoMakersCustomDomainResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	d, err := NewTeoMakersService(r.Client()).DescribeCustomDomain(ctx,
		state.ZoneId.ValueString(), state.ProjectId.ValueString(), state.Domain.ValueString())
	if err != nil {
		resp.Diagnostics.Append(fwhelper.WrapSDKError("Error reading Makers custom domain", err))
		return
	}
	if d == nil {
		resp.Diagnostics.AddWarning("Makers custom domain not found",
			fmt.Sprintf("domain %s is not bound to project %s any more, removing it from state.",
				state.Domain.ValueString(), state.ProjectId.ValueString()))
		resp.State.RemoveResource(ctx)
		return
	}
	resp.Diagnostics.Append(makersFlattenDomain(&state, d)...)
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *TeoMakersCustomDomainResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	defer tccommon.LogElapsed("resource.tencentcloud_teo_makers_custom_domain.update")()

	var plan, state TeoMakersCustomDomainResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if !strings.EqualFold(plan.Environment.ValueString(), state.Environment.ValueString()) {
		err := NewTeoMakersService(r.Client()).ModifyCustomDomainEnv(ctx,
			state.ZoneId.ValueString(), state.ProjectId.ValueString(), state.Domain.ValueString(), plan.Environment.ValueString())
		if err != nil {
			resp.Diagnostics.Append(fwhelper.WrapSDKError("Error changing environment of Makers custom domain", err))
			return
		}
	}
	plan.Id = state.Id
	resp.Diagnostics.Append(r.refresh(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *TeoMakersCustomDomainResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	defer tccommon.LogElapsed("resource.tencentcloud_teo_makers_custom_domain.delete")()

	var state TeoMakersCustomDomainResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	zoneId, projectId, domain := state.ZoneId.ValueString(), state.ProjectId.ValueString(), state.Domain.ValueString()
	service := NewTeoMakersService(r.Client())
	if err := service.DeleteCustomDomain(ctx, zoneId, projectId, domain); err != nil {
		// The unbind error for a missing domain is a generic FailedOperation, so
		// check whether the domain is really gone instead of matching messages.
		d, describeErr := service.DescribeCustomDomain(ctx, zoneId, projectId, domain)
		if describeErr != nil || d != nil {
			resp.Diagnostics.Append(fwhelper.WrapSDKError("Error unbinding Makers custom domain", err))
		}
	}
}

func (r *TeoMakersCustomDomainResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	parts := strings.Split(req.ID, tccommon.FILED_SP)
	if len(parts) != 3 || parts[0] == "" || parts[1] == "" || parts[2] == "" {
		resp.Diagnostics.AddError("Invalid import id", "The import id must be in the format `zone_id#project_id#domain`.")
		return
	}
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("id"), req.ID)...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("zone_id"), parts[0])...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("project_id"), parts[1])...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("domain"), parts[2])...)
}

func (r *TeoMakersCustomDomainResource) refresh(ctx context.Context, model *TeoMakersCustomDomainResourceModel) (diags diag.Diagnostics) {
	d, err := NewTeoMakersService(r.Client()).DescribeCustomDomain(ctx,
		model.ZoneId.ValueString(), model.ProjectId.ValueString(), model.Domain.ValueString())
	if err != nil {
		diags.Append(fwhelper.WrapSDKError("Error reading Makers custom domain", err))
		return
	}
	if d == nil {
		diags.AddError("Makers custom domain not readable",
			fmt.Sprintf("domain %s was bound but cannot be read back", model.Domain.ValueString()))
		return
	}
	diags.Append(makersFlattenDomain(model, d)...)
	return
}

func makersFlattenDomain(model *TeoMakersCustomDomainResourceModel, d *MakersDomain) (diags diag.Diagnostics) {
	if !strings.EqualFold(model.Environment.ValueString(), d.Env) {
		model.Environment = types.StringValue(d.Env)
	}
	model.Status = types.StringValue(d.Status)
	model.Cname = types.StringValue("")
	if d.Cname != nil {
		model.Cname = types.StringValue(*d.Cname)
	}
	model.CurrentCname = types.StringValue(d.CurrentCname)

	dns := types.ObjectNull(makersDnsVerificationAttrTypes)
	file := types.ObjectNull(makersFileVerificationAttrTypes)
	model.OwnershipVerification = types.ObjectNull(makersOwnershipAttrTypes)
	ov := d.OwnershipVerification
	if ov == nil {
		return
	}
	var dd diag.Diagnostics
	if ov.DnsVerification != nil {
		dns, dd = types.ObjectValue(makersDnsVerificationAttrTypes, map[string]attr.Value{
			"record_type":  types.StringValue(ov.DnsVerification.RecordType),
			"subdomain":    types.StringValue(ov.DnsVerification.Subdomain),
			"record_value": types.StringValue(ov.DnsVerification.RecordValue),
		})
		diags.Append(dd...)
	}
	if ov.FileVerification != nil {
		file, dd = types.ObjectValue(makersFileVerificationAttrTypes, map[string]attr.Value{
			"path":    types.StringValue(ov.FileVerification.Path),
			"content": types.StringValue(ov.FileVerification.Content),
		})
		diags.Append(dd...)
	}
	model.OwnershipVerification, dd = types.ObjectValue(makersOwnershipAttrTypes, map[string]attr.Value{
		"dns":  dns,
		"file": file,
	})
	diags.Append(dd...)
	return
}
