package client

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"path"
	"strings"
	"time"
)

// VKEClient is the HTTP client for the VKE REST API; uses a Keystone token as X-Auth-Token.
type VKEClient struct {
	baseURL     *url.URL
	tokenSource *TokenSource
	httpClient  *http.Client
}

// Config holds client options.
// Authentication: same password fields as the OpenStack provider, or application credentials.
type Config struct {
	// Endpoint is the VKE API base URL (e.g. https://api.example.com/api/v1).
	Endpoint string

	// AuthURL is OpenStack Identity (Keystone); same as auth_url in the OpenStack provider.
	AuthURL string

	// Password auth — compatible with OpenStack provider (user_name, password, user_domain_name, tenant_name).
	UserName         string
	Password         string
	UserDomainName   string
	TenantName       string
	ProjectDomainName string

	// Application credential (instead of password)
	ApplicationCredentialID     string
	ApplicationCredentialSecret string

	InsecureSkipVerify bool
}

// New builds a VKEClient from Config.
func New(cfg Config) (*VKEClient, error) {
	if cfg.Endpoint == "" {
		return nil, fmt.Errorf("endpoint is required")
	}
	if cfg.AuthURL == "" {
		return nil, fmt.Errorf("auth_url is required (OpenStack Identity / Keystone)")
	}

	passwordMode := cfg.UserName != "" && cfg.Password != ""
	appCredMode := cfg.ApplicationCredentialID != "" && cfg.ApplicationCredentialSecret != ""

	if passwordMode && appCredMode {
		return nil, fmt.Errorf("password and application_credential cannot be used together")
	}
	if !passwordMode && !appCredMode {
		return nil, fmt.Errorf("either password (user_name, password, user_domain_name, tenant_name) or application_credential credentials are required")
	}
	if passwordMode {
		if cfg.UserDomainName == "" || cfg.TenantName == "" {
			return nil, fmt.Errorf("user_domain_name and tenant_name are required for password authentication")
		}
	}

	base, err := url.Parse(cfg.Endpoint)
	if err != nil {
		return nil, fmt.Errorf("invalid endpoint: %w", err)
	}
	if base.Scheme == "" || base.Host == "" {
		return nil, fmt.Errorf("endpoint must be a full URL (e.g. https://api.example.com/api/v1)")
	}

	transport := &http.Transport{
		TLSClientConfig: &tls.Config{
			InsecureSkipVerify: cfg.InsecureSkipVerify,
		},
	}
	httpClient := &http.Client{
		Transport: transport,
		Timeout:   120 * time.Second,
	}

	ts := &TokenSource{
		IdentityEndpoint:  cfg.AuthURL,
		HTTPClient:        httpClient,
		UsePassword:       passwordMode,
		UserName:          cfg.UserName,
		Password:          cfg.Password,
		UserDomainName:    cfg.UserDomainName,
		TenantName:        cfg.TenantName,
		ProjectDomainName: cfg.ProjectDomainName,
		AppCredID:         cfg.ApplicationCredentialID,
		AppCredSecret:     cfg.ApplicationCredentialSecret,
	}

	return &VKEClient{
		baseURL:     base,
		tokenSource: ts,
		httpClient:  httpClient,
	}, nil
}

func (c *VKEClient) joinPath(segments ...string) string {
	u := *c.baseURL
	trimmed := strings.TrimPrefix(path.Join(segments...), "/")
	u.Path = strings.TrimSuffix(u.Path, "/") + "/" + trimmed
	return u.String()
}

// HTTPSuccess is the VKE API success envelope `{ "data": ... }`.
type HTTPSuccess struct {
	Data json.RawMessage `json:"data"`
}

func (c *VKEClient) authHeader(ctx context.Context) (string, error) {
	return c.tokenSource.Token(ctx)
}

func (c *VKEClient) do(ctx context.Context, method, reqURL string, body io.Reader, contentType string) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, method, reqURL, body)
	if err != nil {
		return nil, err
	}
	token, err := c.authHeader(ctx)
	if err != nil {
		return nil, err
	}
	req.Header.Set("X-Auth-Token", token)
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", "terraform-provider-portvmind (+https://github.com/vmindtech/terraform-provider-portvmind)")
	if contentType != "" {
		req.Header.Set("Content-Type", contentType)
	}
	return c.httpClient.Do(req)
}

func readBody(resp *http.Response) ([]byte, error) {
	b, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	_ = resp.Body.Close()
	return b, nil
}

func (c *VKEClient) doJSON(ctx context.Context, method, reqURL string, body io.Reader) ([]byte, int, error) {
	resp, err := c.do(ctx, method, reqURL, body, "application/json")
	if err != nil {
		return nil, 0, err
	}
	code := resp.StatusCode
	b, err := readBody(resp)
	if err != nil {
		return nil, code, err
	}
	if code < 200 || code >= 300 {
		return b, code, statusErr(code, strings.TrimSpace(string(b)))
	}
	return b, code, nil
}

func decodeDataEnvelope(raw []byte, target interface{}) error {
	var wrap HTTPSuccess
	if err := json.Unmarshal(raw, &wrap); err != nil {
		return err
	}
	return json.Unmarshal(wrap.Data, target)
}

