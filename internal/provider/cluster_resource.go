package provider

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/int64planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/listplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/vmindtech/terraform-provider-vke/internal/client"
)

var _ resource.Resource = &clusterResource{}

type clusterResource struct {
	client *client.VKEClient
}

func NewClusterResource() resource.Resource {
	return &clusterResource{}
}

func (r *clusterResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_cluster"
}

func (r *clusterResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Creates or destroys a VKE Kubernetes cluster. Populates kubeconfig after the cluster is Active.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Description: "Cluster UUID.",
				Computed:    true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"project_id": schema.StringAttribute{
				Description: "OpenStack project UUID (VKE projectId).",
				Required:    true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"name": schema.StringAttribute{
				Description: "Cluster name (clusterName).",
				Required:    true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"kubernetes_version": schema.StringAttribute{
				Description: "Kubernetes version.",
				Required:    true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"node_key_pair_name": schema.StringAttribute{
				Description: "Nova keypair name.",
				Required:    true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"cluster_api_access": schema.StringAttribute{
				Description: "Cluster API access address (clusterApiAccess).",
				Required:    true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"subnet_ids": schema.ListAttribute{
				Description: "List of subnet UUIDs.",
				Required:    true,
				ElementType: types.StringType,
				PlanModifiers: []planmodifier.List{
					listplanmodifier.RequiresReplace(),
				},
			},
			"worker_node_group_min_size": schema.Int64Attribute{
				Description: "Initial worker node group minimum size.",
				Required:    true,
				PlanModifiers: []planmodifier.Int64{
					int64planmodifier.RequiresReplace(),
				},
			},
			"worker_node_group_max_size": schema.Int64Attribute{
				Description: "Initial worker node group maximum size.",
				Required:    true,
				PlanModifiers: []planmodifier.Int64{
					int64planmodifier.RequiresReplace(),
				},
			},
			"worker_instance_flavor_uuid": schema.StringAttribute{
				Description: "Worker flavor UUID.",
				Required:    true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"master_instance_flavor_uuid": schema.StringAttribute{
				Description: "Master flavor UUID.",
				Required:    true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"worker_disk_size_gb": schema.Int64Attribute{
				Description: "Worker disk size in GB (minimum 20).",
				Required:    true,
				PlanModifiers: []planmodifier.Int64{
					int64planmodifier.RequiresReplace(),
				},
			},
			"allowed_cidrs": schema.ListAttribute{
				Description: "CIDRs allowed to access the API.",
				Required:    true,
				ElementType: types.StringType,
				PlanModifiers: []planmodifier.List{
					listplanmodifier.RequiresReplace(),
				},
			},
			"status": schema.StringAttribute{
				Description: "Cluster status (e.g. Active).",
				Computed:    true,
			},
			"kubeconfig": schema.StringAttribute{
				Description: "Kubeconfig YAML after the cluster becomes Active.",
				Computed:    true,
				Sensitive:   true,
			},
			"create_timeout_minutes": schema.Int64Attribute{
				Description: "Timeout in minutes waiting for Active status. Default 60.",
				Optional:    true,
			},
			"delete_timeout_minutes": schema.Int64Attribute{
				Description: "Timeout in minutes waiting for delete to complete. Default 30.",
				Optional:    true,
			},
		},
	}
}

func (r *clusterResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func (r *clusterResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan clusterResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	subnets, d := tfStringListToSlice(ctx, plan.SubnetIDs)
	resp.Diagnostics.Append(d...)
	cidrs, d2 := tfStringListToSlice(ctx, plan.AllowedCIDRs)
	resp.Diagnostics.Append(d2...)
	if resp.Diagnostics.HasError() {
		return
	}

	createTO := plan.createTimeout()
	deleteTO := plan.deleteTimeout()

	apiReq := client.CreateClusterRequest{
		ClusterName:              plan.Name.ValueString(),
		ProjectID:                plan.ProjectID.ValueString(),
		KubernetesVersion:        plan.KubernetesVersion.ValueString(),
		NodeKeyPairName:          plan.NodeKeyPairName.ValueString(),
		ClusterAPIAccess:         plan.ClusterAPIAccess.ValueString(),
		SubnetIDs:                subnets,
		WorkerNodeGroupMinSize:   int(plan.WorkerNodeGroupMinSize.ValueInt64()),
		WorkerNodeGroupMaxSize:   int(plan.WorkerNodeGroupMaxSize.ValueInt64()),
		WorkerInstanceFlavorUUID: plan.WorkerInstanceFlavorUUID.ValueString(),
		MasterInstanceFlavorUUID: plan.MasterInstanceFlavorUUID.ValueString(),
		WorkerDiskSizeGB:         int(plan.WorkerDiskSizeGB.ValueInt64()),
		AllowedCIDRS:             cidrs,
	}

	out, err := r.client.CreateCluster(ctx, apiReq)
	if err != nil {
		resp.Diagnostics.AddError("Could not create cluster", err.Error())
		return
	}

	plan.ID = types.StringValue(out.ClusterUUID)
	plan.Status = types.StringValue(out.ClusterStatus)
	plan.CreateTimeoutMinutes = types.Int64Value(createTO)
	plan.DeleteTimeoutMinutes = types.Int64Value(deleteTO)

	if err := r.waitClusterActive(ctx, out.ClusterUUID, time.Duration(createTO)*time.Minute); err != nil {
		resp.Diagnostics.AddError("Cluster did not become ready", err.Error())
		return
	}

	st, err := r.client.GetCluster(ctx, out.ClusterUUID)
	if err != nil {
		resp.Diagnostics.AddError("Could not read cluster", err.Error())
		return
	}
	plan.Status = types.StringValue(st.ClusterStatus)

	kube, err := r.client.GetKubeYAML(ctx, out.ClusterUUID)
	if err != nil {
		resp.Diagnostics.AddError("Could not fetch kubeconfig", err.Error())
		return
	}
	plan.Kubeconfig = types.StringValue(kube)

	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *clusterResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state clusterResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	st, err := r.client.GetCluster(ctx, state.ID.ValueString())
	if err != nil {
		var nf client.ErrNotFound
		if errors.As(err, &nf) {
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError("Could not read cluster", err.Error())
		return
	}

	state.Status = types.StringValue(st.ClusterStatus)

	if strings.EqualFold(st.ClusterStatus, client.ClusterStatusActive) {
		kube, err := r.client.GetKubeYAML(ctx, state.ID.ValueString())
		if err == nil {
			state.Kubeconfig = types.StringValue(kube)
		}
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *clusterResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan clusterResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	st, err := r.client.GetCluster(ctx, plan.ID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Could not read cluster", err.Error())
		return
	}
	plan.Status = types.StringValue(st.ClusterStatus)

	if strings.EqualFold(st.ClusterStatus, client.ClusterStatusActive) {
		kube, err := r.client.GetKubeYAML(ctx, plan.ID.ValueString())
		if err == nil {
			plan.Kubeconfig = types.StringValue(kube)
		}
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *clusterResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state clusterResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	if err := r.client.DestroyCluster(ctx, state.ID.ValueString()); err != nil {
		resp.Diagnostics.AddError("Could not delete cluster", err.Error())
		return
	}

	delTO := state.deleteTimeout()
	if err := r.waitClusterGone(ctx, state.ID.ValueString(), time.Duration(delTO)*time.Minute); err != nil {
		resp.Diagnostics.AddError("Cluster delete did not complete", err.Error())
		return
	}
}

// transientClusterPollError returns true when GetCluster failed but polling should continue
// (e.g. VKE occasionally returns 5xx/422 while the cluster is still provisioning).
func transientClusterPollError(err error) bool {
	var h client.HTTPStatusError
	if !errors.As(err, &h) {
		return false
	}
	switch h.StatusCode {
	case http.StatusTooManyRequests: // 429
		return true
	case http.StatusInternalServerError, http.StatusBadGateway,
		http.StatusServiceUnavailable, http.StatusGatewayTimeout:
		return true
	case http.StatusUnprocessableEntity: // 422 — some deployments wrap transient backend errors here
		return true
	default:
		return false
	}
}

func (r *clusterResource) waitClusterActive(ctx context.Context, id string, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	tick := 30 * time.Second
	for {
		cl, err := r.client.GetCluster(ctx, id)
		if err != nil {
			if transientClusterPollError(err) && time.Now().Before(deadline) {
				select {
				case <-ctx.Done():
					return ctx.Err()
				case <-time.After(tick):
				}
				continue
			}
			return err
		}
		if strings.EqualFold(cl.ClusterStatus, client.ClusterStatusActive) {
			return nil
		}
		if strings.EqualFold(cl.ClusterStatus, "Error") {
			return fmt.Errorf("cluster entered Error status")
		}
		if time.Now().After(deadline) {
			return fmt.Errorf("timeout waiting for Active after %v (last status: %s)", timeout, cl.ClusterStatus)
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(tick):
		}
	}
}

func (r *clusterResource) waitClusterGone(ctx context.Context, id string, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	tick := 15 * time.Second
	for {
		cl, err := r.client.GetCluster(ctx, id)
		if err != nil {
			var nf client.ErrNotFound
			if errors.As(err, &nf) {
				return nil
			}
			return err
		}
		if strings.EqualFold(cl.ClusterStatus, "Deleted") {
			return nil
		}
		if time.Now().After(deadline) {
			return fmt.Errorf("timeout waiting for cluster to be deleted after %v (last status: %s)", timeout, cl.ClusterStatus)
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(tick):
		}
	}
}

type clusterResourceModel struct {
	ID                          types.String `tfsdk:"id"`
	ProjectID                   types.String `tfsdk:"project_id"`
	Name                        types.String `tfsdk:"name"`
	KubernetesVersion           types.String `tfsdk:"kubernetes_version"`
	NodeKeyPairName             types.String `tfsdk:"node_key_pair_name"`
	ClusterAPIAccess            types.String `tfsdk:"cluster_api_access"`
	SubnetIDs                   types.List   `tfsdk:"subnet_ids"`
	WorkerNodeGroupMinSize      types.Int64  `tfsdk:"worker_node_group_min_size"`
	WorkerNodeGroupMaxSize      types.Int64  `tfsdk:"worker_node_group_max_size"`
	WorkerInstanceFlavorUUID    types.String `tfsdk:"worker_instance_flavor_uuid"`
	MasterInstanceFlavorUUID    types.String `tfsdk:"master_instance_flavor_uuid"`
	WorkerDiskSizeGB            types.Int64  `tfsdk:"worker_disk_size_gb"`
	AllowedCIDRs                types.List   `tfsdk:"allowed_cidrs"`
	Status                      types.String `tfsdk:"status"`
	Kubeconfig                  types.String `tfsdk:"kubeconfig"`
	CreateTimeoutMinutes        types.Int64  `tfsdk:"create_timeout_minutes"`
	DeleteTimeoutMinutes        types.Int64  `tfsdk:"delete_timeout_minutes"`
}

func (m *clusterResourceModel) createTimeout() int64 {
	if m.CreateTimeoutMinutes.IsNull() || m.CreateTimeoutMinutes.IsUnknown() {
		return 60
	}
	v := m.CreateTimeoutMinutes.ValueInt64()
	if v <= 0 {
		return 60
	}
	return v
}

func (m *clusterResourceModel) deleteTimeout() int64 {
	if m.DeleteTimeoutMinutes.IsNull() || m.DeleteTimeoutMinutes.IsUnknown() {
		return 30
	}
	v := m.DeleteTimeoutMinutes.ValueInt64()
	if v <= 0 {
		return 30
	}
	return v
}
