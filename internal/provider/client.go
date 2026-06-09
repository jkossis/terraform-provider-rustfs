// Copyright IBM Corp. 2021, 2025
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"bytes"
	"context"
	"crypto/sha256"
	"crypto/tls"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
)

const rustfsAdminV3Prefix = "/rustfs/admin/v3"

type siteReplicationAdminClient interface {
	SiteReplicationAdd(context.Context, []peerSite, srAddOptions) (replicateAddStatus, error)
	SiteReplicationEdit(context.Context, peerInfo, srEditOptions) (replicateEditStatus, error)
	SiteReplicationInfo(context.Context) (siteReplicationInfo, error)
	SiteReplicationRemove(context.Context, srRemoveReq) (replicateRemoveStatus, error)
	SRMetaInfo(context.Context, srStatusOptions) (srInfo, error)
	SRStatusInfo(context.Context, srStatusOptions) (srStatusInfo, error)
}

type rustfsClient struct {
	httpClient *http.Client
	endpoint   string
	secure     bool
	accessKey  string
	secretKey  string
}

func newRustFSClient(endpoint, accessKey, secretKey string, insecureSkipTLSVerify bool) (*rustfsClient, error) {
	normalizedEndpoint, secure, err := normalizeEndpoint(endpoint)
	if err != nil {
		return nil, err
	}

	defaultTransport, ok := http.DefaultTransport.(*http.Transport)
	if !ok {
		return nil, fmt.Errorf("default HTTP transport has unexpected type %T", http.DefaultTransport)
	}

	transport := defaultTransport.Clone()
	if secure && insecureSkipTLSVerify {
		transport.TLSClientConfig = &tls.Config{InsecureSkipVerify: true}
	}

	return &rustfsClient{
		httpClient: &http.Client{Transport: transport},
		endpoint:   normalizedEndpoint,
		secure:     secure,
		accessKey:  accessKey,
		secretKey:  secretKey,
	}, nil
}

func normalizeEndpoint(endpoint string) (string, bool, error) {
	endpoint = strings.TrimSpace(endpoint)
	if endpoint == "" {
		return "", false, fmt.Errorf("endpoint must not be empty")
	}

	if !strings.Contains(endpoint, "://") {
		if strings.Contains(endpoint, "/") {
			return "", false, fmt.Errorf("endpoint without a scheme must be a host[:port] value")
		}

		return endpoint, true, nil
	}

	parsed, err := url.Parse(endpoint)
	if err != nil {
		return "", false, err
	}

	if parsed.Host == "" {
		return "", false, fmt.Errorf("endpoint must include a host")
	}

	if parsed.Path != "" && parsed.Path != "/" {
		return "", false, fmt.Errorf("endpoint must not include a path")
	}

	if parsed.RawQuery != "" || parsed.Fragment != "" {
		return "", false, fmt.Errorf("endpoint must not include query parameters or a fragment")
	}

	switch parsed.Scheme {
	case "http":
		return parsed.Host, false, nil
	case "https":
		return parsed.Host, true, nil
	default:
		return "", false, fmt.Errorf("endpoint scheme must be http or https")
	}
}

func (c *rustfsClient) SiteReplicationAdd(ctx context.Context, sites []peerSite, opts srAddOptions) (replicateAddStatus, error) {
	var result replicateAddStatus
	body, err := json.Marshal(sites)
	if err != nil {
		return result, err
	}

	err = c.executeSiteReplicationRequest(ctx, http.MethodPut, "/add", siteReplicationAddQuery(opts), body, &result)
	return result, err
}

func (c *rustfsClient) SiteReplicationEdit(ctx context.Context, site peerInfo, opts srEditOptions) (replicateEditStatus, error) {
	var result replicateEditStatus
	body, err := json.Marshal(site)
	if err != nil {
		return result, err
	}

	err = c.executeSiteReplicationRequest(ctx, http.MethodPut, "/edit", siteReplicationEditQuery(opts), body, &result)
	return result, err
}

func (c *rustfsClient) SiteReplicationInfo(ctx context.Context) (siteReplicationInfo, error) {
	var result siteReplicationInfo
	err := c.executeSiteReplicationRequest(ctx, http.MethodGet, "/info", siteReplicationBaseQuery(), nil, &result)
	return result, err
}

func (c *rustfsClient) SiteReplicationRemove(ctx context.Context, removeReq srRemoveReq) (replicateRemoveStatus, error) {
	var result replicateRemoveStatus
	body, err := json.Marshal(removeReq)
	if err != nil {
		return result, err
	}

	err = c.executeSiteReplicationRequest(ctx, http.MethodPut, "/remove", siteReplicationBaseQuery(), body, &result)
	return result, err
}

