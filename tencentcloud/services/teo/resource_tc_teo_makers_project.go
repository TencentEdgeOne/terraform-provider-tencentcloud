package teo

import (
	"context"
	"fmt"
	"regexp"
	"strings"

	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"

	tccommon "github.com/tencentcloudstack/terraform-provider-tencentcloud/tencentcloud/common"
	"github.com/tencentcloudstack/terraform-provider-tencentcloud/tencentcloud/framework/fw"
	fwhelper "github.com/tencentcloudstack/terraform-provider-tencentcloud/tencentcloud/framework/helper"
)

var (
	_ resource.ResourceWithConfigure   = &TeoMakersProjectResource{}
	_ resource.ResourceWithImportState = &TeoMakersProjectResource{}
)

var makersProjectNameRegexp = regexp.MustCompile(`^[a-z0-9][a-z0-9-]{3,61}[a-z0-9]$`)

var makersNodejsVersions = []string{"14.21.3", "16.20.2", "18.20.4", "20.18.0", "22.11.0", "22.17.1", "24.5.0"}

// NewTeoMakersProjectResource is the factory referenced by
// tencentcloud/framework/registry.go to register this resource.
func NewTeoMakersProjectResource() resource.Resource {
	return &TeoMakersProjectResource{}
}

// TeoMakersProjectResource implements tencentcloud_teo_makers_project.
type TeoMakersProjectResource struct {
	fw.ResourceWithConfigure
}

type TeoMakersProjectResourceModel struct {
	Id              types.String `tfsdk:"id"`
	ZoneId          types.String `tfsdk:"zone_id"`
	ProjectId       types.String `tfsdk:"project_id"`
	Name            types.String `tfsdk:"name"`
	Area            types.String `tfsdk:"area"`
	NodejsVersion   types.String `tfsdk:"nodejs_version"`
	RootDirectory   types.String `tfsdk:"root_directory"`
	OutputDirectory types.String `tfsdk:"output_directory"`
	BuildCommand    types.String `tfsdk:"build_command"`
	InstallCommand  types.String `tfsdk:"install_command"`
	Framework       types.String `tfsdk:"framework"`
	PresetDomain    types.String `tfsdk:"preset_domain"`
	Status          types.String `tfsdk:"status"`
	CreatedOn       types.String `tfsdk:"created_on"`
}

func (r *TeoMakersProjectResource) Metadata(_ context.Context, _ resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = "tencentcloud_teo_makers_project"
}

func makersOptionalComputedString(description string, validators ...validator.String) schema.StringAttribute {
	return schema.StringAttribute{
		Optional:    true,
		Computed:    true,
		Description: description,
		Validators:  validators,
		PlanModifiers: []planmodifier.String{
			stringplanmodifier.UseStateForUnknown(),
		},
	}
}

func (r *TeoMakersProjectResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Provides a resource to create an EdgeOne Makers project whose source is uploaded directly.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed:    true,
				Description: "Id of the resource, in the format `zone_id#project_id`.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"zone_id": schema.StringAttribute{
				Required:    true,
				Description: "Id of the Makers zone. It can be found with `tencentcloud_teo_makers_zones`.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"project_id": schema.StringAttribute{
				Computed:    true,
				Description: "Id of the project.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"name": schema.StringAttribute{
				Required: true,
				Description: "Name of the project. It must match `^[a-z0-9][a-z0-9-]{3,61}[a-z0-9]$` and be unique within " +
					"the zone. It is also used as the prefix of the preset domain.",
				Validators: []validator.String{
					makersMatches(makersProjectNameRegexp),
				},
			},
			"area": schema.StringAttribute{
				Optional: true,
				Computed: true,
				Description: "Acceleration area. Valid values: `mainland`, `overseas`, `global`. When it is not set, the " +
					"zone default is used. Custom domains of `mainland` and `global` projects must have ICP filing. " +
					"Changing it creates a new project.",
				Validators: []validator.String{
					makersOneOf("mainland", "overseas", "global"),
				},
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
					stringplanmodifier.RequiresReplace(),
				},
			},
			"nodejs_version": makersOptionalComputedString(
				"Node.js version used to build the project. Valid values: `"+strings.Join(makersNodejsVersions, "`, `")+"`.",
				makersOneOf(makersNodejsVersions...),
			),
			"root_directory":   makersOptionalComputedString("Root directory of the project inside the uploaded source."),
			"output_directory": makersOptionalComputedString("Directory of the build output."),
			"build_command":    makersOptionalComputedString("Build command."),
			"install_command":  makersOptionalComputedString("Dependency install command."),
			"framework":        makersOptionalComputedString("Framework identifier, for example `next` or `vite`."),
			"preset_domain": schema.StringAttribute{
				Computed:    true,
				Description: "Preset domain assigned by the platform. For `mainland` and `global` projects it only serves signed requests.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"status": schema.StringAttribute{
				Computed:    true,
				Description: "Status of the project, for example `Normal` or `Ban`.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"created_on": schema.StringAttribute{
				Computed:    true,
				Description: "Creation time in ISO 8601 format.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
		},
	}
}

func makersStringPointer(v types.String) *string {
	if v.IsNull() || v.IsUnknown() {
		return nil
	}
	return v.ValueStringPointer()
}

func makersNullableString(v *string) types.String {
	if v == nil {
		return types.StringValue("")
	}
	return types.StringValue(*v)
}

