package applications

import (
	"context"
	"reflect"
	"testing"

	"github.com/asgardeo/terraform-provider-asgardeo/asgardeo"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

func TestEncodeCallbackURLs(t *testing.T) {
	tests := []struct {
		name string
		urls []string
		want []string
	}{
		{
			name: "empty list returns nil",
			urls: nil,
			want: nil,
		},
		{
			name: "empty non-nil slice returns nil",
			urls: []string{},
			want: nil,
		},
		{
			name: "single url passes through bare",
			urls: []string{"https://app.example.com/callback"},
			want: []string{"https://app.example.com/callback"},
		},
		{
			name: "two urls packed into regex alternation",
			urls: []string{"https://a.example.com/cb", "https://b.example.com/cb"},
			want: []string{"regexp=(https://a.example.com/cb|https://b.example.com/cb)"},
		},
		{
			name: "three urls packed into regex alternation",
			urls: []string{"https://a.example.com/cb", "https://b.example.com/cb", "https://c.example.com/cb"},
			want: []string{"regexp=(https://a.example.com/cb|https://b.example.com/cb|https://c.example.com/cb)"},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := encodeCallbackURLs(tc.urls)
			if !reflect.DeepEqual(got, tc.want) {
				t.Errorf("encodeCallbackURLs(%v): want %v, got %v", tc.urls, tc.want, got)
			}
		})
	}
}

func TestDecodeCallbackURLs(t *testing.T) {
	tests := []struct {
		name string
		urls []string
		want []string
	}{
		{
			name: "empty list returned unchanged",
			urls: []string{},
			want: []string{},
		},
		{
			name: "single bare url returned unchanged",
			urls: []string{"https://app.example.com/callback"},
			want: []string{"https://app.example.com/callback"},
		},
		{
			name: "single regex-packed string split back into urls",
			urls: []string{"regexp=(https://a.example.com/cb|https://b.example.com/cb)"},
			want: []string{"https://a.example.com/cb", "https://b.example.com/cb"},
		},
		{
			name: "three urls round-tripped from regex alternation",
			urls: []string{"regexp=(https://a.example.com/cb|https://b.example.com/cb|https://c.example.com/cb)"},
			want: []string{"https://a.example.com/cb", "https://b.example.com/cb", "https://c.example.com/cb"},
		},
		{
			name: "multi-element list returned unchanged (not a packed form)",
			urls: []string{"https://a.example.com/cb", "https://b.example.com/cb"},
			want: []string{"https://a.example.com/cb", "https://b.example.com/cb"},
		},
		{
			name: "bare string lacking the regexp prefix is left intact",
			urls: []string{"(https://a.example.com/cb|https://b.example.com/cb)"},
			want: []string{"(https://a.example.com/cb|https://b.example.com/cb)"},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := decodeCallbackURLs(tc.urls)
			if !reflect.DeepEqual(got, tc.want) {
				t.Errorf("decodeCallbackURLs(%v): want %v, got %v", tc.urls, tc.want, got)
			}
		})
	}
}