// ClusterModel matches GET /cluster/:id response data.
type ClusterModel struct {
	ClusterUUID                string `json:"cluster_uuid"`
	ClusterName                string `json:"cluster_name"`
	ClusterProjectUUID         string `json:"cluster_project_uuid"`
	ClusterVersion             string `json:"cluster_version"`
	ClusterAPIAccess           string `json:"cluster_api_access"`
	ClusterStatus              string `json:"cluster_status"`
	ClusterSharedSecurityGroup string `json:"cluster_shared_security_group"`
}

// CreateClusterRequest is the POST /cluster body (PortVMind VKE API).
type CreateClusterRequest struct {
	ClusterName              string   `json:"clusterName"`
	ProjectID                string   `json:"projectId"`
	KubernetesVersion        string   `json:"kubernetesVersion"`
	NodeKeyPairName          string   `json:"nodeKeyPairName"`
	ClusterAPIAccess         string   `json:"clusterApiAccess"`
	SubnetIDs                []string `json:"subnetIds"`
	WorkerNodeGroupMinSize   int      `json:"workerNodeGroupMinSize"`
	WorkerNodeGroupMaxSize   int      `json:"workerNodeGroupMaxSize"`
	WorkerInstanceFlavorUUID string   `json:"workerInstanceFlavorUUID"`
	MasterInstanceFlavorUUID string   `json:"masterInstanceFlavorUUID"`
	WorkerDiskSizeGB         int      `json:"workerDiskSizeGB"`
	AllowedCIDRS             []string `json:"allowedCIDRs"`
}

// CreateClusterResponse is the create-cluster API response payload.
type CreateClusterResponse struct {
	ClusterUUID   string `json:"cluster_uuid"`
	ClusterName   string `json:"cluster_name"`
	ClusterStatus string `json:"cluster_status"`
}

// CreateNodeGroupRequest is the POST /cluster/:id/nodegroups body.
type CreateNodeGroupRequest struct {
	NodeGroupName    string   `json:"nodeGroupName"`
	NodeFlavorUUID   string   `json:"nodeFlavorUUID"`
	NodeDiskSize     int      `json:"nodeDiskSize"`
	NodeGroupLabels  []string `json:"nodeGroupLabels"`
	NodeGroupTaints  []string `json:"nodeGroupTaints"`
	NodeGroupMinSize int      `json:"nodeGroupMinSize"`
	NodeGroupMaxSize int      `json:"nodeGroupMaxSize"`
}

// UpdateNodeGroupRequest is the PUT /cluster/:id/nodegroups/:ngid body.
type UpdateNodeGroupRequest struct {
	MinNodes  *uint32 `json:"minNodes,omitempty"`
	MaxNodes  *uint32 `json:"maxNodes,omitempty"`
	Autoscale *bool   `json:"autoscale,omitempty"`
}

// NodeGroup is one node group in GET responses.
type NodeGroup struct {
	ClusterUUID      string `json:"cluster_uuid"`
	NodeGroupUUID    string `json:"node_group_uuid"`
	NodeGroupName    string `json:"node_group_name"`
	NodeGroupMinSize int    `json:"node_group_min_size"`
	NodeGroupMaxSize int    `json:"node_group_max_size"`
	NodeDiskSize     int    `json:"node_disk_size"`
	NodeFlavorUUID   string `json:"node_flavor_uuid"`
	NodeGroupsType   string `json:"node_groups_type"`
	CurrentNodes     int    `json:"current_nodes"`
	NodeGroupsStatus string `json:"node_groups_status"`
}

// CreateNodeGroupResponse is the raw create response (not wrapped in `data`).
type CreateNodeGroupResponse struct {
	ClusterID   string `json:"cluster_id"`
	NodeGroupID string `json:"node_group_id"`
}

// UpdateNodeGroupResponse is the raw update response.
type UpdateNodeGroupResponse struct {
	ClusterID   string `json:"cluster_id"`
	NodeGroupID string `json:"node_group_id"`
	MinSize     int    `json:"min_size"`
	MaxSize     int    `json:"max_size"`
	Status      string `json:"status"`
}

const (
	// ClusterStatusActive VKE internal/service/cluster.go ActiveClusterStatus.
	ClusterStatusActive = "Active"
)

// GetCluster GET /cluster/:cluster_id.
func (c *VKEClient) GetCluster(ctx context.Context, clusterID string) (*ClusterModel, error) {
	reqURL := c.joinPath("cluster", clusterID)
	b, _, err := c.doJSON(ctx, http.MethodGet, reqURL, nil)
	if err != nil {
		return nil, err
	}
	var cluster ClusterModel
	if err := decodeDataEnvelope(b, &cluster); err != nil {
		return nil, fmt.Errorf("decode cluster: %w", err)
	}
	return &cluster, nil
}

