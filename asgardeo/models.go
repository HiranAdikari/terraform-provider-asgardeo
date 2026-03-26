// Package asgardeo provides a pure-Go client for the Asgardeo Management API.
// It has no dependency on any Terraform SDK and can be used independently.
package asgardeo

// ─── Application ─────────────────────────────────────────────────────────────

// ApplicationCreateRequest is sent to POST /applications.
type ApplicationCreateRequest struct {
	Name                        string                       `json:"name"`
	Description                 string                       `json:"description,omitempty"`
	ImageURL                    string                       `json:"imageUrl,omitempty"`
	AccessURL                   string                       `json:"accessUrl,omitempty"`
	LogoutReturnURL             string                       `json:"logoutReturnUrl,omitempty"`
	IsManagementApp             bool                         `json:"isManagementApp,omitempty"`
	ApplicationEnabled          bool                         `json:"applicationEnabled,omitempty"`
	InboundProtocolConfiguration *InboundProtocolConfiguration `json:"inboundProtocolConfiguration,omitempty"`
	AdvancedConfigurations      *AdvancedConfigurations      `json:"advancedConfigurations,omitempty"`
	ClaimConfiguration          *ClaimConfiguration          `json:"claimConfiguration,omitempty"`
}

// ApplicationPatchRequest is sent to PATCH /applications/{id}.
// All fields are optional — only non-zero values are serialised.
type ApplicationPatchRequest struct {
	Name                   string                  `json:"name,omitempty"`
	Description            string                  `json:"description,omitempty"`
	AccessURL              string                  `json:"accessUrl,omitempty"`
	LogoutReturnURL        string                  `json:"logoutReturnUrl,omitempty"`
	ApplicationEnabled     *bool                   `json:"applicationEnabled,omitempty"`
	AdvancedConfigurations *AdvancedConfigurations `json:"advancedConfigurations,omitempty"`
	ClaimConfiguration     *ClaimConfiguration     `json:"claimConfiguration,omitempty"`
}

// ApplicationResponse is returned by GET /applications/{id}.
type ApplicationResponse struct {
	ID                     string                  `json:"id"`
	Name                   string                  `json:"name"`
	Description            string                  `json:"description"`
	ImageURL               string                  `json:"imageUrl"`
	AccessURL              string                  `json:"accessUrl"`
	LogoutReturnURL        string                  `json:"logoutReturnUrl"`
	ApplicationEnabled     bool                    `json:"applicationEnabled"`
	IsManagementApp        bool                    `json:"isManagementApp"`
	AdvancedConfigurations *AdvancedConfigurations `json:"advancedConfigurations"`
	ClaimConfiguration     *ClaimConfiguration     `json:"claimConfiguration"`
	// InboundProtocols contains lightweight references; full config is fetched separately.
	InboundProtocols []InboundProtocolRef `json:"inboundProtocols"`
}

// InboundProtocolRef is the lightweight reference returned inside ApplicationResponse.
type InboundProtocolRef struct {
	Type      string `json:"type"`
	SelfLink  string `json:"self"`
}

// ApplicationListResponse is returned by GET /applications.
type ApplicationListResponse struct {
	TotalResults int                   `json:"totalResults"`
	StartIndex   int                   `json:"startIndex"`
	Count        int                   `json:"count"`
	Applications []ApplicationResponse `json:"applications"`
}

// ─── Inbound Protocol Configuration ──────────────────────────────────────────

// InboundProtocolConfiguration wraps OIDC and SAML protocol configs used on create.
type InboundProtocolConfiguration struct {
	OIDC *OIDCConfiguration `json:"oidc,omitempty"`
	SAML *SAMLConfiguration `json:"saml,omitempty"`
}

// ─── OIDC ─────────────────────────────────────────────────────────────────────

// OIDCConfiguration maps to the OpenIDConnectConfiguration schema in the Asgardeo API.
type OIDCConfiguration struct {
	// Read-only fields returned by the server.
	ClientID     string `json:"clientId,omitempty"`
	ClientSecret string `json:"clientSecret,omitempty"`
	State        string `json:"state,omitempty"`

	// Writable fields.
	GrantTypes                       []string         `json:"grantTypes"`
	CallbackURLs                     []string         `json:"callbackURLs,omitempty"`
	AllowedOrigins                   []string         `json:"allowedOrigins,omitempty"`
	PublicClient                     bool             `json:"publicClient,omitempty"`
	PKCE                             *PKCEConfig      `json:"pkce,omitempty"`
	AccessToken                      *AccessTokenConfig `json:"accessToken,omitempty"`
	RefreshToken                     *RefreshTokenConfig `json:"refreshToken,omitempty"`
	IDToken                          *IDTokenConfig   `json:"idToken,omitempty"`
	Logout                           *OIDCLogoutConfig `json:"logout,omitempty"`
	ValidateRequestObjectSignature   bool             `json:"validateRequestObjectSignature,omitempty"`
	IsFAPIApplication                bool             `json:"isFAPIApplication,omitempty"`
}

// PKCEConfig controls PKCE settings.
type PKCEConfig struct {
	Mandatory                      bool `json:"mandatory"`
	SupportPlainTransformAlgorithm bool `json:"supportPlainTransformAlgorithm"`
}

// AccessTokenConfig controls access token behaviour.
type AccessTokenConfig struct {
	Type                              string `json:"type,omitempty"`
	UserAccessTokenExpiryInSeconds    int64  `json:"userAccessTokenExpiryInSeconds,omitempty"`
	ApplicationAccessTokenExpiryInSeconds int64 `json:"applicationAccessTokenExpiryInSeconds,omitempty"`
	BindingType                       string `json:"bindingType,omitempty"`
	RevokeTokensWhenIDPSessionTerminated bool `json:"revokeTokensWhenIDPSessionTerminated,omitempty"`
	ValidateTokenBinding              bool   `json:"validateTokenBinding,omitempty"`
}