// TestCallbackURLsRoundTrip verifies that decode(encode(x)) == x for the cases
// the implementation is designed to support, and pins the KNOWN-BROKEN cases so
// that any future change to the encode/decode contract is caught here.
func TestCallbackURLsRoundTrip(t *testing.T) {
	tests := []struct {
		name string
		urls []string
		// want is what comes back after decode(encode(urls)). For well-behaved
		// inputs this equals urls; for the pinned broken cases it does not.
		want []string
		// lossless documents whether this round-trip is expected to be exact.
		lossless bool
	}{
		{
			name:     "single url round-trips exactly",
			urls:     []string{"https://app.example.com/callback"},
			want:     []string{"https://app.example.com/callback"},
			lossless: true,
		},
		{
			name:     "two ordinary urls round-trip exactly",
			urls:     []string{"https://a.example.com/cb", "https://b.example.com/cb"},
			want:     []string{"https://a.example.com/cb", "https://b.example.com/cb"},
			lossless: true,
		},
		{
			// KNOWN BEHAVIOR: a URL containing a literal "|" is split on that
			// pipe during decode, because decodeCallbackURLs does a naive
			// strings.Split(inner, "|"). The pipe is the alternation separator,
			// so multi-URL encoding cannot represent URLs that contain one. The
			// implementation does NOT escape pipes; this test pins that fact
			// rather than fixing it.
			name:     "pipe inside a multi-url entry corrupts the round-trip (known limitation)",
			urls:     []string{"https://a.example.com/cb?x=1|2", "https://b.example.com/cb"},
			want:     []string{"https://a.example.com/cb?x=1", "2", "https://b.example.com/cb"},
			lossless: false,
		},
		{
			// KNOWN BEHAVIOR: regex-special characters other than "|" survive
			// the round-trip untouched. encode just string-concatenates and
			// decode just trims the prefix/suffix and splits on "|", so parens,
			// dots, plus, etc. in the URL body pass through unchanged.
			name: "regex-special chars (except pipe) survive the round-trip",
			urls: []string{
				"https://a.example.com/cb?q=(x).+*",
				"https://b.example.com/cb?q=[y]",
			},
			want: []string{
				"https://a.example.com/cb?q=(x).+*",
				"https://b.example.com/cb?q=[y]",
			},
			lossless: true,
		},
		{
			// KNOWN BEHAVIOR: a single URL whose literal value already looks
			// like the regexp-packed wire form is sent bare by encode (len==1),
			// but decode then unpacks it because it matches the prefix/suffix.
			// So the round-trip splits a legitimate single URL.
			name:     "single url that mimics the wire form is unpacked on decode (known limitation)",
			urls:     []string{"regexp=(https://a.example.com/cb|https://b.example.com/cb)"},
			want:     []string{"https://a.example.com/cb", "https://b.example.com/cb"},
			lossless: false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := decodeCallbackURLs(encodeCallbackURLs(tc.urls))
			if !reflect.DeepEqual(got, tc.want) {
				t.Errorf("decode(encode(%v)): want %v, got %v", tc.urls, tc.want, got)
			}
			roundTripped := reflect.DeepEqual(got, tc.urls)
			if roundTripped != tc.lossless {
				t.Errorf("decode(encode(%v)) lossless=%v, expected lossless=%v (got %v)",
					tc.urls, roundTripped, tc.lossless, got)
			}
		})
	}
}

func TestPreserveOrder(t *testing.T) {
	tests := []struct {
		name    string
		desired []string
		current []string
		want    []string
	}{
		{
			name:    "same set different order returns desired order",
			desired: []string{"a", "b", "c"},
			current: []string{"c", "a", "b"},
			want:    []string{"a", "b", "c"},
		},
		{
			name:    "identical order returns desired order unchanged",
			desired: []string{"a", "b", "c"},
			current: []string{"a", "b", "c"},
			want:    []string{"a", "b", "c"},
		},
		{
			name:    "length mismatch (current has extra) returns current unchanged",
			desired: []string{"a", "b"},
			current: []string{"a", "b", "c"},
			want:    []string{"a", "b", "c"},
		},
		{
			name:    "length mismatch (current missing one) returns current unchanged",
			desired: []string{"a", "b", "c"},
			current: []string{"a", "b"},
			want:    []string{"a", "b"},
		},
		{
			name:    "same length but differing element returns current unchanged",
			desired: []string{"a", "b", "c"},
			current: []string{"a", "b", "d"},
			want:    []string{"a", "b", "d"},
		},
		{
			name:    "both empty returns empty",
			desired: []string{},
			current: []string{},
			want:    []string{},
		},
		{
			// Duplicate handling: the multiset count must match too. desired
			// asks for two "a" but current only has one, so the API ordering
			// wins.
			name:    "duplicate count mismatch returns current unchanged",
			desired: []string{"a", "a"},
			current: []string{"a", "b"},
			want:    []string{"a", "b"},
		},
		{
			name:    "matching duplicates are reordered to desired",
			desired: []string{"a", "b", "a"},
			current: []string{"a", "a", "b"},
			want:    []string{"a", "b", "a"},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := preserveOrder(tc.desired, tc.current)
			if !reflect.DeepEqual(got, tc.want) {
				t.Errorf("preserveOrder(%v, %v): want %v, got %v",
					tc.desired, tc.current, tc.want, got)
			}
		})
	}
}

