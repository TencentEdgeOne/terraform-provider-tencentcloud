package teo

import (
	"context"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/hashicorp/terraform-plugin-framework-timeouts/resource/timeouts"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/mapplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringdefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"

	tccommon "github.com/tencentcloudstack/terraform-provider-tencentcloud/tencentcloud/common"
	"github.com/tencentcloudstack/terraform-provider-tencentcloud/tencentcloud/framework/fw"
	fwhelper "github.com/tencentcloudstack/terraform-provider-tencentcloud/tencentcloud/framework/helper"
)

const (
	makersDeploymentPollInterval  = 5 * time.Second
	makersDeploymentCreateTimeout = 20 * time.Minute
	makersDeploymentLogLines      = 50
)

var (
	_ resource.ResourceWithConfigure      = &TeoMakersDeploymentResource{}
	_ resource.ResourceWithImportState    = &TeoMakersDeploymentResource{}
	_ resource.ResourceWithModifyPlan     = &TeoMakersDeploymentResource{}
	_ resource.ResourceWithValidateConfig = &TeoMakersDeploymentResource{}
)

// NewTeoMakersDeploymentResource is the factory referenced by
// tencentcloud/framework/registry.go to register this resource.
func NewTeoMakersDeploymentResource() resource.Resource {
	return &TeoMakersDeploymentResource{}
}

// TeoMakersDeploymentResource implements tencentcloud_teo_makers_deployment.
// Each instance is one immutable deployment built from an uploaded directory
// or zip archive.
type TeoMakersDeploymentResource struct {
	fw.ResourceWithConfigure
}

type TeoMakersDeploymentResourceModel struct {
	Id           types.String   `tfsdk:"id"`
	ZoneId       types.String   `tfsdk:"zone_id"`
	ProjectId    types.String   `tfsdk:"project_id"`
	Environment  types.String   `tfsdk:"environment"`
	SourceDir    types.String   `tfsdk:"source_dir"`
	SourceZip    types.String   `tfsdk:"source_zip"`
	SourceHash   types.String   `tfsdk:"source_hash"`
	Triggers     types.Map      `tfsdk:"triggers"`
	DeploymentId types.String   `tfsdk:"deployment_id"`
	Status       types.String   `tfsdk:"status"`
	PreviewUrl   types.String   `tfsdk:"preview_url"`
	ProjectUrl   types.String   `tfsdk:"project_url"`
	CreatedOn    types.String   `tfsdk:"created_on"`
	Timeouts     timeouts.Value `tfsdk:"timeouts"`
}

func (r *TeoMakersDeploymentResource) Metadata(_ context.Context, _ resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = "tencentcloud_teo_makers_deployment"
}

func (r *TeoMakersDeploymentResource) Schema(ctx context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	computedString := func(description string) schema.StringAttribute {
		return schema.StringAttribute{
			Computed:    true,
			Description: description,
			PlanModifiers: []planmodifier.String{
				stringplanmodifier.UseStateForUnknown(),
			},
		}
	}
	resp.Schema = schema.Schema{
		Description: "Provides a resource to deploy a local directory or zip archive to an EdgeOne Makers project. " +
			"The source is uploaded and built by the platform. Any change of the source content creates a new deployment.",
		Attributes: map[string]schema.Attribute{
			"id": computedString("Id of the resource, in the format `zone_id#project_id#deployment_id`."),
			"zone_id": schema.StringAttribute{
				Required:    true,
				Description: "Id of the Makers zone.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"project_id": schema.StringAttribute{
				Required:    true,
				Description: "Id of the project.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"environment": schema.StringAttribute{
				Optional: true,
				Computed: true,
				Default:  stringdefault.StaticString("Production"),
				Description: "Slug of the target environment: `Production`, `Preview`, or the slug of a custom " +
					"environment of the project. Default is `Production`.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"source_dir": schema.StringAttribute{
				Optional: true,
				Description: "Local directory to upload. `node_modules`, `.edgeone`, `.env`, `.git` and `.DS_Store` " +
					"are skipped. Relative paths are resolved against the working directory, so `path.module` is recommended. " +
					"Exactly one of `source_dir` and `source_zip` must be set.",
			},
			"source_zip": schema.StringAttribute{
				Optional: true,
				Description: "Local zip archive to upload, for example the `output_path` of an `archive_file` data source " +
					"or an artifact built by CI. The archive is uploaded as is. Recommended over `source_dir`. " +
					"Exactly one of `source_dir` and `source_zip` must be set.",
			},
			"source_hash": schema.StringAttribute{
				Optional: true,
				Computed: true,
				Description: "Hash of the uploaded source. When it is not set, the provider computes it during plan from " +
					"the files of `source_dir`, or from the content of `source_zip`. A different value creates a new deployment.",
			},
			"triggers": schema.MapAttribute{
				Optional:    true,
				ElementType: types.StringType,
				Description: "Arbitrary values that create a new deployment when they change, for example the id of the " +
					"environment variables resource.",
				PlanModifiers: []planmodifier.Map{
					mapplanmodifier.RequiresReplace(),
				},
			},
			"deployment_id": computedString("Id of the deployment."),
			"status": schema.StringAttribute{
				Computed:    true,
				Description: "Status of the deployment: `Success`, `Failed`, `Timeout`, `Cancelled` or `Invalid`.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"preview_url": computedString("Access URL of this deployment."),
			"project_url": computedString("Default URL of the project."),
			"created_on":  computedString("Creation time in ISO 8601 format."),
			"timeouts": timeouts.Attributes(ctx, timeouts.Opts{
				Create: true,
			}),
		},
	}
}

func (r *TeoMakersDeploymentResource) ValidateConfig(ctx context.Context, req resource.ValidateConfigRequest, resp *resource.ValidateConfigResponse) {
	var config TeoMakersDeploymentResourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &config)...)
	if resp.Diagnostics.HasError() || config.SourceDir.IsUnknown() || config.SourceZip.IsUnknown() {
		return
	}
	if config.SourceDir.IsNull() == config.SourceZip.IsNull() {
		resp.Diagnostics.AddAttributeError(path.Root("source_dir"), "Invalid source",
			"exactly one of `source_dir` and `source_zip` must be set")
	}
}

