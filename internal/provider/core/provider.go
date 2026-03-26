package core

import (
	"context"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/provider"
	"github.com/hashicorp/terraform-plugin-framework/provider/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/vmindtech/terraform-provider-portvmind/internal/client"
	"github.com/vmindtech/terraform-provider-portvmind/internal/provider/vke"
)

var (
	_ provider.Provider = &portvmindProvider{}
)

type portvmindProvider struct{}

func New() provider.Provider {
	return &portvmindProvider{}
}

func (p *portvmindProvider) Metadata(_ context.Context, _ provider.MetadataRequest, resp *provider.MetadataResponse) {
	resp.TypeName = "portvmind"
}

func (p *portvmindProvider) Schema(_ context.Context, _ provider.SchemaRequest, resp *provider.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "PortVMind cloud provider. Current resources use VKE APIs and Keystone token authentication.",
		Attributes: map[string]schema.Attribute{
			"endpoint": schema.StringAttribute{
				Description: "VKE API base URL (e.g. https://example.com/api/v1).",
				Required:    true,
			},
			"auth_url": schema.StringAttribute{
				Description: "OpenStack Identity (Keystone) URL.",
				Required:    true,
			},
			"user_name": schema.StringAttribute{
				Description: "Keystone user name (password authentication).",
				Optional:    true,
			},
			"password": schema.StringAttribute{
				Description: "Keystone password.",
				Optional:    true,
				Sensitive:   true,
			},
			"user_domain_name": schema.StringAttribute{
				Description: "User domain name (e.g. Default).",
				Optional:    true,
			},
			"tenant_name": schema.StringAttribute{
				Description: "Project (tenant) name.",
				Optional:    true,
			},
			"project_domain_name": schema.StringAttribute{
				Description: "Project domain for Keystone scope. If empty, Default is used.",
				Optional:    true,
			},
			"application_credential_id": schema.StringAttribute{
				Description: "Application credential ID (alternative to password auth).",
				Optional:    true,
			},
			"application_credential_secret": schema.StringAttribute{
				Description: "Application credential secret.",
				Optional:    true,
				Sensitive:   true,
			},
			"insecure": schema.BoolAttribute{
				Description: "Skip TLS certificate verification (testing only).",
				Optional:    true,
			},
		},
	}
}

func (p *portvmindProvider) Configure(ctx context.Context, req provider.ConfigureRequest, resp *provider.ConfigureResponse) {
	var config providerModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &config)...)
	if resp.Diagnostics.HasError() {
		return
	}

	insecure := false
	if !config.Insecure.IsNull() && !config.Insecure.IsUnknown() {
		insecure = config.Insecure.ValueBool()
	}

	passwordMode := !config.UserName.IsNull() && config.UserName.ValueString() != ""
	appCredMode := !config.ApplicationCredentialID.IsNull() && config.ApplicationCredentialID.ValueString() != ""

	if passwordMode && appCredMode {
		resp.Diagnostics.AddError(
			"Invalid provider configuration",
			"password authentication and application_credential cannot be used together.",
		)
		return
	}
	if !passwordMode && !appCredMode {
		resp.Diagnostics.AddError(
			"Invalid provider configuration",
			"Either password (user_name, password, user_domain_name, tenant_name) or application_credential_id + application_credential_secret is required.",
		)
		return
	}

	cfg := client.Config{
		Endpoint:                    config.Endpoint.ValueString(),
		AuthURL:                     config.AuthURL.ValueString(),
		InsecureSkipVerify:          insecure,
		UserName:                    config.UserName.ValueString(),
		Password:                    config.Password.ValueString(),
		UserDomainName:              config.UserDomainName.ValueString(),
		TenantName:                  config.TenantName.ValueString(),
		ProjectDomainName:           config.ProjectDomainName.ValueString(),
		ApplicationCredentialID:     config.ApplicationCredentialID.ValueString(),
		ApplicationCredentialSecret: config.ApplicationCredentialSecret.ValueString(),
	}

	if passwordMode {
		if cfg.Password == "" || cfg.UserDomainName == "" || cfg.TenantName == "" {
			resp.Diagnostics.AddError(
				"Invalid provider configuration",
				"For password authentication, user_name, password, user_domain_name, and tenant_name are required.",
			)
			return
		}
	}

	if appCredMode {
		if cfg.ApplicationCredentialSecret == "" {
			resp.Diagnostics.AddError(
				"Invalid provider configuration",
				"application_credential_secret is required.",
			)
			return
		}
	}

	c, err := client.New(cfg)
	if err != nil {
		resp.Diagnostics.AddError("Could not create API client", err.Error())
		return
	}

	resp.DataSourceData = c
	resp.ResourceData = c
}

func (p *portvmindProvider) Resources(_ context.Context) []func() resource.Resource {
	return []func() resource.Resource{
		vke.NewClusterResource,
		vke.NewNodeGroupResource,
	}
}

func (p *portvmindProvider) DataSources(_ context.Context) []func() datasource.DataSource {
	return []func() datasource.DataSource{
		vke.NewClusterDataSource,
	}
}

type providerModel struct {
	Endpoint                    types.String `tfsdk:"endpoint"`
	AuthURL                     types.String `tfsdk:"auth_url"`
	UserName                    types.String `tfsdk:"user_name"`
	Password                    types.String `tfsdk:"password"`
	UserDomainName              types.String `tfsdk:"user_domain_name"`
	TenantName                  types.String `tfsdk:"tenant_name"`
	ProjectDomainName           types.String `tfsdk:"project_domain_name"`
	ApplicationCredentialID     types.String `tfsdk:"application_credential_id"`
	ApplicationCredentialSecret types.String `tfsdk:"application_credential_secret"`
	Insecure                    types.Bool   `tfsdk:"insecure"`
}
