// Package asgardeo provides a pure-Go client for the Asgardeo Management API.
// It is independent of any Terraform SDK and can be used as a standalone library.
package asgardeo

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"
)

const (
	defaultHTTPTimeout = 30 * time.Second
	tokenExpiryBuffer  = 30 * time.Second
)

// Client is the main Asgardeo API client. It is safe for concurrent use.
type Client struct {
	httpClient   *http.Client
	baseURL      string // e.g. https://api.asgardeo.io/t/myorg/api/server/v1
	tokenURL     string // e.g. https://api.asgardeo.io/t/myorg/oauth2/token
	clientID     string
	clientSecret string

	mu          sync.Mutex
	accessToken string
	tokenExpiry time.Time
}

// tokenResponse is the OAuth2 token endpoint response.
type tokenResponse struct {
	AccessToken string `json:"access_token"`
	TokenType   string `json:"token_type"`
	ExpiresIn   int    `json:"expires_in"`
	Scope       string `json:"scope"`
}

// NewClient creates a ready-to-use Asgardeo API client.
// The first API call will trigger token acquisition.
func NewClient(orgName, clientID, clientSecret string) *Client {
	return &Client{
		httpClient:   &http.Client{Timeout: defaultHTTPTimeout},
		baseURL:      fmt.Sprintf("https://api.asgardeo.io/t/%s/api/server/v1", orgName),
		tokenURL:     fmt.Sprintf("https://api.asgardeo.io/t/%s/oauth2/token", orgName),
		clientID:     clientID,
		clientSecret: clientSecret,
	}
}

// doRequest executes an authenticated HTTP request against the Asgardeo API.
// path must begin with "/" and is relative to the base URL.
func (c *Client) doRequest(ctx context.Context, method, path string, body io.Reader) (*http.Response, error) {
	return c.do(ctx, method, c.baseURL+path, body)
}

// doOrgRequest executes an authenticated HTTP request against the
// organization-scoped path (/o/api/server/v1/...). This path gates a few
// endpoints (notably OIDC clientSecret retrieval) behind the
// "internal_org_application_mgt_client_secret_*" scopes — see fetchToken.
func (c *Client) doOrgRequest(ctx context.Context, method, path string, body io.Reader) (*http.Response, error) {
	orgBase := strings.Replace(c.baseURL, "/api/server/v1", "/o/api/server/v1", 1)
	return c.do(ctx, method, orgBase+path, body)
}

// do is the shared HTTP request helper used by doRequest and doOrgRequest.
func (c *Client) do(ctx context.Context, method, reqURL string, body io.Reader) (*http.Response, error) {
	token, err := c.getToken(ctx)
	if err != nil {
		return nil, fmt.Errorf("obtain access token: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, method, reqURL, body)
	if err != nil {
		return nil, fmt.Errorf("build request %s %s: %w", method, reqURL, err)
	}

	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	return c.httpClient.Do(req)
}

// getToken returns a valid access token, refreshing proactively when near expiry.
func (c *Client) getToken(ctx context.Context) (string, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.accessToken != "" && time.Now().Add(tokenExpiryBuffer).Before(c.tokenExpiry) {
		return c.accessToken, nil
	}
	return c.fetchToken(ctx)
}

// fetchToken calls the Asgardeo token endpoint using the client_credentials grant.
// Caller must hold c.mu.
//
// internal_org_application_mgt_client_secret_view is required so GET on
// /o/api/server/v1/applications/{id}/inbound-protocols/oidc returns the
// generated clientSecret in its response body. Without that scope, the
// endpoint omits the secret and the resource's client_secret attribute
// ends up unknown after apply (Terraform error: "Provider returned invalid
// result object after apply"). The corresponding API authorization must
// be granted to the M2M application in the Asgardeo Console:
//
//	Resource: Application Management API
//	Scope:    "View application client secret"
func (c *Client) fetchToken(ctx context.Context) (string, error) {
	scopes := []string{
		"internal_application_mgt_create",
		"internal_application_mgt_view",
		"internal_application_mgt_update",
		"internal_application_mgt_delete",
		"internal_org_application_mgt_client_secret_view",
		// internal_api_resource_view lets the authorized-api resource resolve an
		// API-resource identifier (e.g. "/scim2/Users") to its UUID via
		// GET /api-resources before authorizing it on an application.
		"internal_api_resource_view",
		// PATCH/DELETE on /applications/{id}/authorized-apis additionally
		// require these per API-resource type: _internal_ for management/
		// organization APIs (e.g. /scim2/Users), _business_ for user-defined
		// business APIs. Without them Asgardeo answers APP-60520 Forbidden on
		// scope updates and de-authorizations even when
		// internal_application_mgt_update was granted. Asgardeo intersects the
		// requested scopes with the app's console authorizations and silently
		// drops ungranted ones, so requesting them unconditionally is safe.
		"internal_application_internal_api_update",
		"internal_application_business_api_update",
	}

	form := url.Values{}
	form.Set("grant_type", "client_credentials")
	form.Set("scope", strings.Join(scopes, " "))

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.tokenURL,
		strings.NewReader(form.Encode()))
	if err != nil {
		return "", fmt.Errorf("build token request: %w", err)
	}
	req.SetBasicAuth(c.clientID, c.clientSecret)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("token request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		raw, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("token endpoint HTTP %d: %s", resp.StatusCode, string(raw))
	}

	var tr tokenResponse
	if err := json.NewDecoder(resp.Body).Decode(&tr); err != nil {
		return "", fmt.Errorf("decode token response: %w", err)
	}

	c.accessToken = tr.AccessToken
	c.tokenExpiry = time.Now().Add(time.Duration(tr.ExpiresIn) * time.Second)
	return c.accessToken, nil
}