func (r *TeoMakersProjectResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	defer tccommon.LogElapsed("resource.tencentcloud_teo_makers_project.create")()

	var plan TeoMakersProjectResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	zoneId := plan.ZoneId.ValueString()
	service := NewTeoMakersService(r.Client())
	// CreatePagesProject is not idempotent, so it is never retried.
	projectId, err := service.CreateProject(ctx, zoneId, makersCreateProjectRequest{
		Name:          plan.Name.ValueString(),
		Channel:       "Custom",
		Provider:      "Upload",
		Source:        makersProjectSource,
		Area:          makersStringPointer(plan.Area),
		NodejsVersion: makersStringPointer(plan.NodejsVersion),
		RootDir:       makersStringPointer(plan.RootDirectory),
		OutputDir:     makersStringPointer(plan.OutputDirectory),
		BuildCmd:      makersStringPointer(plan.BuildCommand),
		InstallCmd:    makersStringPointer(plan.InstallCommand),
		Framework:     makersStringPointer(plan.Framework),
	})
	if err != nil {
		resp.Diagnostics.Append(fwhelper.WrapSDKError("Error creating Makers project", err))
		return
	}

	plan.Id = types.StringValue(zoneId + tccommon.FILED_SP + projectId)
	plan.ProjectId = types.StringValue(projectId)
	resp.Diagnostics.Append(r.refresh(ctx, &plan, true)...)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *TeoMakersProjectResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	defer tccommon.LogElapsed("resource.tencentcloud_teo_makers_project.read")()

	var state TeoMakersProjectResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	project, err := NewTeoMakersService(r.Client()).DescribeProjectById(ctx, state.ZoneId.ValueString(), state.ProjectId.ValueString())
	if err != nil {
		resp.Diagnostics.Append(fwhelper.WrapSDKError("Error reading Makers project", err))
		return
	}
	if project == nil {
		resp.Diagnostics.AddWarning("Makers project not found",
			fmt.Sprintf("project %s does not exist any more, removing it from state.", state.ProjectId.ValueString()))
		resp.State.RemoveResource(ctx)
		return
	}
	makersFlattenProject(&state, project)
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *TeoMakersProjectResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	defer tccommon.LogElapsed("resource.tencentcloud_teo_makers_project.update")()

	var plan, state TeoMakersProjectResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	changed := func(p, s types.String) *string {
		if p.IsNull() || p.IsUnknown() || p.Equal(s) {
			return nil
		}
		return p.ValueStringPointer()
	}
	request := makersModifyProjectRequest{
		ProjectId:     state.ProjectId.ValueString(),
		Name:          changed(plan.Name, state.Name),
		NodejsVersion: changed(plan.NodejsVersion, state.NodejsVersion),
		RootDir:       changed(plan.RootDirectory, state.RootDirectory),
		OutputDir:     changed(plan.OutputDirectory, state.OutputDirectory),
		BuildCmd:      changed(plan.BuildCommand, state.BuildCommand),
		InstallCmd:    changed(plan.InstallCommand, state.InstallCommand),
		Framework:     changed(plan.Framework, state.Framework),
	}
	if request != (makersModifyProjectRequest{ProjectId: request.ProjectId}) {
		if err := NewTeoMakersService(r.Client()).ModifyProject(ctx, state.ZoneId.ValueString(), request); err != nil {
			resp.Diagnostics.Append(fwhelper.WrapSDKError("Error updating Makers project", err))
			return
		}
	}

	plan.Id = state.Id
	plan.ProjectId = state.ProjectId
	resp.Diagnostics.Append(r.refresh(ctx, &plan, false)...)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *TeoMakersProjectResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	defer tccommon.LogElapsed("resource.tencentcloud_teo_makers_project.delete")()

	var state TeoMakersProjectResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	err := NewTeoMakersService(r.Client()).DeleteProject(ctx, state.ZoneId.ValueString(), state.ProjectId.ValueString())
	if err != nil && !makersIsNotFound(err) {
		resp.Diagnostics.Append(fwhelper.WrapSDKError("Error deleting Makers project", err))
	}
}

func (r *TeoMakersProjectResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	parts := strings.Split(req.ID, tccommon.FILED_SP)
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		resp.Diagnostics.AddError("Invalid import id", "The import id must be in the format `zone_id#project_id`.")
		return
	}
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("id"), req.ID)...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("zone_id"), parts[0])...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("project_id"), parts[1])...)
}

func (r *TeoMakersProjectResource) refresh(ctx context.Context, model *TeoMakersProjectResourceModel, created bool) (diags diag.Diagnostics) {
	project, err := NewTeoMakersService(r.Client()).DescribeProjectById(ctx, model.ZoneId.ValueString(), model.ProjectId.ValueString())
	if err != nil {
		diags.Append(fwhelper.WrapSDKError("Error reading Makers project", err))
		return
	}
	if project == nil {
		if created {
			diags.AddError("Makers project not readable after create",
				fmt.Sprintf("project %s was created but cannot be read back", model.ProjectId.ValueString()))
		} else {
			diags.AddError("Makers project not readable after update",
				fmt.Sprintf("project %s cannot be read back", model.ProjectId.ValueString()))
		}
		return
	}
	makersFlattenProject(model, project)
	return
}

func makersFlattenProject(model *TeoMakersProjectResourceModel, project *MakersProject) {
	model.Name = types.StringValue(project.Name)
	model.Area = types.StringValue(project.Area)
	model.NodejsVersion = types.StringValue(project.NodejsVersion)
	model.RootDirectory = makersNullableString(project.RootDir)
	model.OutputDirectory = makersNullableString(project.OutputDir)
	model.BuildCommand = makersNullableString(project.BuildCmd)
	model.InstallCommand = makersNullableString(project.InstallCmd)
	model.Framework = makersNullableString(project.Framework)
	model.PresetDomain = types.StringValue(project.PresetDomain)
	model.Status = types.StringValue(project.Status)
	model.CreatedOn = types.StringValue(project.CreatedOn)
}
