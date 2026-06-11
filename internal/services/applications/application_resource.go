package applications

import (
	"context"
	"fmt"
	"strings"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"

	"github.com/asgardeo/terraform-provider-asgardeo/asgardeo"
	"github.com/asgardeo/terraform-provider-asgardeo/internal/clients"
)

// Ensure ApplicationResource satisfies the resource.Resource interface.
var _ resource.Resource = &ApplicationResource{}
var _ resource.ResourceWithImportState = &ApplicationResource{}

// ApplicationResource manages the asgardeo_application resource.
type ApplicationResource struct {
	client *clients.AsgardeoClient
}

// NewApplicationResource is the factory function registered with the provider.
func NewApplicationResource() resource.Resource {
	return &ApplicationResource{}
}

// ─── Metadata ─────────────────────────────────────────────────────────────────

func (r *ApplicationResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_application"
}

// ─── Schema ───────────────────────────────────────────────────────────────────

func (r *ApplicationResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = applicationResourceSchema()
}

// ─── Configure ────────────────────────────────────────────────────────────────

func (r *ApplicationResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}
	client, ok := req.ProviderData.(*clients.AsgardeoClient)
	if !ok {
		resp.Diagnostics.AddError(
			"Unexpected provider data type",
			fmt.Sprintf("Expected *clients.AsgardeoClient, got %T", req.ProviderData),
		)
		return
	}
	r.client = client
}

// ─── Create ───────────────────────────────────────────────────────────────────