// RefreshTokenConfig controls refresh token behaviour.
type RefreshTokenConfig struct {
	ExpiryInSeconds                 int64 `json:"expiryInSeconds,omitempty"`
	RenewRefreshToken               bool  `json:"renewRefreshToken,omitempty"`
	ExtendRenewedRefreshTokenExpiry bool  `json:"extendRenewedRefreshTokenExpiryTime,omitempty"`
}

// IDTokenConfig controls ID token behaviour.
type IDTokenConfig struct {
	ExpiryInSeconds        int64    `json:"expiryInSeconds,omitempty"`
	Audience               []string `json:"audience,omitempty"`
	IDTokenSignedResponseAlg string `json:"idTokenSignedResponseAlg,omitempty"`
}

// OIDCLogoutConfig holds front-channel and back-channel logout URLs.
type OIDCLogoutConfig struct {
	BackChannelLogoutURL  string `json:"backChannelLogoutUrl,omitempty"`
	FrontChannelLogoutURL string `json:"frontChannelLogoutUrl,omitempty"`
}

// ─── SAML ─────────────────────────────────────────────────────────────────────

// SAMLConfiguration wraps the three mutually exclusive SAML config modes.
type SAMLConfiguration struct {
	MetadataFile        string                    `json:"metadataFile,omitempty"`
	MetadataURL         string                    `json:"metadataURL,omitempty"`
	ManualConfiguration *SAMLManualConfiguration  `json:"manualConfiguration,omitempty"`
}

// SAMLManualConfiguration is used when SAML metadata is supplied manually.
type SAMLManualConfiguration struct {
	Issuer                    string              `json:"issuer"`
	AssertionConsumerURLs     []string            `json:"assertionConsumerUrls"`
	DefaultAssertionConsumerURL string            `json:"defaultAssertionConsumerUrl,omitempty"`
	SingleSignOnProfile       *SAMLSSOProfile     `json:"singleSignOnProfile,omitempty"`
	SingleLogoutProfile       *SAMLSLOProfile     `json:"singleLogoutProfile,omitempty"`
	ResponseSigning           *SAMLResponseSigning `json:"responseSigning,omitempty"`
}

// SAMLSSOProfile controls SSO bindings and assertion settings.
type SAMLSSOProfile struct {
	Bindings                                     []string         `json:"bindings,omitempty"`
	EnableSignatureValidationForArtifactBinding  bool             `json:"enableSignatureValidationForArtifactBinding,omitempty"`
	EnableIdpInitiatedSingleSignOn               bool             `json:"enableIdpInitiatedSingleSignOn,omitempty"`
	Assertion                                    *SAMLAssertion   `json:"assertion,omitempty"`
}

// SAMLAssertion controls assertion-level settings.
type SAMLAssertion struct {
	NameIDFormat     string `json:"nameIdFormat,omitempty"`
	DigestAlgorithm  string `json:"digestAlgorithm,omitempty"`
}

// SAMLSLOProfile controls single logout settings.
type SAMLSLOProfile struct {
	Enabled              bool   `json:"enabled"`
	LogoutRequestURL     string `json:"logoutRequestUrl,omitempty"`
	LogoutResponseURL    string `json:"logoutResponseUrl,omitempty"`
	LogoutMethod         string `json:"logoutMethod,omitempty"`
}

// SAMLResponseSigning controls signing of SAML responses.
type SAMLResponseSigning struct {
	Enabled          bool   `json:"enabled"`
	SigningAlgorithm string `json:"signingAlgorithm,omitempty"`
}

// ─── Claim Configuration ──────────────────────────────────────────────────────

// ClaimConfiguration controls which user attributes are included in tokens issued to the application.
type ClaimConfiguration struct {
	Dialect         string           `json:"dialect,omitempty"`
	RequestedClaims []RequestedClaim `json:"requestedClaims,omitempty"`
}

// RequestedClaim represents a single claim requested from the identity provider.
type RequestedClaim struct {
	Claim     ClaimRef `json:"claim"`
	Mandatory bool     `json:"mandatory"`
}

// ClaimRef references a local claim by its URI.
type ClaimRef struct {
	URI string `json:"uri"`
}

// ─── Advanced Configuration ───────────────────────────────────────────────────

// AdvancedConfigurations holds advanced application settings.
type AdvancedConfigurations struct {
	Saas                        bool `json:"saas,omitempty"`
	DiscoverableByEndUsers      bool `json:"discoverableByEndUsers,omitempty"`
	SkipLoginConsent            bool `json:"skipLoginConsent,omitempty"`
	SkipLogoutConsent           bool `json:"skipLogoutConsent,omitempty"`
	ReturnAuthenticatedIdpList  bool `json:"returnAuthenticatedIdpList,omitempty"`
	EnableAuthorization         bool `json:"enableAuthorization,omitempty"`
	EnableAPIBasedAuthentication bool `json:"enableAPIBasedAuthentication,omitempty"`
}

// ─── Error ────────────────────────────────────────────────────────────────────

// APIError represents an error response from the Asgardeo API.
type APIError struct {
	Code        string `json:"code"`
	Message     string `json:"message"`
	Description string `json:"description"`
	TraceID     string `json:"traceId"`
}

func (e *APIError) Error() string {
	if e.Description != "" {
		return e.Code + ": " + e.Message + " — " + e.Description
	}
	return e.Code + ": " + e.Message
}
