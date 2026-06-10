package asgardeo

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
)

// ─── Sentinel errors ──────────────────────────────────────────────────────────

// ErrAPIResourceNotFound is returned by GetAPIResourceByIdentifier when no API
// resource matches the requested identifier. Callers can use errors.Is to map
// this to a clear "not found" diagnostic.
var ErrAPIResourceNotFound = errors.New("api resource not found")

// ErrAPIResourceAmbiguous is returned by GetAPIResourceByIdentifier when more
// than one API resource exact-matches the requested identifier.
var ErrAPIResourceAmbiguous = errors.New("multiple api resources matched identifier")

// ─── API Resource lookup ──────────────────────────────────────────────────────

// GetAPIResourceByIdentifier resolves an API-resource identifier (e.g.
// "/scim2/Users") to its full APIResource — most importantly its UUID, which
// the authorize call requires.
//
// It queries GET /api-resources with a server-side filter and then performs an
// exact identifier match client-side (the filter is a contains/prefix match on
// some Asgardeo versions, so it can return near-matches). It returns
// ErrAPIResourceNotFound when nothing matches and ErrAPIResourceAmbiguous when
// more than one entry exact-matches — both wrapped so callers can errors.Is
// them while still seeing the identifier in the message.
func (c *Client) GetAPIResourceByIdentifier(ctx context.Context, identifier string) (*APIResource, error) {
	filter := fmt.Sprintf("identifier eq %s", identifier)
	path := "/api-resources?filter=" + url.QueryEscape(filter)

	resp, err := c.doRequest(ctx, http.MethodGet, path, nil)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, parseAPIError(resp)
	}

	var list APIResourceListResponse
	if err := json.NewDecoder(resp.Body).Decode(&list); err != nil {
		return nil, fmt.Errorf("decode api-resources list: %w", err)
	}

	// Asgardeo has used both "apiResources" and "items" as the list key across
	// versions; merge whichever was populated.
	candidates := list.APIResources
	if len(candidates) == 0 {
		candidates = list.Items
	}

	var matches []APIResource
	for i := range candidates {
		if candidates[i].Identifier == identifier {
			matches = append(matches, candidates[i])
		}
	}

	switch len(matches) {
	case 0:
		return nil, fmt.Errorf("%w: %q", ErrAPIResourceNotFound, identifier)
	case 1:
		m := matches[0]
		return &m, nil
	default:
		ids := make([]string, 0, len(matches))
		for _, m := range matches {
			ids = append(ids, m.ID)
		}
		return nil, fmt.Errorf("%w: %q matched %d resources (%v)",
			ErrAPIResourceAmbiguous, identifier, len(matches), ids)
	}
}

// ─── Authorized API CRUD ──────────────────────────────────────────────────────

// GetAuthorizedAPIs returns all API resources authorized on the application.
func (c *Client) GetAuthorizedAPIs(ctx context.Context, appID string) ([]AuthorizedAPI, error) {
	path := fmt.Sprintf("/applications/%s/authorized-apis", appID)
	resp, err := c.doRequest(ctx, http.MethodGet, path, nil)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, parseAPIError(resp)
	}

	var apis []AuthorizedAPI
	if err := json.NewDecoder(resp.Body).Decode(&apis); err != nil {
		return nil, fmt.Errorf("decode authorized-apis: %w", err)
	}
	return apis, nil
}

// GetAuthorizedAPI returns the single authorized API matching apiResourceID, or
// nil when it is not authorized on the application.
func (c *Client) GetAuthorizedAPI(ctx context.Context, appID, apiResourceID string) (*AuthorizedAPI, error) {
	apis, err := c.GetAuthorizedAPIs(ctx, appID)
	if err != nil {
		return nil, err
	}
	for i := range apis {
		if apis[i].ID == apiResourceID {
			return &apis[i], nil
		}
	}
	return nil, nil
}

// AuthorizeAPI authorizes an API resource on an application via
// POST /applications/{appId}/authorized-apis. apiResourceID is the API-resource
// UUID (NOT the identifier string).
func (c *Client) AuthorizeAPI(ctx context.Context, appID, apiResourceID, policyIdentifier string, scopes []string) error {
	reqBody := AuthorizeAPIRequest{
		ID:               apiResourceID,
		PolicyIdentifier: policyIdentifier,
		Scopes:           scopes,
	}
	body, err := json.Marshal(reqBody)
	if err != nil {
		return fmt.Errorf("marshal authorize-api request: %w", err)
	}

	path := fmt.Sprintf("/applications/%s/authorized-apis", appID)
	resp, err := c.doRequest(ctx, http.MethodPost, path, bytes.NewReader(body))
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated && resp.StatusCode != http.StatusNoContent {
		return parseAPIError(resp)
	}
	return nil
}

// PatchAuthorizedAPI adds and/or removes granted scopes on an already-authorized
// API resource via PATCH /applications/{appId}/authorized-apis/{apiResourceId}.
func (c *Client) PatchAuthorizedAPI(ctx context.Context, appID, apiResourceID string, added, removed []string) error {
	reqBody := PatchAuthorizedAPIRequest{
		AddedScopes:   added,
		RemovedScopes: removed,
	}
	body, err := json.Marshal(reqBody)
	if err != nil {
		return fmt.Errorf("marshal patch authorized-api request: %w", err)
	}

	path := fmt.Sprintf("/applications/%s/authorized-apis/%s", appID, apiResourceID)
	resp, err := c.doRequest(ctx, http.MethodPatch, path, bytes.NewReader(body))
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusNoContent {
		return parseAPIError(resp)
	}
	return nil
}

// DeleteAuthorizedAPI removes an API-resource authorization from an application
// via DELETE /applications/{appId}/authorized-apis/{apiResourceId}. A 404 is
// treated as success (already gone).
func (c *Client) DeleteAuthorizedAPI(ctx context.Context, appID, apiResourceID string) error {
	path := fmt.Sprintf("/applications/%s/authorized-apis/%s", appID, apiResourceID)
	resp, err := c.doRequest(ctx, http.MethodDelete, path, nil)
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
