package asgardeo

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
)

// ─── CRUD ─────────────────────────────────────────────────────────────────────

// CreateApplication creates a new application.
// Returns the full ApplicationResponse after following the Location header.
func (c *Client) CreateApplication(ctx context.Context, req ApplicationCreateRequest) (*ApplicationResponse, error) {
	body, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("marshal create request: %w", err)
	}

	resp, err := c.doRequest(ctx, http.MethodPost, "/applications", bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		return nil, parseAPIError(resp)
	}

	appID, err := extractIDFromLocation(resp.Header.Get("Location"))
	if err != nil {
		return nil, fmt.Errorf("parse Location header: %w", err)
	}

	return c.GetApplication(ctx, appID)
}

// GetApplication retrieves a single application by ID.
// Returns nil, nil when the application does not exist (404).
func (c *Client) GetApplication(ctx context.Context, id string) (*ApplicationResponse, error) {
	resp, err := c.doRequest(ctx, http.MethodGet, "/applications/"+id, nil)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return nil, nil
	}
	if resp.StatusCode != http.StatusOK {
		return nil, parseAPIError(resp)
	}

	var app ApplicationResponse
	if err := json.NewDecoder(resp.Body).Decode(&app); err != nil {
		return nil, fmt.Errorf("decode application: %w", err)
	}
	return &app, nil
}

// PatchApplication partially updates an application's base properties.
func (c *Client) PatchApplication(ctx context.Context, id string, req ApplicationPatchRequest) error {
	body, err := json.Marshal(req)
	if err != nil {
		return fmt.Errorf("marshal patch request: %w", err)
	}

	resp, err := c.doRequest(ctx, http.MethodPatch, "/applications/"+id, bytes.NewReader(body))
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusNoContent {
		return parseAPIError(resp)
	}
	return nil
}

// DeleteApplication deletes an application by ID.
func (c *Client) DeleteApplication(ctx context.Context, id string) error {
	resp, err := c.doRequest(ctx, http.MethodDelete, "/applications/"+id, nil)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return nil
	}
	if resp.StatusCode != http.StatusNoContent && resp.StatusCode != http.StatusOK {
		return parseAPIError(resp)
	}
	return nil
}

// ListApplications returns all applications. Simple single-page fetch (limit 100).
func (c *Client) ListApplications(ctx context.Context) ([]ApplicationResponse, error) {
	resp, err := c.doRequest(ctx, http.MethodGet, "/applications?limit=100", nil)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, parseAPIError(resp)
	}

	var list ApplicationListResponse
	if err := json.NewDecoder(resp.Body).Decode(&list); err != nil {
		return nil, fmt.Errorf("decode application list: %w", err)
	}
	return list.Applications, nil
}

// GetApplicationByName returns the first application matching the given name, or nil.
func (c *Client) GetApplicationByName(ctx context.Context, name string) (*ApplicationResponse, error) {
	apps, err := c.ListApplications(ctx)
	if err != nil {
		return nil, err
	}
	for i := range apps {
		if apps[i].Name == name {
			return &apps[i], nil
		}
	}
	return nil, nil
}

// ─── OIDC Protocol ────────────────────────────────────────────────────────────

// GetOIDCConfig retrieves the OIDC inbound protocol configuration.
// Returns nil, nil when no OIDC config exists (404).
func (c *Client) GetOIDCConfig(ctx context.Context, appID string) (*OIDCConfiguration, error) {
	path := fmt.Sprintf("/applications/%s/inbound-protocols/oidc", appID)
	resp, err := c.doRequest(ctx, http.MethodGet, path, nil)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return nil, nil
	}
	if resp.StatusCode != http.StatusOK {
		return nil, parseAPIError(resp)
	}

	var cfg OIDCConfiguration
	if err := json.NewDecoder(resp.Body).Decode(&cfg); err != nil {
		return nil, fmt.Errorf("decode OIDC config: %w", err)
	}
	return &cfg, nil
}

// PutOIDCConfig replaces the OIDC inbound protocol configuration.
func (c *Client) PutOIDCConfig(ctx context.Context, appID string, cfg OIDCConfiguration) (*OIDCConfiguration, error) {
	path := fmt.Sprintf("/applications/%s/inbound-protocols/oidc", appID)
	body, err := json.Marshal(cfg)
	if err != nil {
		return nil, fmt.Errorf("marshal OIDC config: %w", err)
	}

	resp, err := c.doRequest(ctx, http.MethodPut, path, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated && resp.StatusCode != http.StatusNoContent {
		return nil, parseAPIError(resp)
	}

	// Asgardeo sometimes returns 200 with an empty body after a successful PUT.
	// Fall back to a GET in that case so callers always receive the current config.
	var result OIDCConfiguration
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		if err == io.EOF {
			return c.GetOIDCConfig(ctx, appID)
		}
		return nil, fmt.Errorf("decode OIDC config response: %w", err)
	}
	return &result, nil
}

// ─── SAML Protocol ────────────────────────────────────────────────────────────

// GetSAMLConfig retrieves the SAML inbound protocol configuration.
func (c *Client) GetSAMLConfig(ctx context.Context, appID string) (*SAMLConfiguration, error) {
	path := fmt.Sprintf("/applications/%s/inbound-protocols/saml", appID)
	resp, err := c.doRequest(ctx, http.MethodGet, path, nil)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return nil, nil
	}
	if resp.StatusCode != http.StatusOK {
		return nil, parseAPIError(resp)
	}

	var cfg SAMLConfiguration
	if err := json.NewDecoder(resp.Body).Decode(&cfg); err != nil {
		return nil, fmt.Errorf("decode SAML config: %w", err)
	}
	return &cfg, nil
}

// PutSAMLConfig replaces the SAML inbound protocol configuration.
func (c *Client) PutSAMLConfig(ctx context.Context, appID string, cfg SAMLConfiguration) (*SAMLConfiguration, error) {
	path := fmt.Sprintf("/applications/%s/inbound-protocols/saml", appID)
	body, err := json.Marshal(cfg)
	if err != nil {
		return nil, fmt.Errorf("marshal SAML config: %w", err)
	}

	resp, err := c.doRequest(ctx, http.MethodPut, path, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		return nil, parseAPIError(resp)
	}

	var result SAMLConfiguration
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("decode SAML config response: %w", err)
	}
	return &result, nil
}

// ─── Internal helpers ─────────────────────────────────────────────────────────

func parseAPIError(resp *http.Response) error {
	raw, _ := io.ReadAll(resp.Body)
	var apiErr APIError
	if err := json.Unmarshal(raw, &apiErr); err != nil || apiErr.Code == "" {
		return fmt.Errorf("HTTP %d: %s", resp.StatusCode, string(raw))
	}
	return &apiErr
}

// extractIDFromLocation parses the application ID from a Location header URL:
// https://api.asgardeo.io/t/org/api/server/v1/applications/{id}
func extractIDFromLocation(location string) (string, error) {
	if location == "" {
		return "", fmt.Errorf("empty Location header")
	}
	u, err := url.Parse(location)
	if err != nil {
		return "", err
	}
	parts := strings.Split(u.Path, "/")
	for i, p := range parts {
		if p == "applications" && i+1 < len(parts) && parts[i+1] != "" {
			return parts[i+1], nil
		}
	}
	return "", fmt.Errorf("application ID not found in path %q", u.Path)
}
