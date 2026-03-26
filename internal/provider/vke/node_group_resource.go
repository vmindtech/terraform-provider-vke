package vke

import (
	"context"
	"errors"
	"strings"

	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/int64planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/listplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/vmindtech/terraform-provider-portvmind/internal/client"
)

var (
	_ resource.Resource                = &nodeGroupResource{}
	_ resource.ResourceWithImportState = &nodeGroupResource{}
)

type nodeGroupResource struct {
	client *client.VKEClient
}

func NewNodeGroupResource() resource.Resource {
	return &nodeGroupResource{}
}

func (r *nodeGroupResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_vke_node_group"
}

func (r *nodeGroupResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Creates, updates, or deletes a worker node group on a VKE cluster.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Description: "Node group UUID.",
				Computed:    true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"cluster_id": schema.StringAttribute{
				Description: "Parent cluster UUID.",
				Required:    true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"name": schema.StringAttribute{
				Description: "Node group name (max 20 characters).",
				Required:    true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"node_flavor_uuid": schema.StringAttribute{
				Description: "Nova flavor UUID.",
				Required:    true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"node_disk_size": schema.Int64Attribute{
				Description: "Node disk size in GB.",
				Required:    true,
				PlanModifiers: []planmodifier.Int64{
					int64planmodifier.RequiresReplace(),
				},
			},
			"node_group_labels": schema.ListAttribute{
				Description: "Kubernetes labels (e.g. key=value). If empty, the API default applies.",
				Optional:    true,
				ElementType: types.StringType,
				PlanModifiers: []planmodifier.List{
					listplanmodifier.RequiresReplace(),
				},
			},
			"node_group_taints": schema.ListAttribute{
				Description: "Taint listesi (key=value:Effect).",
				Optional:    true,
				ElementType: types.StringType,
				PlanModifiers: []planmodifier.List{
					listplanmodifier.RequiresReplace(),
				},
			},
			"min_size": schema.Int64Attribute{
				Description: "Minimum number of nodes.",
				Required:    true,
			},
			"max_size": schema.Int64Attribute{
				Description: "Maximum number of nodes.",
				Required:    true,
			},
			"current_nodes": schema.Int64Attribute{
				Description: "Current number of nodes.",
				Computed:    true,
			},
			"status": schema.StringAttribute{
				Description: "Node group status.",
				Computed:    true,
			},
		},
	}
}

func (r *nodeGroupResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}
	c, ok := req.ProviderData.(*client.VKEClient)
	if !ok {
		resp.Diagnostics.AddError("Configuration error", "Expected *client.VKEClient in provider data.")
		return
	}
	r.client = c
}

func (r *nodeGroupResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	parts := strings.Split(req.ID, "/")
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		resp.Diagnostics.AddError("Invalid import ID", "Expected format: <cluster_uuid>/<node_group_uuid>")
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &nodeGroupResourceModel{
		ClusterID: types.StringValue(parts[0]),
		ID:        types.StringValue(parts[1]),
	})...)
}

func (r *nodeGroupResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan nodeGroupResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	labels, d := tfStringListToSlice(ctx, plan.NodeGroupLabels)
	resp.Diagnostics.Append(d...)
	taints, d2 := tfStringListToSlice(ctx, plan.NodeGroupTaints)
	resp.Diagnostics.Append(d2...)
	if resp.Diagnostics.HasError() {
		return
	}

	apiReq := client.CreateNodeGroupRequest{
		NodeGroupName:    plan.Name.ValueString(),
		NodeFlavorUUID:   plan.NodeFlavorUUID.ValueString(),
		NodeDiskSize:     int(plan.NodeDiskSize.ValueInt64()),
		NodeGroupLabels:  labels,
		NodeGroupTaints:  taints,
		NodeGroupMinSize: int(plan.MinSize.ValueInt64()),
		NodeGroupMaxSize: int(plan.MaxSize.ValueInt64()),
	}

	out, err := r.client.CreateNodeGroup(ctx, plan.ClusterID.ValueString(), apiReq)
	if err != nil {
		resp.Diagnostics.AddError("Could not create node group", err.Error())
		return
	}

	plan.ID = types.StringValue(out.NodeGroupID)

	ng, err := r.client.GetNodeGroup(ctx, plan.ClusterID.ValueString(), out.NodeGroupID)
	if err != nil {
		resp.Diagnostics.AddError("Could not read node group", err.Error())
		return
	}
	plan.CurrentNodes = types.Int64Value(int64(ng.CurrentNodes))
	plan.Status = types.StringValue(ng.NodeGroupsStatus)

	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *nodeGroupResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state nodeGroupResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	ng, err := r.client.GetNodeGroup(ctx, state.ClusterID.ValueString(), state.ID.ValueString())
	if err != nil {
		var nf client.ErrNotFound
		if errors.As(err, &nf) {
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError("Could not read node group", err.Error())
		return
	}

	state.NodeFlavorUUID = types.StringValue(ng.NodeFlavorUUID)
	state.NodeDiskSize = types.Int64Value(int64(ng.NodeDiskSize))
	state.MinSize = types.Int64Value(int64(ng.NodeGroupMinSize))
	state.MaxSize = types.Int64Value(int64(ng.NodeGroupMaxSize))
	state.CurrentNodes = types.Int64Value(int64(ng.CurrentNodes))
	state.Status = types.StringValue(ng.NodeGroupsStatus)

	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *nodeGroupResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan nodeGroupResourceModel
	var state nodeGroupResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	if plan.MinSize.Equal(state.MinSize) && plan.MaxSize.Equal(state.MaxSize) {
		return
	}

	mn := uint32(plan.MinSize.ValueInt64())
	mx := uint32(plan.MaxSize.ValueInt64())
	_, err := r.client.UpdateNodeGroup(ctx, state.ClusterID.ValueString(), state.ID.ValueString(), client.UpdateNodeGroupRequest{
		MinNodes: &mn,
		MaxNodes: &mx,
	})
	if err != nil {
		resp.Diagnostics.AddError("Could not update node group", err.Error())
		return
	}

	ng, err := r.client.GetNodeGroup(ctx, state.ClusterID.ValueString(), state.ID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Could not read node group", err.Error())
		return
	}
	plan.CurrentNodes = types.Int64Value(int64(ng.CurrentNodes))
	plan.Status = types.StringValue(ng.NodeGroupsStatus)

	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *nodeGroupResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state nodeGroupResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	if err := r.client.DeleteNodeGroup(ctx, state.ClusterID.ValueString(), state.ID.ValueString()); err != nil {
		resp.Diagnostics.AddError("Could not delete node group", err.Error())
		return
	}
}

type nodeGroupResourceModel struct {
	ClusterID       types.String `tfsdk:"cluster_id"`
	ID              types.String `tfsdk:"id"`
	Name            types.String `tfsdk:"name"`
	NodeFlavorUUID  types.String `tfsdk:"node_flavor_uuid"`
	NodeDiskSize    types.Int64  `tfsdk:"node_disk_size"`
	NodeGroupLabels types.List   `tfsdk:"node_group_labels"`
	NodeGroupTaints types.List   `tfsdk:"node_group_taints"`
	MinSize         types.Int64  `tfsdk:"min_size"`
	MaxSize         types.Int64  `tfsdk:"max_size"`
	CurrentNodes    types.Int64  `tfsdk:"current_nodes"`
	Status          types.String `tfsdk:"status"`
}
