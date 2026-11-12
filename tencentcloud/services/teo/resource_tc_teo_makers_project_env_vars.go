package teo

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"github.com/hashicorp/terraform-plugin-framework/attr"
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
	_ resource.ResourceWithConfigure      = &TeoMakersProjectEnvVarsResource{}
	_ resource.ResourceWithImportState    = &TeoMakersProjectEnvVarsResource{}
	_ resource.ResourceWithValidateConfig = &TeoMakersProjectEnvVarsResource{}
)

// NewTeoMakersProjectEnvVarsResource is the factory referenced by
// tencentcloud/framework/registry.go to register this resource.
func NewTeoMakersProjectEnvVarsResource() resource.Resource {
	return &TeoMakersProjectEnvVarsResource{}
}

// TeoMakersProjectEnvVarsResource implements
// tencentcloud_teo_makers_project_env_vars. It owns the complete set of
// environment variables of a project.
type TeoMakersProjectEnvVarsResource struct {
	fw.ResourceWithConfigure
}

type TeoMakersProjectEnvVarsResourceModel struct {
	Id        types.String `tfsdk:"id"`
	ZoneId    types.String `tfsdk:"zone_id"`
	ProjectId types.String `tfsdk:"project_id"`
	EnvVars   types.Set    `tfsdk:"env_vars"`
}

type makersEnvVarModel struct {
	Key          types.String `tfsdk:"key"`
	Value        types.String `tfsdk:"value"`
	Comment      types.String `tfsdk:"comment"`
	Environments types.Set    `tfsdk:"environments"`
}

var makersEnvVarAttrTypes = map[string]attr.Type{
	"key":          types.StringType,
	"value":        types.StringType,
	"comment":      types.StringType,
	"environments": types.SetType{ElemType: types.StringType},
}

func (r *TeoMakersProjectEnvVarsResource) Metadata(_ context.Context, _ resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = "tencentcloud_teo_makers_project_env_vars"
}

func (r *TeoMakersProjectEnvVarsResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Provides a resource to manage the complete set of environment variables of an EdgeOne Makers " +
			"project. Variables that exist on the project but are not listed here are deleted.",
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
			"env_vars": schema.SetNestedAttribute{
				Required: true,
				Description: "Environment variables. The same key may appear more than once when the `environments` " +
					"of those entries do not overlap.",
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						"key": schema.StringAttribute{
							Required:    true,
							Description: "Name of the variable, at most 255 characters.",
							Validators:  []validator.String{makersTrimmed(255), makersNonEmpty()},
						},
						"value": schema.StringAttribute{
							Required:    true,
							Sensitive:   true,
							Description: "Value of the variable, at most 1000 characters.",
							Validators:  []validator.String{makersTrimmed(1000), makersNonEmpty()},
						},
						"comment": schema.StringAttribute{
							Optional:    true,
							Description: "Comment of the variable, at most 255 characters.",
							Validators:  []validator.String{makersTrimmed(255), makersNonEmpty()},
						},
						"environments": schema.SetAttribute{
							Optional:    true,
							ElementType: types.StringType,
							Description: "Slugs of the environments the variable applies to, for example `Production`, " +
								"`Preview` or a custom environment slug. When it is not set, the variable applies to every " +
								"environment that exists on the project when it is written.",
						},
					},
				},
			},
		},
	}
}

func makersNonEmpty() validator.String {
	return makersStringCheck{
		description: "value must not be empty",
		check: func(s string) string {
			if s == "" {
				return "value must not be empty"
			}
			return ""
		},
	}
}

func (r *TeoMakersProjectEnvVarsResource) ValidateConfig(ctx context.Context, req resource.ValidateConfigRequest, resp *resource.ValidateConfigResponse) {
	var config TeoMakersProjectEnvVarsResourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &config)...)
	if resp.Diagnostics.HasError() || config.EnvVars.IsUnknown() || config.EnvVars.IsNull() {
		return
	}
	var items []makersEnvVarModel
	resp.Diagnostics.Append(config.EnvVars.ElementsAs(ctx, &items, false)...)
	if resp.Diagnostics.HasError() {
		return
	}

	scopes := map[string][][]string{}
	for _, item := range items {
		if item.Key.IsUnknown() || item.Environments.IsUnknown() {
			return
		}
		var envs []string
		if !item.Environments.IsNull() {
			resp.Diagnostics.Append(item.Environments.ElementsAs(ctx, &envs, false)...)
			if len(envs) == 0 {
				resp.Diagnostics.AddAttributeError(path.Root("env_vars"), "Invalid environments",
					fmt.Sprintf("environments of %q must not be empty; leave it unset to apply to every environment", item.Key.ValueString()))
			}
		}
		key := item.Key.ValueString()
		for _, prev := range scopes[key] {
			if prev == nil || envs == nil || makersOverlap(prev, envs) {
				resp.Diagnostics.AddAttributeError(path.Root("env_vars"), "Overlapping environment variables",
					fmt.Sprintf("%q is defined more than once for the same environment", key))
			}
		}
		scopes[key] = append(scopes[key], envs)
	}
}

