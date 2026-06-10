package asgardeo

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"
	"time"
)

// newTestClient builds a Client pointed at the given test server. The server is
// expected to serve both the token endpoint (/oauth2/token) and the API
// endpoints under /api/server/v1. Because the test lives in package asgardeo it
// can populate the unexported fields directly.
func newTestClient(serverURL string) *Client {
	return &Client{
		httpClient:   &http.Client{Timeout: 5 * time.Second},
		baseURL:      serverURL + "/api/server/v1",
		tokenURL:     serverURL + "/oauth2/token",
		clientID:     "test-client",
		clientSecret: "test-secret",
	}
}

// writeToken writes a minimal OAuth2 token response.
func writeToken(w http.ResponseWriter) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(tokenResponse{
		AccessToken: "test-token",
		TokenType:   "Bearer",
		ExpiresIn:   3600,
	})
}

func TestGetAPIResourceByIdentifier_ExactMatch(t *testing.T) {
	var capturedQuery string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/oauth2/token":
			writeToken(w)
		case r.URL.Path == "/api/server/v1/api-resources":
			capturedQuery = r.URL.RawQuery
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(APIResourceListResponse{
				APIResources: []APIResource{
					// A near-match that must NOT be selected.
					{ID: "uuid-near", Name: "Users Other", Identifier: "/scim2/Users/Other", Type: "BUSINESS"},
					{ID: "uuid-users", Name: "SCIM2 Users", Identifier: "/scim2/Users", Type: "MGT"},
				},
			})
		default:
			t.Errorf("unexpected path %q", r.URL.Path)
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()

	c := newTestClient(srv.URL)
	got, err := c.GetAPIResourceByIdentifier(context.Background(), "/scim2/Users")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.ID != "uuid-users" {
		t.Errorf("expected exact match uuid-users, got %q", got.ID)
	}

	// The identifier must be URL-escaped inside the filter param. A raw "/" and
	// space would corrupt the query, so assert the escaped substring is present.
	if !strings.Contains(capturedQuery, "identifier+eq+%2Fscim2%2FUsers") {
		t.Errorf("filter not URL-escaped as expected, got query %q", capturedQuery)
	}
}

func TestGetAPIResourceByIdentifier_ItemsEnvelope(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/oauth2/token":
			writeToken(w)
		case "/api/server/v1/api-resources":
			w.Header().Set("Content-Type", "application/json")
			// Use the alternative "items" envelope key.
			_ = json.NewEncoder(w).Encode(APIResourceListResponse{
				Items: []APIResource{
					{ID: "uuid-groups", Name: "SCIM2 Groups", Identifier: "/scim2/Groups", Type: "MGT"},
				},
			})
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()

	c := newTestClient(srv.URL)
	got, err := c.GetAPIResourceByIdentifier(context.Background(), "/scim2/Groups")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.ID != "uuid-groups" {
		t.Errorf("expected uuid-groups, got %q", got.ID)
	}
}

func TestGetAPIResourceByIdentifier_NotFound(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/oauth2/token":
			writeToken(w)
		case "/api/server/v1/api-resources":
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(APIResourceListResponse{
				// Only a near-match — no exact identifier match.
				APIResources: []APIResource{
					{ID: "uuid-x", Identifier: "/scim2/UsersExtra"},
				},
			})
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()

	c := newTestClient(srv.URL)
	_, err := c.GetAPIResourceByIdentifier(context.Background(), "/scim2/Users")
	if !errors.Is(err, ErrAPIResourceNotFound) {
		t.Fatalf("expected ErrAPIResourceNotFound, got %v", err)
	}
	if !strings.Contains(err.Error(), "/scim2/Users") {
		t.Errorf("error should name the identifier, got %q", err.Error())
	}
}

func TestGetAPIResourceByIdentifier_Ambiguous(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/oauth2/token":
			writeToken(w)
		case "/api/server/v1/api-resources":
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(APIResourceListResponse{
				APIResources: []APIResource{
					{ID: "uuid-a", Identifier: "/scim2/Users"},
					{ID: "uuid-b", Identifier: "/scim2/Users"},
				},
			})
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()

	c := newTestClient(srv.URL)
	_, err := c.GetAPIResourceByIdentifier(context.Background(), "/scim2/Users")
	if !errors.Is(err, ErrAPIResourceAmbiguous) {
		t.Fatalf("expected ErrAPIResourceAmbiguous, got %v", err)
	}
}