// TestFlattenApplication_TemplateID pins the template_id behavior: Asgardeo's
// GET /applications/{id} never returns templateId, so flattenApplication must
// carry it through from the prior plan/state. When prior has a value it is
// preserved; when prior is empty it stays empty.
func TestFlattenApplication_TemplateID(t *testing.T) {
	app := &asgardeo.ApplicationResponse{
		ID:                 "app-123",
		Name:               "test-app",
		ApplicationEnabled: true,
	}

	tests := []struct {
		name  string
		prior applicationModel
		want  types.String
	}{
		{
			name:  "prior has template_id is preserved through the read",
			prior: applicationModel{TemplateID: types.StringValue("oidc-web-application")},
			want:  types.StringValue("oidc-web-application"),
		},
		{
			name:  "prior null template_id stays null",
			prior: applicationModel{TemplateID: types.StringNull()},
			want:  types.StringNull(),
		},
		{
			name:  "prior empty-string template_id stays empty string",
			prior: applicationModel{TemplateID: types.StringValue("")},
			want:  types.StringValue(""),
		},
		{
			name:  "zero-value prior model yields null template_id",
			prior: applicationModel{},
			want:  types.StringNull(),
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := flattenApplication(context.Background(), app, nil, nil, tc.prior)
			if !got.TemplateID.Equal(tc.want) {
				t.Errorf("flattenApplication TemplateID: want %v, got %v", tc.want, got.TemplateID)
			}
		})
	}
}

// TestBuildOIDCConfig_UnknownAndNullLists is the direct regression for the field
// failure: an M2M app sets only grant_types and omits callback_urls /
// allowed_origins / logout_redirect_urls, so those Optional+Computed lists are
// UNKNOWN at plan time. Before the fix the model used Go slices, which the
// framework cannot decode an unknown List into. With types.List + listToStrings,
// an unknown OR null list must collapse to "no value sent" rather than panic.
func TestBuildOIDCConfig_UnknownAndNullLists(t *testing.T) {
	ctx := context.Background()

	tests := []struct {
		name string
		m    oidcModel
	}{
		{
			name: "unknown lists (m2m, callbacks omitted at plan time)",
			m: oidcModel{
				GrantTypes:         []types.String{types.StringValue("client_credentials")},
				CallbackURLs:       types.ListUnknown(types.StringType),
				AllowedOrigins:     types.ListUnknown(types.StringType),
				LogoutRedirectURLs: types.ListUnknown(types.StringType),
			},
		},
		{
			name: "null lists",
			m: oidcModel{
				GrantTypes:         []types.String{types.StringValue("client_credentials")},
				CallbackURLs:       types.ListNull(types.StringType),
				AllowedOrigins:     types.ListNull(types.StringType),
				LogoutRedirectURLs: types.ListNull(types.StringType),
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			cfg := buildOIDCConfig(ctx, tc.m)
			if len(cfg.GrantTypes) != 1 || cfg.GrantTypes[0] != "client_credentials" {
				t.Errorf("GrantTypes: want [client_credentials], got %v", cfg.GrantTypes)
			}
			if cfg.CallbackURLs != nil {
				t.Errorf("CallbackURLs: want nil for unknown/null, got %v", cfg.CallbackURLs)
			}
			if cfg.AllowedOrigins != nil {
				t.Errorf("AllowedOrigins: want nil for unknown/null, got %v", cfg.AllowedOrigins)
			}
			if cfg.Logout != nil {
				t.Errorf("Logout: want nil for unknown/null logout_redirect_urls, got %v", cfg.Logout)
			}
		})
	}
}