func (c *rustfsClient) SRMetaInfo(ctx context.Context, opts srStatusOptions) (srInfo, error) {
	var result srInfo
	err := c.executeSiteReplicationRequest(ctx, http.MethodGet, "/metainfo", siteReplicationStatusQuery(opts), nil, &result)
	return result, err
}

func (c *rustfsClient) SRStatusInfo(ctx context.Context, opts srStatusOptions) (srStatusInfo, error) {
	var result srStatusInfo
	err := c.executeSiteReplicationRequest(ctx, http.MethodGet, "/status", siteReplicationStatusQuery(opts), nil, &result)
	return result, err
}

func (c *rustfsClient) executeSiteReplicationRequest(ctx context.Context, method, suffix string, query url.Values, body []byte, result any) error {
	req, err := c.newAdminRequest(ctx, method, rustfsAdminV3Prefix+"/site-replication"+suffix, query, body)
	if err != nil {
		return err
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return rustfsHTTPError(resp)
	}

	if result == nil {
		return nil
	}

	responseBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}
	if setter, ok := result.(rawJSONSetter); ok {
		setter.setRawJSON(responseBody)
	}

	return json.Unmarshal(responseBody, result)
}

func (c *rustfsClient) newAdminRequest(ctx context.Context, method, path string, query url.Values, body []byte) (*http.Request, error) {
	scheme := "https"
	if !c.secure {
		scheme = "http"
	}

	target := url.URL{
		Scheme:   scheme,
		Host:     c.endpoint,
		Path:     path,
		RawQuery: query.Encode(),
	}

	req, err := http.NewRequestWithContext(ctx, method, target.String(), bytes.NewReader(body))
	if err != nil {
		return nil, err
	}

	if len(body) > 0 {
		req.Body = io.NopCloser(bytes.NewReader(body))
		req.ContentLength = int64(len(body))
	}

	sum := sha256.Sum256(body)
	req.Header.Set("X-Amz-Content-Sha256", hex.EncodeToString(sum[:]))
	req.Header.Set("User-Agent", "terraform-provider-rustfs")
	if len(body) > 0 {
		req.Header.Set("Content-Type", "application/json")
	}

	return signV4(req, c.accessKey, c.secretKey, ""), nil
}

func rustfsHTTPError(resp *http.Response) error {
	body, err := io.ReadAll(io.LimitReader(resp.Body, 100<<10))
	if err != nil {
		return fmt.Errorf("%s: failed to read response body: %w", resp.Status, err)
	}

	detail := strings.TrimSpace(string(body))
	if detail == "" {
		return fmt.Errorf("%s", resp.Status)
	}

	return fmt.Errorf("%s: %s", resp.Status, detail)
}

func siteReplicationBaseQuery() url.Values {
	query := make(url.Values)
	query.Set("api-version", siteReplicationAPIVersion)
	return query
}

func siteReplicationAddQuery(opts srAddOptions) url.Values {
	query := siteReplicationBaseQuery()
	if opts.ReplicateILMExpiry {
		query.Set("replicateILMExpiry", "true")
	}
	if opts.Force {
		query.Set("force", "true")
	}
	return query
}

func siteReplicationEditQuery(opts srEditOptions) url.Values {
	query := siteReplicationBaseQuery()
	if opts.EnableILMExpiryReplication {
		query.Set("enableILMExpiryReplication", "true")
	}
	if opts.DisableILMExpiryReplication {
		query.Set("disableILMExpiryReplication", "true")
	}
	return query
}

func siteReplicationStatusQuery(opts srStatusOptions) url.Values {
	query := siteReplicationBaseQuery()
	setTrueQueryFlag(query, "buckets", opts.Buckets)
	setTrueQueryFlag(query, "policies", opts.Policies)
	setTrueQueryFlag(query, "users", opts.Users)
	setTrueQueryFlag(query, "groups", opts.Groups)
	setTrueQueryFlag(query, "metrics", opts.Metrics)
	setTrueQueryFlag(query, "peer-state", opts.PeerState)
	setTrueQueryFlag(query, "ilm-expiry-rules", opts.ILMExpiryRules)
	setTrueQueryFlag(query, "showDeleted", opts.ShowDeleted)

	if entity := siteReplicationEntityName(opts.Entity); entity != "" {
		query.Set("entity", entity)
	}
	if opts.EntityValue != "" {
		query.Set("entityvalue", opts.EntityValue)
	}

	return query
}

func setTrueQueryFlag(query url.Values, name string, value bool) {
	if value {
		query.Set(name, "true")
	}
}

func siteReplicationEntityName(entity srEntityType) string {
	switch entity {
	case srBucketEntity:
		return "bucket"
	case srPolicyEntity:
		return "policy"
	case srUserEntity:
		return "user"
	case srGroupEntity:
		return "group"
	case srILMExpiryRuleEntity:
		return "ilm-expiry-rule"
	default:
		return ""
	}
}