// ModifyPlan computes source_hash from source_dir or source_zip. An imported
// deployment has no hash in state; it adopts the computed hash instead of
// being replaced.
func (r *TeoMakersDeploymentResource) ModifyPlan(ctx context.Context, req resource.ModifyPlanRequest, resp *resource.ModifyPlanResponse) {
	if req.Plan.Raw.IsNull() {
		return
	}
	var config TeoMakersDeploymentResourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &config)...)
	if resp.Diagnostics.HasError() {
		return
	}

	hash := config.SourceHash
	if hash.IsNull() {
		if config.SourceDir.IsUnknown() || config.SourceZip.IsUnknown() {
			resp.Diagnostics.Append(resp.Plan.SetAttribute(ctx, path.Root("source_hash"), types.StringUnknown())...)
			return
		}
		var sum string
		var err error
		if !config.SourceZip.IsNull() {
			if sum, err = makersSourceZipHash(config.SourceZip.ValueString()); err != nil {
				resp.Diagnostics.AddAttributeError(path.Root("source_zip"), "Cannot read source archive", err.Error())
				return
			}
		} else if sum, err = makersSourceHash(config.SourceDir.ValueString()); err != nil {
			resp.Diagnostics.AddAttributeError(path.Root("source_dir"), "Cannot read source directory", err.Error())
			return
		}
		hash = types.StringValue(sum)
	}
	resp.Diagnostics.Append(resp.Plan.SetAttribute(ctx, path.Root("source_hash"), hash)...)

	if req.State.Raw.IsNull() {
		return
	}
	var state TeoMakersDeploymentResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if !state.SourceHash.IsNull() && !state.SourceHash.Equal(hash) {
		resp.RequiresReplace = append(resp.RequiresReplace, path.Root("source_hash"))
	}
}

func (r *TeoMakersDeploymentResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	defer tccommon.LogElapsed("resource.tencentcloud_teo_makers_deployment.create")()

	var plan TeoMakersDeploymentResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	timeout, diags := plan.Timeouts.Create(ctx, makersDeploymentCreateTimeout)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	zoneId, projectId := plan.ZoneId.ValueString(), plan.ProjectId.ValueString()
	service := NewTeoMakersService(r.Client())

	distType, targetPath := makersDistTypeFolder, ""
	if !plan.SourceZip.IsNull() {
		key, err := service.UploadSourceZip(ctx, zoneId, projectId, plan.SourceZip.ValueString())
		if err != nil {
			resp.Diagnostics.Append(fwhelper.WrapSDKError("Error uploading Makers deployment source", err))
			return
		}
		log.Printf("[DEBUG] uploaded archive to %s", key)
		distType, targetPath = makersDistTypeZip, key
	} else {
		prefix, count, err := service.UploadSource(ctx, zoneId, projectId, plan.SourceDir.ValueString())
		if err != nil {
			resp.Diagnostics.Append(fwhelper.WrapSDKError("Error uploading Makers deployment source", err))
			return
		}
		log.Printf("[DEBUG] uploaded %d files to %s", count, prefix)
		targetPath = prefix
	}

	deploymentId, err := service.CreateDeployment(ctx, zoneId, projectId, plan.Environment.ValueString(), distType, targetPath)
	if err != nil {
		resp.Diagnostics.Append(fwhelper.WrapSDKError("Error creating Makers deployment", err))
		return
	}

	plan.Id = types.StringValue(strings.Join([]string{zoneId, projectId, deploymentId}, tccommon.FILED_SP))
	plan.DeploymentId = types.StringValue(deploymentId)
	plan.Status = types.StringValue("Pending")
	plan.PreviewUrl = types.StringValue("")
	plan.ProjectUrl = types.StringValue("")
	plan.CreatedOn = types.StringValue("")
	// Persist the identity before waiting, so that a failed or timed-out build
	// is tainted and replaced by the next apply.
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	deployment, err := r.wait(ctx, service, zoneId, projectId, deploymentId)
	if deployment != nil {
		makersFlattenDeployment(&plan, deployment)
		resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
	}
	if err != nil {
		resp.Diagnostics.AddError("Error waiting for Makers deployment",
			fmt.Sprintf("deployment %s did not finish: %v", deploymentId, err))
		return
	}
	if deployment.Status != "Success" {
		tail, logErr := service.DeploymentLogTail(context.Background(), zoneId, projectId, deploymentId, makersDeploymentLogLines)
		if logErr != nil {
			tail = fmt.Sprintf("(build log unavailable: %v)", logErr)
		}
		resp.Diagnostics.AddError("Makers deployment failed",
			fmt.Sprintf("deployment %s finished with status %s, code %d.\n\nBuild log (last %d lines):\n%s",
				deploymentId, deployment.Status, deployment.Code, makersDeploymentLogLines, tail))
	}
}