func makersOverlap(a, b []string) bool {
	for _, x := range a {
		for _, y := range b {
			if x == y {
				return true
			}
		}
	}
	return false
}

func (r *TeoMakersProjectEnvVarsResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	defer tccommon.LogElapsed("resource.tencentcloud_teo_makers_project_env_vars.create")()

	var plan TeoMakersProjectEnvVarsResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(r.apply(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	plan.Id = types.StringValue(plan.ZoneId.ValueString() + tccommon.FILED_SP + plan.ProjectId.ValueString())
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *TeoMakersProjectEnvVarsResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	defer tccommon.LogElapsed("resource.tencentcloud_teo_makers_project_env_vars.read")()

	var state TeoMakersProjectEnvVarsResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	zoneId, projectId := state.ZoneId.ValueString(), state.ProjectId.ValueString()
	service := NewTeoMakersService(r.Client())
	project, err := service.DescribeProjectById(ctx, zoneId, projectId)
	if err != nil {
		resp.Diagnostics.Append(fwhelper.WrapSDKError("Error reading Makers project", err))
		return
	}
	if project == nil {
		resp.Diagnostics.AddWarning("Makers project not found",
			fmt.Sprintf("project %s does not exist any more, removing its environment variables from state.", projectId))
		resp.State.RemoveResource(ctx)
		return
	}

	remote, allEnvs, err := r.describe(ctx, zoneId, projectId)
	if err != nil {
		resp.Diagnostics.Append(fwhelper.WrapSDKError("Error reading Makers project environment variables", err))
		return
	}

	var prior []makersEnvVarModel
	if !state.EnvVars.IsNull() && !state.EnvVars.IsUnknown() {
		resp.Diagnostics.Append(state.EnvVars.ElementsAs(ctx, &prior, false)...)
	}
	set, diags := makersFlattenEnvVars(ctx, remote, allEnvs, prior)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
	state.EnvVars = set
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *TeoMakersProjectEnvVarsResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	defer tccommon.LogElapsed("resource.tencentcloud_teo_makers_project_env_vars.update")()

	var plan, state TeoMakersProjectEnvVarsResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(r.apply(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	plan.Id = state.Id
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

// Delete removes every environment variable of the project, because this
// resource owns the complete set.
func (r *TeoMakersProjectEnvVarsResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	defer tccommon.LogElapsed("resource.tencentcloud_teo_makers_project_env_vars.delete")()

	var state TeoMakersProjectEnvVarsResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	zoneId, projectId := state.ZoneId.ValueString(), state.ProjectId.ValueString()
	service := NewTeoMakersService(r.Client())
	remote, err := service.DescribeProjectEnvVars(ctx, zoneId, projectId)
	if err != nil {
		if makersIsNotFound(err) {
			return
		}
		resp.Diagnostics.Append(fwhelper.WrapSDKError("Error reading Makers project environment variables", err))
		return
	}
	var ids []int64
	for _, row := range remote {
		if row.Id != nil {
			ids = append(ids, *row.Id)
		}
	}
	if err := service.DeleteProjectEnvVarsById(ctx, zoneId, projectId, ids); err != nil && !makersIsNotFound(err) {
		resp.Diagnostics.Append(fwhelper.WrapSDKError("Error deleting Makers project environment variables", err))
	}
}

func (r *TeoMakersProjectEnvVarsResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	parts := strings.Split(req.ID, tccommon.FILED_SP)
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		resp.Diagnostics.AddError("Invalid import id", "The import id must be in the format `zone_id#project_id`.")
		return
	}
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("id"), req.ID)...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("zone_id"), parts[0])...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("project_id"), parts[1])...)
}

func (r *TeoMakersProjectEnvVarsResource) describe(ctx context.Context, zoneId, projectId string) ([]MakersEnvVar, []string, error) {
	service := NewTeoMakersService(r.Client())
	remote, err := service.DescribeProjectEnvVars(ctx, zoneId, projectId)
	if err != nil {
		return nil, nil, err
	}
	environments, err := service.DescribeEnvironments(ctx, zoneId, projectId)
	if err != nil {
		return nil, nil, err
	}
	allEnvs := make([]string, 0, len(environments))
	for _, e := range environments {
		allEnvs = append(allEnvs, e.Slug)
	}
	return remote, allEnvs, nil
}