func (r *ApplicationResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan applicationModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	createReq := asgardeo.ApplicationCreateRequest{
		Name:               plan.Name.ValueString(),
		Description:        plan.Description.ValueString(),
		AccessURL:          plan.AccessURL.ValueString(),
		LogoutReturnURL:    plan.LogoutReturnURL.ValueString(),
		ApplicationEnabled: plan.ApplicationEnabled.ValueBool(),
		TemplateID:         plan.TemplateID.ValueString(),
	}

	// Attach OIDC protocol config if specified.
	if len(plan.OIDC) > 0 {
		oidcCfg := buildOIDCConfig(ctx, plan.OIDC[0])
		createReq.InboundProtocolConfiguration = &asgardeo.InboundProtocolConfiguration{
			OIDC: &oidcCfg,
		}
	}

	// Attach SAML protocol config if specified.
	if len(plan.SAML) > 0 {
		samlCfg := buildSAMLConfig(plan.SAML[0])
		if createReq.InboundProtocolConfiguration == nil {
			createReq.InboundProtocolConfiguration = &asgardeo.InboundProtocolConfiguration{}
		}
		createReq.InboundProtocolConfiguration.SAML = &samlCfg
	}

	// Attach advanced config if specified.
	if len(plan.Advanced) > 0 {
		adv := buildAdvancedConfig(plan.Advanced[0])
		createReq.AdvancedConfigurations = &adv
	}

	// Attach claim configuration if specified.
	if len(plan.ClaimConfiguration) > 0 {
		cc := buildClaimConfig(plan.ClaimConfiguration[0])
		createReq.ClaimConfiguration = &cc
	}

	tflog.Debug(ctx, "Creating Asgardeo application", map[string]any{"name": plan.Name.ValueString()})

	app, err := r.client.CreateApplication(ctx, createReq)
	if err != nil {
		resp.Diagnostics.AddError("Error creating application", err.Error())
		return
	}

	// After creation, fetch OIDC config to get the server-generated client_id/secret.
	var oidcCfg *asgardeo.OIDCConfiguration
	if len(plan.OIDC) > 0 {
		oidcCfg, err = r.client.GetOIDCConfig(ctx, app.ID)
		if err != nil {
			resp.Diagnostics.AddError("Error reading OIDC config after create", err.Error())
			return
		}
	}

	state := flattenApplication(ctx, app, oidcCfg, nil, plan)
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

// ─── Read ─────────────────────────────────────────────────────────────────────

func (r *ApplicationResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state applicationModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	app, err := r.client.GetApplication(ctx, state.ID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Error reading application", err.Error())
		return
	}
	if app == nil {
		resp.State.RemoveResource(ctx)
		return
	}

	var oidcCfg *asgardeo.OIDCConfiguration
	if len(state.OIDC) > 0 {
		oidcCfg, err = r.client.GetOIDCConfig(ctx, app.ID)
		if err != nil {
			resp.Diagnostics.AddError("Error reading OIDC config", err.Error())
			return
		}
	}

	var samlCfg *asgardeo.SAMLConfiguration
	if len(state.SAML) > 0 {
		samlCfg, err = r.client.GetSAMLConfig(ctx, app.ID)
		if err != nil {
			resp.Diagnostics.AddError("Error reading SAML config", err.Error())
			return
		}
	}

	newState := flattenApplication(ctx, app, oidcCfg, samlCfg, state)
	resp.Diagnostics.Append(resp.State.Set(ctx, &newState)...)
}

// ─── Update ───────────────────────────────────────────────────────────────────

func (r *ApplicationResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan, state applicationModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	id := state.ID.ValueString()
	enabled := plan.ApplicationEnabled.ValueBool()

	patchReq := asgardeo.ApplicationPatchRequest{
		Name:               plan.Name.ValueString(),
		Description:        plan.Description.ValueString(),
		AccessURL:          plan.AccessURL.ValueString(),
		LogoutReturnURL:    plan.LogoutReturnURL.ValueString(),
		ApplicationEnabled: &enabled,
	}

	if len(plan.Advanced) > 0 {
		adv := buildAdvancedConfig(plan.Advanced[0])
		patchReq.AdvancedConfigurations = &adv
	}

	if len(plan.ClaimConfiguration) > 0 {
		cc := buildClaimConfig(plan.ClaimConfiguration[0])
		patchReq.ClaimConfiguration = &cc
	}

	if err := r.client.PatchApplication(ctx, id, patchReq); err != nil {
		resp.Diagnostics.AddError("Error updating application", err.Error())
		return
	}

	// Update OIDC protocol config.
	var oidcCfg *asgardeo.OIDCConfiguration
	if len(plan.OIDC) > 0 {
		cfg := buildOIDCConfig(ctx, plan.OIDC[0])
		// Asgardeo requires the existing client_id in the PUT body for validation.
		cfg.ClientID = state.ClientID.ValueString()
		updated, err := r.client.PutOIDCConfig(ctx, id, cfg)
		if err != nil {
			resp.Diagnostics.AddError("Error updating OIDC config", err.Error())
			return
		}
		oidcCfg = updated
	}

	app, err := r.client.GetApplication(ctx, id)
	if err != nil {
		resp.Diagnostics.AddError("Error reading application after update", err.Error())
		return
	}

	newState := flattenApplication(ctx, app, oidcCfg, nil, plan)
	resp.Diagnostics.Append(resp.State.Set(ctx, &newState)...)
}

// ─── Delete ───────────────────────────────────────────────────────────────────

func (r *ApplicationResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state applicationModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	tflog.Debug(ctx, "Deleting Asgardeo application", map[string]any{"id": state.ID.ValueString()})

	if err := r.client.DeleteApplication(ctx, state.ID.ValueString()); err != nil {
		resp.Diagnostics.AddError("Error deleting application", err.Error())
	}
}

// ─── ImportState ──────────────────────────────────────────────────────────────

func (r *ApplicationResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	// Import by application ID.
	app, err := r.client.GetApplication(ctx, req.ID)
	if err != nil {
		resp.Diagnostics.AddError("Error importing application", err.Error())
		return
	}
	if app == nil {
		resp.Diagnostics.AddError("Application not found", fmt.Sprintf("No application with ID %q", req.ID))
		return
	}

	oidcCfg, _ := r.client.GetOIDCConfig(ctx, app.ID)
	samlCfg, _ := r.client.GetSAMLConfig(ctx, app.ID)

	state := flattenApplication(ctx, app, oidcCfg, samlCfg, applicationModel{})
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

// ─── Model types ──────────────────────────────────────────────────────────────

// applicationModel is the Terraform state model for asgardeo_application.
type applicationModel struct {
	ID                 types.String              `tfsdk:"id"`
	ClientID           types.String              `tfsdk:"client_id"`
	ClientSecret       types.String              `tfsdk:"client_secret"`
	Name               types.String              `tfsdk:"name"`
	Description        types.String              `tfsdk:"description"`
	AccessURL          types.String              `tfsdk:"access_url"`
	LogoutReturnURL    types.String              `tfsdk:"logout_return_url"`
	ApplicationEnabled types.Bool                `tfsdk:"application_enabled"`
	TemplateID         types.String              `tfsdk:"template_id"`
	OIDC               []oidcModel               `tfsdk:"oidc"`
	SAML               []samlModel               `tfsdk:"saml"`
	ClaimConfiguration []claimConfigurationModel `tfsdk:"claim_configuration"`
	Advanced           []advancedModel           `tfsdk:"advanced"`
}

type claimConfigurationModel struct {
	RequestedClaims []requestedClaimModel `tfsdk:"requested_claims"`
}

type requestedClaimModel struct {
	URI       types.String `tfsdk:"uri"`
	Mandatory types.Bool   `tfsdk:"mandatory"`
}

type oidcModel struct {
	// grant_types is Required-only in the schema, so it can never be unknown at
	// plan time — a plain slice is safe. callback_urls, allowed_origins and
	// logout_redirect_urls are Optional+Computed, so an omitted value is UNKNOWN
	// at plan time and the framework cannot decode unknown into a Go slice; they
	// must be types.List. See the M2M (client_credentials, no callbacks) repro.
	GrantTypes         []types.String      `tfsdk:"grant_types"`
	CallbackURLs       types.List          `tfsdk:"callback_urls"`
	AllowedOrigins     types.List          `tfsdk:"allowed_origins"`
	LogoutRedirectURLs types.List          `tfsdk:"logout_redirect_urls"`
	PublicClient       types.Bool          `tfsdk:"public_client"`
	PKCE               []pkceModel         `tfsdk:"pkce"`
	AccessToken        []accessTokenModel  `tfsdk:"access_token"`
	RefreshToken       []refreshTokenModel `tfsdk:"refresh_token"`
}

type pkceModel struct {
	Mandatory                      types.Bool `tfsdk:"mandatory"`
	SupportPlainTransformAlgorithm types.Bool `tfsdk:"support_plain_transform_algorithm"`
}

type accessTokenModel struct {
	Type                                types.String `tfsdk:"type"`
	UserAccessTokenExpirySeconds        types.Int64  `tfsdk:"user_access_token_expiry_seconds"`
	ApplicationAccessTokenExpirySeconds types.Int64  `tfsdk:"application_access_token_expiry_seconds"`
}

type refreshTokenModel struct {
	ExpirySeconds     types.Int64 `tfsdk:"expiry_seconds"`
	RenewRefreshToken types.Bool  `tfsdk:"renew_refresh_token"`
}

type samlModel struct {
	ManualConfiguration []samlManualModel `tfsdk:"manual_configuration"`
}

type samlManualModel struct {
	Issuer                      types.String   `tfsdk:"issuer"`
	AssertionConsumerURLs       []types.String `tfsdk:"assertion_consumer_urls"`
	DefaultAssertionConsumerURL types.String   `tfsdk:"default_assertion_consumer_url"`
	SingleLogout                []samlSLOModel `tfsdk:"single_logout"`
}

type samlSLOModel struct {
	Enabled           types.Bool   `tfsdk:"enabled"`
	LogoutRequestURL  types.String `tfsdk:"logout_request_url"`
	LogoutResponseURL types.String `tfsdk:"logout_response_url"`
}

type advancedModel struct {
	SkipLoginConsent       types.Bool `tfsdk:"skip_login_consent"`
	SkipLogoutConsent      types.Bool `tfsdk:"skip_logout_consent"`
	Saas                   types.Bool `tfsdk:"saas"`
	DiscoverableByEndUsers types.Bool `tfsdk:"discoverable_by_end_users"`
}

// ─── Callback URL encoding ───────────────────────────────────────────────────
//
// Asgardeo only accepts a single string in the OIDC `callbackURLs` array. To
// register multiple redirect URIs the API requires a regex alternation packed
// into one element: `regexp=(url1|url2|...)`. We hide that quirk from the
// schema so consumers keep using a natural list.

const callbackRegexpPrefix = "regexp=("
const callbackRegexpSuffix = ")"

// encodeCallbackURLs turns a list of URLs into the wire format Asgardeo
// expects. Two or more entries are joined into a regex alternation; a single
// entry is sent as-is.
func encodeCallbackURLs(urls []string) []string {
	switch len(urls) {
	case 0:
		return nil
	case 1:
		return []string{urls[0]}
	default:
		return []string{callbackRegexpPrefix + strings.Join(urls, "|") + callbackRegexpSuffix}
	}
}

// decodeCallbackURLs reverses encodeCallbackURLs. If the API returned a single
// regex-packed string, split it back into the original URLs.
func decodeCallbackURLs(urls []string) []string {
	if len(urls) != 1 {
		return urls
	}
	s := urls[0]
	if strings.HasPrefix(s, callbackRegexpPrefix) && strings.HasSuffix(s, callbackRegexpSuffix) {
		inner := strings.TrimSuffix(strings.TrimPrefix(s, callbackRegexpPrefix), callbackRegexpSuffix)
		return strings.Split(inner, "|")
	}
	return urls
}

// listToStrings unwraps a types.List of strings into a plain []string. A null
// or unknown list yields nil — callers must treat that as "no value set" rather
// than an empty list. This is the read counterpart to stringListValue and is
// what lets Optional+Computed list attributes survive the unknown-at-plan-time
// case without a decode panic.
func listToStrings(ctx context.Context, l types.List) []string {
	if l.IsNull() || l.IsUnknown() {
		return nil
	}
	out := make([]string, 0, len(l.Elements()))
	// Errors here are impossible for a List(StringType); ignore the diagnostics.
	_ = l.ElementsAs(ctx, &out, false)
	return out
}

// stringListValue builds a types.List of strings from a plain []string. An empty
// or nil slice yields a non-null, non-unknown empty list so that Computed list
// attributes always settle to a concrete value in state.
func stringListValue(urls []string) types.List {
	elems := make([]attr.Value, 0, len(urls))
	for _, u := range urls {
		elems = append(elems, types.StringValue(u))
	}
	return types.ListValueMust(types.StringType, elems)
}

// preserveOrder returns `current` reordered to match the order of `desired`
// when the two contain the same set of values. If the sets differ the API is
// authoritative and `current` is returned unchanged. This avoids spurious
// "inconsistent result after apply" errors when Asgardeo returns a list in a
// different order than the user wrote.
func preserveOrder(desired, current []string) []string {
	if len(desired) != len(current) {
		return current
	}
	have := make(map[string]int, len(current))
	for _, v := range current {
		have[v]++
	}
	for _, v := range desired {
		if have[v] == 0 {
			return current
		}
		have[v]--
	}
	out := make([]string, len(desired))
	copy(out, desired)
	return out
}

// ─── Builders (model → API struct) ───────────────────────────────────────────

func buildOIDCConfig(ctx context.Context, m oidcModel) asgardeo.OIDCConfiguration {
	cfg := asgardeo.OIDCConfiguration{
		PublicClient: m.PublicClient.ValueBool(),
	}

	for _, g := range m.GrantTypes {
		cfg.GrantTypes = append(cfg.GrantTypes, g.ValueString())
	}
	cfg.CallbackURLs = encodeCallbackURLs(listToStrings(ctx, m.CallbackURLs))
	cfg.AllowedOrigins = append(cfg.AllowedOrigins, listToStrings(ctx, m.AllowedOrigins)...)
	// Asgardeo's `logout.frontChannelLogoutUrl` is a single string field but
	// also accepts the same `regexp=(url1|url2|...)` alternation as
	// `callbackURLs`, so we use the helper to support multiple logout URLs.
	if logoutURLs := listToStrings(ctx, m.LogoutRedirectURLs); len(logoutURLs) > 0 {
		cfg.Logout = &asgardeo.OIDCLogoutConfig{
			FrontChannelLogoutURL: encodeCallbackURLs(logoutURLs)[0],
		}
	}

	if len(m.PKCE) > 0 {
		cfg.PKCE = &asgardeo.PKCEConfig{
			Mandatory:                      m.PKCE[0].Mandatory.ValueBool(),
			SupportPlainTransformAlgorithm: m.PKCE[0].SupportPlainTransformAlgorithm.ValueBool(),
		}
	}
	if len(m.AccessToken) > 0 {
		cfg.AccessToken = &asgardeo.AccessTokenConfig{
			Type:                                  m.AccessToken[0].Type.ValueString(),
			UserAccessTokenExpiryInSeconds:        m.AccessToken[0].UserAccessTokenExpirySeconds.ValueInt64(),
			ApplicationAccessTokenExpiryInSeconds: m.AccessToken[0].ApplicationAccessTokenExpirySeconds.ValueInt64(),
		}
	}
	if len(m.RefreshToken) > 0 {
		cfg.RefreshToken = &asgardeo.RefreshTokenConfig{
			ExpiryInSeconds:   m.RefreshToken[0].ExpirySeconds.ValueInt64(),
			RenewRefreshToken: m.RefreshToken[0].RenewRefreshToken.ValueBool(),
		}
	}
	return cfg
}

func buildSAMLConfig(m samlModel) asgardeo.SAMLConfiguration {
	if len(m.ManualConfiguration) == 0 {
		return asgardeo.SAMLConfiguration{}
	}
	mc := m.ManualConfiguration[0]
	manual := &asgardeo.SAMLManualConfiguration{
		Issuer:                      mc.Issuer.ValueString(),
		DefaultAssertionConsumerURL: mc.DefaultAssertionConsumerURL.ValueString(),
	}
	for _, u := range mc.AssertionConsumerURLs {
		manual.AssertionConsumerURLs = append(manual.AssertionConsumerURLs, u.ValueString())
	}
	if len(mc.SingleLogout) > 0 {
		slo := mc.SingleLogout[0]
		manual.SingleLogoutProfile = &asgardeo.SAMLSLOProfile{
			Enabled:           slo.Enabled.ValueBool(),
			LogoutRequestURL:  slo.LogoutRequestURL.ValueString(),
			LogoutResponseURL: slo.LogoutResponseURL.ValueString(),
		}
	}
	return asgardeo.SAMLConfiguration{ManualConfiguration: manual}
}

func buildClaimConfig(m claimConfigurationModel) asgardeo.ClaimConfiguration {
	cfg := asgardeo.ClaimConfiguration{Dialect: "LOCAL"}
	for _, rc := range m.RequestedClaims {
		cfg.RequestedClaims = append(cfg.RequestedClaims, asgardeo.RequestedClaim{
			Claim:     asgardeo.ClaimRef{URI: rc.URI.ValueString()},
			Mandatory: rc.Mandatory.ValueBool(),
		})
	}
	return cfg
}

func buildAdvancedConfig(m advancedModel) asgardeo.AdvancedConfigurations {
	return asgardeo.AdvancedConfigurations{
		SkipLoginConsent:       m.SkipLoginConsent.ValueBool(),
		SkipLogoutConsent:      m.SkipLogoutConsent.ValueBool(),
		Saas:                   m.Saas.ValueBool(),
		DiscoverableByEndUsers: m.DiscoverableByEndUsers.ValueBool(),
	}
}

// ─── Flatteners (API struct → model) ─────────────────────────────────────────

// oidcSubBlockConfigured reports whether the prior plan/state declared a
// particular OIDC sub-block (pkce / access_token / refresh_token). The
// framework treats nested blocks as user-authored config: if the prior config
// did not contain the block, the flattened state must not contain it either,
// even when the API returns server-side defaults for it. On Create/Update the
// prior is the plan; on import the prior is empty, so unconfigured sub-blocks
// are suppressed — matching the `advanced {}` import semantics. A nil prior
// (no oidc block at all) means nothing was configured.
func oidcSubBlockConfigured(prior *oidcModel, has func(*oidcModel) bool) bool {
	if prior == nil {
		return false
	}
	return has(prior)
}

// samlSLOConfigured reports whether the prior plan/state declared a
// `single_logout {}` block under saml.manual_configuration. Like the OIDC
// sub-blocks, Asgardeo returns a singleLogoutProfile with defaults even when
// the user never wrote the block, so we suppress it unless it was configured.
func samlSLOConfigured(prior applicationModel) bool {
	if len(prior.SAML) == 0 || len(prior.SAML[0].ManualConfiguration) == 0 {
		return false
	}
	return len(prior.SAML[0].ManualConfiguration[0].SingleLogout) > 0
}

// flattenApplication converts API responses into the Terraform state model.
// The prior state/plan is passed so that blocks absent from the API response
// can be preserved as empty slices (Terraform requires consistent types).
func flattenApplication(
	ctx context.Context,
	app *asgardeo.ApplicationResponse,
	oidcCfg *asgardeo.OIDCConfiguration,
	samlCfg *asgardeo.SAMLConfiguration,
	prior applicationModel,
) applicationModel {
	m := applicationModel{
		ID:                 types.StringValue(app.ID),
		Name:               types.StringValue(app.Name),
		Description:        types.StringValue(app.Description),
		AccessURL:          types.StringValue(app.AccessURL),
		LogoutReturnURL:    types.StringValue(app.LogoutReturnURL),
		ApplicationEnabled: types.BoolValue(app.ApplicationEnabled),
		ClientID:           prior.ClientID,
		ClientSecret:       prior.ClientSecret,
		// Asgardeo's GET /applications/{id} does not return templateId, so
		// preserve the plan/state value through reads. RequiresReplace on the
		// schema means changing it forces a recreate (templates are seed-only).
		TemplateID: prior.TemplateID,
	}

	// Asgardeo always returns advancedConfigurations populated with defaults,
	// so only surface the block when the user actually configured one or when
	// any field has a non-zero value. Otherwise an empty `advanced {}` block
	// would appear in state that the user never wrote.
	if app.AdvancedConfigurations != nil {
		adv := app.AdvancedConfigurations
		anySet := adv.SkipLoginConsent || adv.SkipLogoutConsent || adv.Saas || adv.DiscoverableByEndUsers
		if anySet || len(prior.Advanced) > 0 {
			m.Advanced = []advancedModel{{
				SkipLoginConsent:       types.BoolValue(adv.SkipLoginConsent),
				SkipLogoutConsent:      types.BoolValue(adv.SkipLogoutConsent),
				Saas:                   types.BoolValue(adv.Saas),
				DiscoverableByEndUsers: types.BoolValue(adv.DiscoverableByEndUsers),
			}}
		}
	} else if len(prior.Advanced) > 0 {
		m.Advanced = prior.Advanced
	}

	// Flatten OIDC config.
	if oidcCfg != nil {
		// Update computed client credentials.
		if oidcCfg.ClientID != "" {
			m.ClientID = types.StringValue(oidcCfg.ClientID)
		}
		if oidcCfg.ClientSecret != "" {
			m.ClientSecret = types.StringValue(oidcCfg.ClientSecret)
		}

		om := oidcModel{
			PublicClient: types.BoolValue(oidcCfg.PublicClient),
		}
		for _, g := range oidcCfg.GrantTypes {
			om.GrantTypes = append(om.GrantTypes, types.StringValue(g))
		}
		var priorOIDC *oidcModel
		if len(prior.OIDC) > 0 {
			priorOIDC = &prior.OIDC[0]
		}
		decoded := decodeCallbackURLs(oidcCfg.CallbackURLs)
		if priorOIDC != nil {
			decoded = preserveOrder(listToStrings(ctx, priorOIDC.CallbackURLs), decoded)
		}
		om.CallbackURLs = stringListValue(decoded)

		origins := append([]string(nil), oidcCfg.AllowedOrigins...)
		if priorOIDC != nil {
			origins = preserveOrder(listToStrings(ctx, priorOIDC.AllowedOrigins), origins)
		}
		om.AllowedOrigins = stringListValue(origins)

		var logoutURLs []string
		if oidcCfg.Logout != nil && oidcCfg.Logout.FrontChannelLogoutURL != "" {
			logoutURLs = decodeCallbackURLs([]string{oidcCfg.Logout.FrontChannelLogoutURL})
			if priorOIDC != nil {
				logoutURLs = preserveOrder(listToStrings(ctx, priorOIDC.LogoutRedirectURLs), logoutURLs)
			}
		}
		om.LogoutRedirectURLs = stringListValue(logoutURLs)

		// Asgardeo always returns pkce, accessToken and refreshToken populated
		// with server-side defaults for every OIDC app — even an M2M app that
		// only declared grant_types. The framework forbids materialising a
		// nested block the user never wrote: doing so trips "block count changed
		// from 0 to 1" on apply. So gate each sub-block on whether the prior
		// plan/state actually declared it. When it was declared we surface the
		// API's values (or carry the prior block through on import); when it was
		// null it stays null regardless of the API defaults. This mirrors the
		// `advanced {}` suppression pattern.
		if oidcSubBlockConfigured(priorOIDC, func(p *oidcModel) bool { return len(p.PKCE) > 0 }) && oidcCfg.PKCE != nil {
			om.PKCE = []pkceModel{{
				Mandatory:                      types.BoolValue(oidcCfg.PKCE.Mandatory),
				SupportPlainTransformAlgorithm: types.BoolValue(oidcCfg.PKCE.SupportPlainTransformAlgorithm),
			}}
		}
		if oidcSubBlockConfigured(priorOIDC, func(p *oidcModel) bool { return len(p.AccessToken) > 0 }) && oidcCfg.AccessToken != nil {
			om.AccessToken = []accessTokenModel{{
				Type:                                types.StringValue(oidcCfg.AccessToken.Type),
				UserAccessTokenExpirySeconds:        types.Int64Value(oidcCfg.AccessToken.UserAccessTokenExpiryInSeconds),
				ApplicationAccessTokenExpirySeconds: types.Int64Value(oidcCfg.AccessToken.ApplicationAccessTokenExpiryInSeconds),
			}}
		}
		if oidcSubBlockConfigured(priorOIDC, func(p *oidcModel) bool { return len(p.RefreshToken) > 0 }) && oidcCfg.RefreshToken != nil {
			om.RefreshToken = []refreshTokenModel{{
				ExpirySeconds:     types.Int64Value(oidcCfg.RefreshToken.ExpiryInSeconds),
				RenewRefreshToken: types.BoolValue(oidcCfg.RefreshToken.RenewRefreshToken),
			}}
		}
		m.OIDC = []oidcModel{om}
	} else {
		m.OIDC = []oidcModel{}
	}

	// Flatten claim configuration.
	if app.ClaimConfiguration != nil && len(app.ClaimConfiguration.RequestedClaims) > 0 {
		cm := claimConfigurationModel{}
		for _, rc := range app.ClaimConfiguration.RequestedClaims {
			cm.RequestedClaims = append(cm.RequestedClaims, requestedClaimModel{
				URI:       types.StringValue(rc.Claim.URI),
				Mandatory: types.BoolValue(rc.Mandatory),
			})
		}
		m.ClaimConfiguration = []claimConfigurationModel{cm}
	} else if len(prior.ClaimConfiguration) > 0 {
		m.ClaimConfiguration = prior.ClaimConfiguration
	} else {
		m.ClaimConfiguration = []claimConfigurationModel{}
	}

	// Flatten SAML config.
	if samlCfg != nil && samlCfg.ManualConfiguration != nil {
		mc := samlCfg.ManualConfiguration
		mm := samlManualModel{
			Issuer:                      types.StringValue(mc.Issuer),
			DefaultAssertionConsumerURL: types.StringValue(mc.DefaultAssertionConsumerURL),
		}
		for _, u := range mc.AssertionConsumerURLs {
			mm.AssertionConsumerURLs = append(mm.AssertionConsumerURLs, types.StringValue(u))
		}
		// Same exposure as the OIDC sub-blocks: Asgardeo returns a
		// singleLogoutProfile with defaults even when the user never wrote a
		// `single_logout {}` block. Only surface it when the prior plan/state
		// declared it, so an unconfigured block stays absent after apply.
		if samlSLOConfigured(prior) && mc.SingleLogoutProfile != nil {
			mm.SingleLogout = []samlSLOModel{{
				Enabled:           types.BoolValue(mc.SingleLogoutProfile.Enabled),
				LogoutRequestURL:  types.StringValue(mc.SingleLogoutProfile.LogoutRequestURL),
				LogoutResponseURL: types.StringValue(mc.SingleLogoutProfile.LogoutResponseURL),
			}}
		}
		m.SAML = []samlModel{{ManualConfiguration: []samlManualModel{mm}}}
	} else {
		m.SAML = []samlModel{}
	}

	return m
}