func (r *TeoMakersDeploymentResource) wait(ctx context.Context, service TeoMakersService, zoneId, projectId, deploymentId string) (*MakersDeployment, error) {
	var last *MakersDeployment
	for {
		select {
		case <-ctx.Done():
			return last, ctx.Err()
		case <-time.After(makersDeploymentPollInterval):
		}
		d, err := service.DescribeDeploymentById(ctx, zoneId, projectId, deploymentId)
		if err != nil {
			if fwhelper.IsSDKErrorCode(err, "InternalError", "RequestLimitExceeded") {
				continue
			}
			return last, err
		}
		if d == nil {
			continue
		}
		last = d
		if makersDeploymentFinished(d.Status) {
			return d, nil
		}
	}
}

func (r *TeoMakersDeploymentResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	defer tccommon.LogElapsed("resource.tencentcloud_teo_makers_deployment.read")()

	var state TeoMakersDeploymentResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	d, err := NewTeoMakersService(r.Client()).DescribeDeploymentById(ctx,
		state.ZoneId.ValueString(), state.ProjectId.ValueString(), state.DeploymentId.ValueString())
	if err != nil {
		resp.Diagnostics.Append(fwhelper.WrapSDKError("Error reading Makers deployment", err))
		return
	}
	if d == nil {
		resp.Diagnostics.AddWarning("Makers deployment not found",
			fmt.Sprintf("deployment %s does not exist any more, removing it from state.", state.DeploymentId.ValueString()))
		resp.State.RemoveResource(ctx)
		return
	}
	makersFlattenDeployment(&state, d)
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

// Update only stores attributes that do not trigger a new deployment, such as
// `source_dir` or `source_zip` pointing to identical content, or `timeouts`.
func (r *TeoMakersDeploymentResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan, state TeoMakersDeploymentResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	plan.Id = state.Id
	plan.DeploymentId = state.DeploymentId
	plan.Status = state.Status
	plan.PreviewUrl = state.PreviewUrl
	plan.ProjectUrl = state.ProjectUrl
	plan.CreatedOn = state.CreatedOn
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

// Delete keeps the deployment on the platform: deployments are history that
// can be rolled back to, and the one serving production cannot be deleted.
func (r *TeoMakersDeploymentResource) Delete(_ context.Context, _ resource.DeleteRequest, _ *resource.DeleteResponse) {
}

func (r *TeoMakersDeploymentResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	parts := strings.Split(req.ID, tccommon.FILED_SP)
	if len(parts) != 3 || parts[0] == "" || parts[1] == "" || parts[2] == "" {
		resp.Diagnostics.AddError("Invalid import id", "The import id must be in the format `zone_id#project_id#deployment_id`.")
		return
	}
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("id"), req.ID)...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("zone_id"), parts[0])...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("project_id"), parts[1])...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("deployment_id"), parts[2])...)
}

func makersFlattenDeployment(model *TeoMakersDeploymentResourceModel, d *MakersDeployment) {
	// Environment slugs are case-insensitive on the platform.
	if !strings.EqualFold(model.Environment.ValueString(), d.Env) {
		model.Environment = types.StringValue(d.Env)
	}
	model.Status = types.StringValue(d.Status)
	model.PreviewUrl = types.StringValue(d.PreviewUrl)
	model.ProjectUrl = types.StringValue(d.ProjectUrl)
	model.CreatedOn = types.StringValue(d.CreatedOn)
}
