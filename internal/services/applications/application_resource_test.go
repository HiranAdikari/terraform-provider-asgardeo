package applications

import (
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
			got := flattenApplication(app, nil, nil, tc.prior)
			if !got.TemplateID.Equal(tc.want) {
				t.Errorf("flattenApplication TemplateID: want %v, got %v", tc.want, got.TemplateID)
			}
		})
	}
}