func TestAuthorizeAPI_SendsUUIDNotIdentifier(t *testing.T) {
	var captured AuthorizeAPIRequest
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/oauth2/token":
			writeToken(w)
		case r.Method == http.MethodPost && r.URL.Path == "/api/server/v1/applications/app-1/authorized-apis":
			body, _ := io.ReadAll(r.Body)
			if err := json.Unmarshal(body, &captured); err != nil {
				t.Errorf("decode body: %v", err)
			}
			w.WriteHeader(http.StatusOK)
		default:
			t.Errorf("unexpected request %s %s", r.Method, r.URL.Path)
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()

	c := newTestClient(srv.URL)
	err := c.AuthorizeAPI(context.Background(), "app-1", "uuid-users", "RBAC",
		[]string{"internal_user_mgt_list", "internal_user_mgt_view"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Critical assertion: the body's "id" must be the API-resource UUID, not the
	// identifier string.
	if captured.ID != "uuid-users" {
		t.Errorf("expected body id=uuid-users (the UUID), got %q", captured.ID)
	}
	if captured.PolicyIdentifier != "RBAC" {
		t.Errorf("expected policyIdentifier=RBAC, got %q", captured.PolicyIdentifier)
	}
	wantScopes := []string{"internal_user_mgt_list", "internal_user_mgt_view"}
	if !reflect.DeepEqual(captured.Scopes, wantScopes) {
		t.Errorf("expected scopes %v, got %v", wantScopes, captured.Scopes)
	}
}

func TestAuthorizeAPI_SerializesExpectedJSONKeys(t *testing.T) {
	var rawBody string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/oauth2/token":
			writeToken(w)
		case r.URL.Path == "/api/server/v1/applications/app-1/authorized-apis":
			b, _ := io.ReadAll(r.Body)
			rawBody = string(b)
			w.WriteHeader(http.StatusCreated)
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()

	c := newTestClient(srv.URL)
	if err := c.AuthorizeAPI(context.Background(), "app-1", "uuid-1", "RBAC", []string{"s1"}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	for _, key := range []string{`"id"`, `"policyIdentifier"`, `"scopes"`} {
		if !strings.Contains(rawBody, key) {
			t.Errorf("body missing JSON key %s; body was %s", key, rawBody)
		}
	}
	// Must NOT serialize the identifier under "identifier".
	if strings.Contains(rawBody, `"identifier"`) {
		t.Errorf("body should not contain an identifier key; body was %s", rawBody)
	}
}

func TestPatchAuthorizedAPI_AddedRemovedBody(t *testing.T) {
	var captured PatchAuthorizedAPIRequest
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/oauth2/token":
			writeToken(w)
		case r.Method == http.MethodPatch && r.URL.Path == "/api/server/v1/applications/app-1/authorized-apis/uuid-1":
			body, _ := io.ReadAll(r.Body)
			if err := json.Unmarshal(body, &captured); err != nil {
				t.Errorf("decode body: %v", err)
			}
			w.WriteHeader(http.StatusOK)
		default:
			t.Errorf("unexpected request %s %s", r.Method, r.URL.Path)
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()

	c := newTestClient(srv.URL)
	err := c.PatchAuthorizedAPI(context.Background(), "app-1", "uuid-1",
		[]string{"new_scope"}, []string{"old_scope"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !reflect.DeepEqual(captured.AddedScopes, []string{"new_scope"}) {
		t.Errorf("expected addedScopes=[new_scope], got %v", captured.AddedScopes)
	}
	if !reflect.DeepEqual(captured.RemovedScopes, []string{"old_scope"}) {
		t.Errorf("expected removedScopes=[old_scope], got %v", captured.RemovedScopes)
	}
}

func TestGetAuthorizedAPIs_DecodesList(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/oauth2/token":
			writeToken(w)
		case "/api/server/v1/applications/app-1/authorized-apis":
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode([]AuthorizedAPI{
				{
					ID:          "uuid-users",
					Identifier:  "/scim2/Users",
					DisplayName: "SCIM2 Users",
					Type:        "MGT",
					AuthorizedScopes: []AuthorizedScope{
						{ID: "1", Name: "internal_user_mgt_list", DisplayName: "List users"},
						{ID: "2", Name: "internal_user_mgt_view", DisplayName: "View users"},
					},
				},
			})
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()

	c := newTestClient(srv.URL)
	apis, err := c.GetAuthorizedAPIs(context.Background(), "app-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(apis) != 1 {
		t.Fatalf("expected 1 authorized api, got %d", len(apis))
	}
	if apis[0].ID != "uuid-users" || len(apis[0].AuthorizedScopes) != 2 {
		t.Errorf("unexpected authorized api: %+v", apis[0])
	}
}

func TestGetAuthorizedAPI_FindsByID(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/oauth2/token":
			writeToken(w)
		case "/api/server/v1/applications/app-1/authorized-apis":
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode([]AuthorizedAPI{
				{ID: "uuid-a", Identifier: "/a"},
				{ID: "uuid-b", Identifier: "/b"},
			})
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()

	c := newTestClient(srv.URL)

	got, err := c.GetAuthorizedAPI(context.Background(), "app-1", "uuid-b")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got == nil || got.Identifier != "/b" {
		t.Errorf("expected to find uuid-b, got %+v", got)
	}

	// Absent ID returns nil, nil.
	missing, err := c.GetAuthorizedAPI(context.Background(), "app-1", "uuid-z")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if missing != nil {
		t.Errorf("expected nil for missing api resource, got %+v", missing)
	}
}

func TestDeleteAuthorizedAPI_404IsSuccess(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/oauth2/token":
			writeToken(w)
		case r.Method == http.MethodDelete && r.URL.Path == "/api/server/v1/applications/app-1/authorized-apis/uuid-gone":
			w.WriteHeader(http.StatusNotFound)
		case r.Method == http.MethodDelete && r.URL.Path == "/api/server/v1/applications/app-1/authorized-apis/uuid-1":
			w.WriteHeader(http.StatusNoContent)
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()

	c := newTestClient(srv.URL)

	if err := c.DeleteAuthorizedAPI(context.Background(), "app-1", "uuid-gone"); err != nil {
		t.Errorf("404 should be treated as success, got %v", err)
	}
	if err := c.DeleteAuthorizedAPI(context.Background(), "app-1", "uuid-1"); err != nil {
		t.Errorf("204 delete should succeed, got %v", err)
	}
}
