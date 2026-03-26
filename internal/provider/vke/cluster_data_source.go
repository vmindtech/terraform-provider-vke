package vke

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/vmindtech/terraform-provider-portvmind/internal/client"
)

var _ datasource.DataSource = &clusterDataSource{}

type clusterDataSource struct {
	client *client.VKEClient
}

func NewClusterDataSource() datasource.DataSource {
	return &clusterDataSource{}
}

func (d *clusterDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_vke_cluster"
}

func (d *clusterDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Reads metadata for an existing VKE cluster (GET /cluster/:id).",
		Attributes: map[string]schema.Attribute{
			"cluster_id": schema.StringAttribute{
				Description: "Cluster UUID.",
				Required:    true,
			},
			"id": schema.StringAttribute{
				Description: "Cluster UUID (Terraform resource id).",
				Computed:    true,
			},
			"name": schema.StringAttribute{
				Computed: true,
			},
			"project_id": schema.StringAttribute{
				Computed: true,
			},
			"kubernetes_version": schema.StringAttribute{
				Computed: true,
			},
			"api_access": schema.StringAttribute{
				Description: "Cluster API endpoint address.",
				Computed:    true,
			},
			"status": schema.StringAttribute{
				Computed: true,
			},
			"shared_security_group": schema.StringAttribute{
				Computed: true,
			},
		},
	}
}

func (d *clusterDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}
	c, ok := req.ProviderData.(*client.VKEClient)
	if !ok {
		resp.Diagnostics.AddError(
			"Unexpected provider data type",
			fmt.Sprintf("Expected *client.VKEClient, got %T.", req.ProviderData),
		)
		return
	}
	d.client = c
}

func (d *clusterDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var data clusterDataSourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	cl, err := d.client.GetCluster(ctx, data.ClusterID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Could not read cluster", err.Error())
		return
	}

	data.ID = types.StringValue(cl.ClusterUUID)
	data.Name = types.StringValue(cl.ClusterName)
	data.ProjectID = types.StringValue(cl.ClusterProjectUUID)
	data.KubernetesVersion = types.StringValue(cl.ClusterVersion)
	data.APIAccess = types.StringValue(cl.ClusterAPIAccess)
	data.Status = types.StringValue(cl.ClusterStatus)
	data.SharedSecurityGroup = types.StringValue(cl.ClusterSharedSecurityGroup)

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

type clusterDataSourceModel struct {
	ClusterID           types.String `tfsdk:"cluster_id"`
	ID                  types.String `tfsdk:"id"`
	Name                types.String `tfsdk:"name"`
	ProjectID           types.String `tfsdk:"project_id"`
	KubernetesVersion   types.String `tfsdk:"kubernetes_version"`
	APIAccess           types.String `tfsdk:"api_access"`
	Status              types.String `tfsdk:"status"`
	SharedSecurityGroup types.String `tfsdk:"shared_security_group"`
}