func (r *TeoMakersProjectEnvVarsResource) apply(ctx context.Context, plan *TeoMakersProjectEnvVarsResourceModel) (diags diag.Diagnostics) {
	var desired []makersEnvVarModel
	diags.Append(plan.EnvVars.ElementsAs(ctx, &desired, false)...)
	if diags.HasError() {
		return
	}
	zoneId, projectId := plan.ZoneId.ValueString(), plan.ProjectId.ValueString()
	remote, allEnvs, err := r.describe(ctx, zoneId, projectId)
	if err != nil {
		diags.Append(fwhelper.WrapSDKError("Error reading Makers project environment variables", err))
		return
	}

	wanted := make([]MakersEnvVar, 0, len(desired))
	for _, d := range desired {
		item := MakersEnvVar{Key: d.Key.ValueString(), Value: d.Value.ValueString()}
		comment := ""
		if !d.Comment.IsNull() {
			comment = d.Comment.ValueString()
		}
		item.Comment = &comment
		if !d.Environments.IsNull() {
			diags.Append(d.Environments.ElementsAs(ctx, &item.Env, false)...)
		}
		wanted = append(wanted, item)
	}
	if diags.HasError() {
		return
	}

	deletes, upserts := makersReconcileEnvVars(remote, wanted, allEnvs)
	service := NewTeoMakersService(r.Client())
	if err := service.DeleteProjectEnvVarsById(ctx, zoneId, projectId, deletes); err != nil {
		diags.Append(fwhelper.WrapSDKError("Error deleting Makers project environment variables", err))
		return
	}
	if err := service.ModifyProjectEnvVars(ctx, zoneId, projectId, upserts); err != nil {
		diags.Append(fwhelper.WrapSDKError("Error writing Makers project environment variables", err))
	}
	return
}

// makersReconcileEnvVars matches rows by key and environment scope, where a
// missing scope stands for allEnvs. It returns the ids of rows to delete and
// the rows to create or update. Deletes must run first so that a scope can
// move between rows of the same key without overlapping.
func makersReconcileEnvVars(remote, wanted []MakersEnvVar, allEnvs []string) ([]int64, []MakersEnvVar) {
	scopeOf := func(env []string) string {
		if len(env) == 0 {
			env = allEnvs
		}
		s := append([]string(nil), env...)
		sort.Strings(s)
		return strings.Join(s, ",")
	}
	commentOf := func(c *string) string {
		if c == nil {
			return ""
		}
		return *c
	}

	byScope := map[string]*MakersEnvVar{}
	for i := range remote {
		byScope[remote[i].Key+"\x00"+scopeOf(remote[i].Env)] = &remote[i]
	}

	var upserts []MakersEnvVar
	kept := map[*MakersEnvVar]bool{}
	for _, w := range wanted {
		row := byScope[w.Key+"\x00"+scopeOf(w.Env)]
		if row == nil {
			upserts = append(upserts, w)
			continue
		}
		kept[row] = true
		if row.Value != w.Value || commentOf(row.Comment) != commentOf(w.Comment) {
			w.Id = row.Id
			upserts = append(upserts, w)
		}
	}

	var deletes []int64
	for i := range remote {
		if !kept[&remote[i]] && remote[i].Id != nil {
			deletes = append(deletes, *remote[i].Id)
		}
	}
	return deletes, upserts
}

// makersFlattenEnvVars converts remote rows to state. A row scoped to every
// environment is written with a null `environments`, unless the prior state
// listed that scope explicitly.
func makersFlattenEnvVars(ctx context.Context, remote []MakersEnvVar, allEnvs []string, prior []makersEnvVarModel) (types.Set, diag.Diagnostics) {
	var diags diag.Diagnostics
	sortedJoin := func(env []string) string {
		s := append([]string(nil), env...)
		sort.Strings(s)
		return strings.Join(s, ",")
	}
	all := sortedJoin(allEnvs)

	explicit := map[string]bool{}
	for _, p := range prior {
		if p.Environments.IsNull() || p.Environments.IsUnknown() {
			continue
		}
		var envs []string
		diags.Append(p.Environments.ElementsAs(ctx, &envs, false)...)
		explicit[p.Key.ValueString()+"\x00"+sortedJoin(envs)] = true
	}

	elems := make([]attr.Value, 0, len(remote))
	for _, row := range remote {
		envs := types.SetNull(types.StringType)
		scope := sortedJoin(row.Env)
		if scope != all || explicit[row.Key+"\x00"+scope] {
			var d diag.Diagnostics
			envs, d = types.SetValueFrom(ctx, types.StringType, row.Env)
			diags.Append(d...)
		}
		comment := types.StringNull()
		if row.Comment != nil && *row.Comment != "" {
			comment = types.StringValue(*row.Comment)
		}
		obj, d := types.ObjectValue(makersEnvVarAttrTypes, map[string]attr.Value{
			"key":          types.StringValue(row.Key),
			"value":        types.StringValue(row.Value),
			"comment":      comment,
			"environments": envs,
		})
		diags.Append(d...)
		elems = append(elems, obj)
	}
	set, d := types.SetValue(types.ObjectType{AttrTypes: makersEnvVarAttrTypes}, elems)
	diags.Append(d...)
	return set, diags
}
