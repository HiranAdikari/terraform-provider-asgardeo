package applications

import (
	"context"
	"fmt"

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
	}

	// Attach OIDC protocol config if specified.
	if len(plan.OIDC) > 0 {
		oidcCfg := buildOIDCConfig(plan.OIDC[0])
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

	state := flattenApplication(app, oidcCfg, nil, plan)
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

	newState := flattenApplication(app, oidcCfg, samlCfg, state)
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
		cfg := buildOIDCConfig(plan.OIDC[0])
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

	newState := flattenApplication(app, oidcCfg, nil, plan)
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

	state := flattenApplication(app, oidcCfg, samlCfg, applicationModel{})
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

// ─── Model types ──────────────────────────────────────────────────────────────

// applicationModel is the Terraform state model for asgardeo_application.
type applicationModel struct {
	ID                 types.String             `tfsdk:"id"`
	ClientID           types.String             `tfsdk:"client_id"`
	ClientSecret       types.String             `tfsdk:"client_secret"`
	Name               types.String             `tfsdk:"name"`
	Description        types.String             `tfsdk:"description"`
	AccessURL          types.String             `tfsdk:"access_url"`
	LogoutReturnURL    types.String             `tfsdk:"logout_return_url"`
	ApplicationEnabled types.Bool               `tfsdk:"application_enabled"`
	OIDC               []oidcModel              `tfsdk:"oidc"`
	SAML               []samlModel              `tfsdk:"saml"`
	ClaimConfiguration []claimConfigurationModel `tfsdk:"claim_configuration"`
	Advanced           []advancedModel          `tfsdk:"advanced"`
}

type claimConfigurationModel struct {
	RequestedClaims []requestedClaimModel `tfsdk:"requested_claims"`
}

type requestedClaimModel struct {
	URI       types.String `tfsdk:"uri"`
	Mandatory types.Bool   `tfsdk:"mandatory"`
}

type oidcModel struct {
	GrantTypes         []types.String     `tfsdk:"grant_types"`
	CallbackURLs       []types.String     `tfsdk:"callback_urls"`
	AllowedOrigins     []types.String     `tfsdk:"allowed_origins"`
	LogoutRedirectURLs []types.String     `tfsdk:"logout_redirect_urls"`
	PublicClient       types.Bool         `tfsdk:"public_client"`
	PKCE               []pkceModel        `tfsdk:"pkce"`
	AccessToken        []accessTokenModel `tfsdk:"access_token"`
	RefreshToken       []refreshTokenModel `tfsdk:"refresh_token"`
}

type pkceModel struct {
	Mandatory                     types.Bool `tfsdk:"mandatory"`
	SupportPlainTransformAlgorithm types.Bool `tfsdk:"support_plain_transform_algorithm"`
}

type accessTokenModel struct {
	Type                              types.String `tfsdk:"type"`
	UserAccessTokenExpirySeconds      types.Int64  `tfsdk:"user_access_token_expiry_seconds"`
	ApplicationAccessTokenExpirySeconds types.Int64 `tfsdk:"application_access_token_expiry_seconds"`
}

type refreshTokenModel struct {
	ExpirySeconds    types.Int64 `tfsdk:"expiry_seconds"`
	RenewRefreshToken types.Bool  `tfsdk:"renew_refresh_token"`
}

type samlModel struct {
	ManualConfiguration []samlManualModel `tfsdk:"manual_configuration"`
}

type samlManualModel struct {
	Issuer                    types.String    `tfsdk:"issuer"`
	AssertionConsumerURLs     []types.String  `tfsdk:"assertion_consumer_urls"`
	DefaultAssertionConsumerURL types.String  `tfsdk:"default_assertion_consumer_url"`
	SingleLogout              []samlSLOModel  `tfsdk:"single_logout"`
}

type samlSLOModel struct {
	Enabled            types.Bool   `tfsdk:"enabled"`
	LogoutRequestURL   types.String `tfsdk:"logout_request_url"`
	LogoutResponseURL  types.String `tfsdk:"logout_response_url"`
}

type advancedModel struct {
	SkipLoginConsent      types.Bool `tfsdk:"skip_login_consent"`
	SkipLogoutConsent     types.Bool `tfsdk:"skip_logout_consent"`
	Saas                  types.Bool `tfsdk:"saas"`
	DiscoverableByEndUsers types.Bool `tfsdk:"discoverable_by_end_users"`
}

// ─── Builders (model → API struct) ───────────────────────────────────────────

func buildOIDCConfig(m oidcModel) asgardeo.OIDCConfiguration {
	cfg := asgardeo.OIDCConfiguration{
		PublicClient: m.PublicClient.ValueBool(),
	}

	for _, g := range m.GrantTypes {
		cfg.GrantTypes = append(cfg.GrantTypes, g.ValueString())
	}
	for _, u := range m.CallbackURLs {
		cfg.CallbackURLs = append(cfg.CallbackURLs, u.ValueString())
	}
	for _, o := range m.AllowedOrigins {
		cfg.AllowedOrigins = append(cfg.AllowedOrigins, o.ValueString())
	}
	// Map logout_redirect_urls to the front-channel logout URL.
	// Asgardeo stores only one front-channel logout URL; we use the first entry.
	if len(m.LogoutRedirectURLs) > 0 {
		cfg.Logout = &asgardeo.OIDCLogoutConfig{
			FrontChannelLogoutURL: m.LogoutRedirectURLs[0].ValueString(),
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
			Type:                              m.AccessToken[0].Type.ValueString(),
			UserAccessTokenExpiryInSeconds:    m.AccessToken[0].UserAccessTokenExpirySeconds.ValueInt64(),
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
		Issuer:                    mc.Issuer.ValueString(),
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

// flattenApplication converts API responses into the Terraform state model.
// The prior state/plan is passed so that blocks absent from the API response
// can be preserved as empty slices (Terraform requires consistent types).
func flattenApplication(
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
	}

	// Flatten advanced config.
	if app.AdvancedConfigurations != nil {
		adv := app.AdvancedConfigurations
		m.Advanced = []advancedModel{{
			SkipLoginConsent:       types.BoolValue(adv.SkipLoginConsent),
			SkipLogoutConsent:      types.BoolValue(adv.SkipLogoutConsent),
			Saas:                   types.BoolValue(adv.Saas),
			DiscoverableByEndUsers: types.BoolValue(adv.DiscoverableByEndUsers),
		}}
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
		for _, u := range oidcCfg.CallbackURLs {
			om.CallbackURLs = append(om.CallbackURLs, types.StringValue(u))
		}
		for _, o := range oidcCfg.AllowedOrigins {
			om.AllowedOrigins = append(om.AllowedOrigins, types.StringValue(o))
		}
		if oidcCfg.Logout != nil && oidcCfg.Logout.FrontChannelLogoutURL != "" {
			om.LogoutRedirectURLs = []types.String{types.StringValue(oidcCfg.Logout.FrontChannelLogoutURL)}
		}
		if oidcCfg.PKCE != nil {
			om.PKCE = []pkceModel{{
				Mandatory:                     types.BoolValue(oidcCfg.PKCE.Mandatory),
				SupportPlainTransformAlgorithm: types.BoolValue(oidcCfg.PKCE.SupportPlainTransformAlgorithm),
			}}
		}
		if oidcCfg.AccessToken != nil {
			om.AccessToken = []accessTokenModel{{
				Type:                              types.StringValue(oidcCfg.AccessToken.Type),
				UserAccessTokenExpirySeconds:      types.Int64Value(oidcCfg.AccessToken.UserAccessTokenExpiryInSeconds),
				ApplicationAccessTokenExpirySeconds: types.Int64Value(oidcCfg.AccessToken.ApplicationAccessTokenExpiryInSeconds),
			}}
		}
		if oidcCfg.RefreshToken != nil {
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
			Issuer:                    types.StringValue(mc.Issuer),
			DefaultAssertionConsumerURL: types.StringValue(mc.DefaultAssertionConsumerURL),
		}
		for _, u := range mc.AssertionConsumerURLs {
			mm.AssertionConsumerURLs = append(mm.AssertionConsumerURLs, types.StringValue(u))
		}
		if mc.SingleLogoutProfile != nil {
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