// CreateCluster POST /cluster.
func (c *VKEClient) CreateCluster(ctx context.Context, reqBody CreateClusterRequest) (*CreateClusterResponse, error) {
	payload, err := json.Marshal(reqBody)
	if err != nil {
		return nil, err
	}
	reqURL := c.joinPath("cluster")
	b, _, err := c.doJSON(ctx, http.MethodPost, reqURL, bytes.NewReader(payload))
	if err != nil {
		return nil, err
	}
	var out CreateClusterResponse
	if err := decodeDataEnvelope(b, &out); err != nil {
		return nil, fmt.Errorf("decode create cluster response: %w", err)
	}
	return &out, nil
}

// DestroyCluster DELETE /cluster/:cluster_id.
func (c *VKEClient) DestroyCluster(ctx context.Context, clusterID string) error {
	reqURL := c.joinPath("cluster", clusterID)
	_, _, err := c.doJSON(ctx, http.MethodDelete, reqURL, nil)
	return err
}

// GetKubeYAML GET /kubeconfig/:cluster_id — raw kubeconfig YAML text.
func (c *VKEClient) GetKubeYAML(ctx context.Context, clusterID string) (string, error) {
	reqURL := c.joinPath("kubeconfig", clusterID)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, reqURL, nil)
	if err != nil {
		return "", err
	}
	token, err := c.authHeader(ctx)
	if err != nil {
		return "", err
	}
	req.Header.Set("X-Auth-Token", token)
	req.Header.Set("Accept", "application/x-yaml, */*")
	req.Header.Set("User-Agent", "terraform-provider-portvmind (+https://github.com/vmindtech/terraform-provider-portvmind)")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return "", err
	}
	b, err := readBody(resp)
	if err != nil {
		return "", err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", statusErr(resp.StatusCode, strings.TrimSpace(string(b)))
	}
	return string(b), nil
}

// ListNodeGroups GET /cluster/:cluster_id/nodegroups
func (c *VKEClient) ListNodeGroups(ctx context.Context, clusterID string) ([]NodeGroup, error) {
	reqURL := c.joinPath("cluster", clusterID, "nodegroups")
	b, _, err := c.doJSON(ctx, http.MethodGet, reqURL, nil)
	if err != nil {
		return nil, err
	}
	var groups []NodeGroup
	if err := json.Unmarshal(b, &groups); err != nil {
		return nil, fmt.Errorf("decode node group list: %w", err)
	}
	return groups, nil
}

// GetNodeGroup GET /cluster/:cluster_id/nodegroups/:nodegroup_id
func (c *VKEClient) GetNodeGroup(ctx context.Context, clusterID, nodeGroupID string) (*NodeGroup, error) {
	reqURL := c.joinPath("cluster", clusterID, "nodegroups", nodeGroupID)
	b, _, err := c.doJSON(ctx, http.MethodGet, reqURL, nil)
	if err != nil {
		return nil, err
	}
	var groups []NodeGroup
	if err := json.Unmarshal(b, &groups); err != nil {
		return nil, err
	}
	if len(groups) == 0 {
		return nil, ErrNotFound{Resource: "node_group"}
	}
	return &groups[0], nil
}

// CreateNodeGroup POST /cluster/:cluster_id/nodegroups
func (c *VKEClient) CreateNodeGroup(ctx context.Context, clusterID string, reqBody CreateNodeGroupRequest) (*CreateNodeGroupResponse, error) {
	payload, err := json.Marshal(reqBody)
	if err != nil {
		return nil, err
	}
	reqURL := c.joinPath("cluster", clusterID, "nodegroups")
	b, _, err := c.doJSON(ctx, http.MethodPost, reqURL, bytes.NewReader(payload))
	if err != nil {
		return nil, err
	}
	var out CreateNodeGroupResponse
	if err := json.Unmarshal(b, &out); err != nil {
		return nil, fmt.Errorf("decode create node group response: %w", err)
	}
	return &out, nil
}

// UpdateNodeGroup PUT /cluster/:cluster_id/nodegroups/:nodegroup_id
func (c *VKEClient) UpdateNodeGroup(ctx context.Context, clusterID, nodeGroupID string, reqBody UpdateNodeGroupRequest) (*UpdateNodeGroupResponse, error) {
	payload, err := json.Marshal(reqBody)
	if err != nil {
		return nil, err
	}
	reqURL := c.joinPath("cluster", clusterID, "nodegroups", nodeGroupID)
	b, _, err := c.doJSON(ctx, http.MethodPut, reqURL, bytes.NewReader(payload))
	if err != nil {
		return nil, err
	}
	var out UpdateNodeGroupResponse
	if err := json.Unmarshal(b, &out); err != nil {
		return nil, fmt.Errorf("decode update node group response: %w", err)
	}
	return &out, nil
}

// DeleteNodeGroup DELETE /cluster/:cluster_id/nodegroups/:nodegroup_id
func (c *VKEClient) DeleteNodeGroup(ctx context.Context, clusterID, nodeGroupID string) error {
	reqURL := c.joinPath("cluster", clusterID, "nodegroups", nodeGroupID)
	resp, err := c.do(ctx, http.MethodDelete, reqURL, nil, "")
	if err != nil {
		return err
	}
	b, err := readBody(resp)
	if err != nil {
		return err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return statusErr(resp.StatusCode, strings.TrimSpace(string(b)))
	}
	return nil
}