// TestBuildOIDCConfig_PopulatedLists pins that concrete (known) list values flow
// through to the wire format: a single callback passes through bare, multiple
// callbacks pack into the regexp alternation, and allowed_origins pass through.
func TestBuildOIDCConfig_PopulatedLists(t *testing.T) {
	ctx := context.Background()

	m := oidcModel{
		GrantTypes:   []types.String{types.StringValue("authorization_code")},
		CallbackURLs: stringListValue([]string{"https://a.example.com/cb", "https://b.example.com/cb"}),
		AllowedOrigins: stringListValue([]string{
			"https://a.example.com",
		}),
		LogoutRedirectURLs: stringListValue([]string{"https://a.example.com/logout"}),
	}

	cfg := buildOIDCConfig(ctx, m)

	wantCB := []string{"regexp=(https://a.example.com/cb|https://b.example.com/cb)"}
	if !reflect.DeepEqual(cfg.CallbackURLs, wantCB) {
		t.Errorf("CallbackURLs: want %v, got %v", wantCB, cfg.CallbackURLs)
	}
	if !reflect.DeepEqual(cfg.AllowedOrigins, []string{"https://a.example.com"}) {
		t.Errorf("AllowedOrigins: want [https://a.example.com], got %v", cfg.AllowedOrigins)
	}
	if cfg.Logout == nil || cfg.Logout.FrontChannelLogoutURL != "https://a.example.com/logout" {
		t.Errorf("Logout: want front-channel https://a.example.com/logout, got %+v", cfg.Logout)
	}
}

// TestFlattenApplication_OIDCLists is the flatten-side regression: an OIDC config
// returned from the API must always settle the Optional+Computed list attributes
// to concrete (non-null, non-unknown) lists, even when the API returns nothing
// for them — otherwise a Computed attribute would stay unknown after apply.
func TestFlattenApplication_OIDCLists(t *testing.T) {
	ctx := context.Background()
	app := &asgardeo.ApplicationResponse{ID: "app-1", Name: "m2m", ApplicationEnabled: true}

	t.Run("empty oidc config yields concrete empty lists", func(t *testing.T) {
		got := flattenApplication(ctx, app, &asgardeo.OIDCConfiguration{
			GrantTypes: []string{"client_credentials"},
		}, nil, applicationModel{})
		if len(got.OIDC) != 1 {
			t.Fatalf("OIDC: want 1 block, got %d", len(got.OIDC))
		}
		om := got.OIDC[0]
		for name, l := range map[string]types.List{
			"callback_urls":        om.CallbackURLs,
			"allowed_origins":      om.AllowedOrigins,
			"logout_redirect_urls": om.LogoutRedirectURLs,
		} {
			if l.IsNull() || l.IsUnknown() {
				t.Errorf("%s: want concrete empty list, got null=%v unknown=%v", name, l.IsNull(), l.IsUnknown())
			}
			if len(l.Elements()) != 0 {
				t.Errorf("%s: want 0 elements, got %d", name, len(l.Elements()))
			}
		}
	})

	t.Run("populated oidc config round-trips through the lists", func(t *testing.T) {
		got := flattenApplication(ctx, app, &asgardeo.OIDCConfiguration{
			GrantTypes:     []string{"authorization_code"},
			CallbackURLs:   []string{"regexp=(https://a.example.com/cb|https://b.example.com/cb)"},
			AllowedOrigins: []string{"https://a.example.com"},
			Logout:         &asgardeo.OIDCLogoutConfig{FrontChannelLogoutURL: "https://a.example.com/logout"},
		}, nil, applicationModel{})
		om := got.OIDC[0]
		if vals := elementStrings(t, ctx, om.CallbackURLs); !reflect.DeepEqual(vals, []string{"https://a.example.com/cb", "https://b.example.com/cb"}) {
			t.Errorf("callback_urls decoded wrong: %v", vals)
		}
		if vals := elementStrings(t, ctx, om.AllowedOrigins); !reflect.DeepEqual(vals, []string{"https://a.example.com"}) {
			t.Errorf("allowed_origins wrong: %v", vals)
		}
		if vals := elementStrings(t, ctx, om.LogoutRedirectURLs); !reflect.DeepEqual(vals, []string{"https://a.example.com/logout"}) {
			t.Errorf("logout_redirect_urls wrong: %v", vals)
		}
	})
}

func elementStrings(t *testing.T, ctx context.Context, l types.List) []string {
	t.Helper()
	if l.IsNull() || l.IsUnknown() {
		return nil
	}
	out := make([]string, 0, len(l.Elements()))
	if diags := l.ElementsAs(ctx, &out, false); diags.HasError() {
		t.Fatalf("ElementsAs: %v", diags)
	}
	return out
}
