---
page_title: "asgardeo_application Resource - asgardeo"
subcategory: ""
description: |-
  Manages an Asgardeo application with OIDC or SAML inbound protocol support.
---

# asgardeo_application (Resource)

Manages an application in Asgardeo. Supports **OIDC**, **OAuth2**, and **SAML** inbound protocols.

After creation, the provider exposes the server-generated `client_id` and `client_secret` as
computed attributes — these are the credentials your service uses to authenticate with Asgardeo.

### OIDC Web Application

```terraform
# Minimal OIDC application — used by terraform-plugin-docs to generate docs.
resource "asgardeo_application" "example" {
  name        = "my-web-app"
  description = "Example OIDC web application"
  access_url  = "https://app.example.com/login"

  oidc {
    grant_types     = ["authorization_code", "refresh_token"]
    callback_urls   = ["https://app.example.com/callback"]
    allowed_origins = ["https://app.example.com"]

    pkce {
      mandatory = true
    }
  }

  claim_configuration {
    requested_claims {
      uri       = "http://wso2.org/claims/emailaddress"
      mandatory = true
    }
    requested_claims {
      uri       = "http://wso2.org/claims/username"
      mandatory = true
    }
  }

  advanced {
    skip_login_consent            = true
    return_authenticated_idp_list = true
  }
}

output "client_id" {
  value = asgardeo_application.example.client_id
}

output "client_secret" {
  value     = asgardeo_application.example.client_secret
  sensitive = true
}
```

### SAML 2.0 Application

```terraform
# Example SAML web application
resource "asgardeo_application" "saml_app" {
  name        = "my-saml-webapp"
  description = "A sample SAML 2.0 web application"
  access_url  = "https://app.example.com/saml/login"

  saml {
    issuer                         = "my-saml-issuer"
    assertion_consumer_urls        = ["https://app.example.com/saml/acs"]
    default_assertion_consumer_url = "https://app.example.com/saml/acs"
  }
}
```

### Rancher Manager OIDC

This example shows how to configure Asgardeo as an OIDC provider for Rancher, ensuring required claims like `email` are returned.

```terraform
terraform {
  required_providers {
    asgardeo = {
      source  = "asgardeo/asgardeo"
      version = "~> 0.1"
    }
    # Uncomment to wire directly into harvester-dc-terraform's Rancher provider:
    # rancher2 = {
    #   source  = "rancher/rancher2"
    #   version = "~> 4.0"
    # }
  }
}

provider "asgardeo" {
  org_name      = "hiranadikari" #var.asgardeo_org_name
  client_id     = "yAuf8LpcVZfLdOMiXlGodIFIeEwa" #var.asgardeo_client_id
  client_secret = "cV4p5rfkutLmshdubVHJEh5Zce593tVRQUc4k_tvXqQa" #var.asgardeo_client_secret
}

# ──────────────────────────────────────────────────────────────────────────────
# Rancher OIDC application in Asgardeo
#
# Rancher Manager uses the following OIDC endpoints:
#   Callback (verify-auth): <rancher_url>/verify-auth
#   Logout redirect:        <rancher_url>/dashboard/auth/logout
#
# The output client_id + client_secret are fed into Rancher's
# "Generic OIDC" authentication provider settings.
# ──────────────────────────────────────────────────────────────────────────────
# resource "asgardeo_application" "rancher" {
#   name        = var.app_name
#   description = "Rancher Manager SSO via Asgardeo. Managed by Terraform."
#   access_url  = "${var.rancher_url}/dashboard"

#   oidc {
#     # Rancher requires authorization_code; refresh_token recommended for session keep-alive.
#     grant_types = ["authorization_code", "refresh_token"]

#     # Rancher's OIDC callback URL (configured in Rancher UI → Authentication → OIDC).
#     callback_urls = ["${var.rancher_url}/verify-auth"]

#     # Allow requests from the Rancher origin (needed for token refresh XHR calls).
#     allowed_origins = [var.rancher_url]

#     # Where Asgardeo redirects after a logout is triggered from Rancher.
#     logout_redirect_urls = ["${var.rancher_url}/dashboard/auth/logout"]

#     # Rancher supports PKCE — enable for better security.
#     pkce {
#       mandatory                         = true
#       support_plain_transform_algorithm = false
#     }

#     access_token {
#       type                             = "JWT"
#       user_access_token_expiry_seconds = 3600
#     }

#     refresh_token {
#       expiry_seconds      = 86400
#       renew_refresh_token = true
#     }
#   }

#   advanced {
#     # Skip consent screens for a seamless SSO experience in Rancher.
#     skip_login_consent  = var.skip_consent
#     skip_logout_consent = var.skip_consent
#   }
# }

# ──────────────────────────────────────────────────────────────────────────────
# (Optional) Configure Rancher's Generic OIDC provider using the outputs above.
# Uncomment when the rancher2 provider is available and rancher_url is resolvable.
# ──────────────────────────────────────────────────────────────────────────────
# provider "rancher2" {
#   api_url  = var.rancher_url
#   # Supply token from harvester-dc-terraform's 01-rancher-auth remote state.
#   token_key = "<rancher-admin-token>"
# }
#
# resource "rancher2_auth_config_genericoidc" "asgardeo" {
#   name                        = "generic-oidc"
#   display_name_field          = "name"
#   groups_field                = "groups"
#   username_field              = "email"
#   issuer                      = "https://api.asgardeo.io/t/${var.asgardeo_org_name}/oauth2/token"
#   auth_endpoint               = "https://api.asgardeo.io/t/${var.asgardeo_org_name}/oauth2/authorize"
#   token_endpoint              = "https://api.asgardeo.io/t/${var.asgardeo_org_name}/oauth2/token"
#   user_info_endpoint          = "https://api.asgardeo.io/t/${var.asgardeo_org_name}/oauth2/userinfo"
#   jwks_url                    = "https://api.asgardeo.io/t/${var.asgardeo_org_name}/oauth2/jwks"
#   client_id                   = asgardeo_application.rancher.client_id
#   client_secret               = asgardeo_application.rancher.client_secret
#   scopes                      = "openid profile email groups"
#   enabled                     = true
# }
```

## Import

Import an existing application using its Asgardeo application ID:

```shell
terraform import asgardeo_application.example <application-id>
```

<!-- schema generated by tfplugindocs -->
## Schema

### Required

- `name` (String) Display name of the application. Must be unique within the organisation.

### Optional

- `access_url` (String) URL users land on when they launch this application from the My Account portal.
- `advanced` (Block List) Advanced application configuration. (see [below for nested schema](#nestedblock--advanced))
- `application_enabled` (Boolean) Whether the application is enabled. Defaults to `true`.
- `claim_configuration` (Block List) Controls which user attributes (claims) are included in tokens issued to this application. Use this to ensure claims like `email` or `username` are returned in the OIDC token so that Rancher (and other relying parties) can display readable usernames. (see [below for nested schema](#nestedblock--claim_configuration))
- `description` (String) Human-readable description of the application.
- `logout_return_url` (String) URL users are redirected to after a global logout.
- `oidc` (Block List) OIDC / OAuth2 inbound protocol configuration. Specify exactly one of `oidc` or `saml`. (see [below for nested schema](#nestedblock--oidc))
- `saml` (Block List) SAML 2.0 inbound protocol configuration. Specify exactly one of `oidc` or `saml`. (see [below for nested schema](#nestedblock--saml))

### Read-Only

- `client_id` (String) OAuth2 / OIDC client ID generated by Asgardeo. Available after creation.
- `client_secret` (String, Sensitive) OAuth2 / OIDC client secret generated by Asgardeo. Sensitive.
- `id` (String) Unique identifier of the application assigned by Asgardeo.

<a id="nestedblock--advanced"></a>
### Nested Schema for `advanced`

Optional:

- `discoverable_by_end_users` (Boolean) Show this application in the My Account portal. Defaults to `false`.
- `saas` (Boolean) Mark this as a SaaS (multi-tenant) application. Defaults to `false`.
- `skip_login_consent` (Boolean) Skip the user consent screen on login. Defaults to `false`.
- `skip_logout_consent` (Boolean) Skip the user consent screen on logout. Defaults to `false`.


<a id="nestedblock--claim_configuration"></a>
### Nested Schema for `claim_configuration`

Optional:

- `requested_claims` (Block List) List of local claims to include in the token. Common Asgardeo claim URIs:

- `http://wso2.org/claims/emailaddress` — email address
- `http://wso2.org/claims/username` — username
- `http://wso2.org/claims/givenname` — first name
- `http://wso2.org/claims/lastname` — last name (see [below for nested schema](#nestedblock--claim_configuration--requested_claims))

<a id="nestedblock--claim_configuration--requested_claims"></a>
### Nested Schema for `claim_configuration.requested_claims`

Required:

- `uri` (String) Local claim URI, e.g. `http://wso2.org/claims/emailaddress`.

Optional:

- `mandatory` (Boolean) Require the claim to be present. Defaults to `false`.



<a id="nestedblock--oidc"></a>
### Nested Schema for `oidc`

Required:

- `grant_types` (List of String) OAuth2 grant types to enable. At least one is required.

Supported values: `authorization_code`, `client_credentials`, `refresh_token`, `implicit`, `password`, `token_exchange`, `organization_switch`.

Optional:

- `access_token` (Block List) Access token configuration. (see [below for nested schema](#nestedblock--oidc--access_token))
- `allowed_origins` (List of String) CORS-allowed origins for token and authorisation requests. Relevant for browser-based (SPA) applications and Rancher OIDC.
- `callback_urls` (List of String) Allowed redirect (callback) URIs. Required when `authorization_code` or `implicit` grant types are used.

Tip: this is the field Rancher calls **Authorized Redirect URIs**.
- `logout_redirect_urls` (List of String) Post-logout redirect URIs. Stored as the OIDC front-channel logout URL in Asgardeo. Tip: this is the URL Rancher uses after a user signs out.
- `pkce` (Block List) PKCE (Proof Key for Code Exchange) settings. (see [below for nested schema](#nestedblock--oidc--pkce))
- `public_client` (Boolean) When `true` the client may authenticate without a secret (e.g. SPAs). Defaults to `false`.
- `refresh_token` (Block List) Refresh token configuration. (see [below for nested schema](#nestedblock--oidc--refresh_token))

<a id="nestedblock--oidc--access_token"></a>
### Nested Schema for `oidc.access_token`

Optional:

- `application_access_token_expiry_seconds` (Number) Application (M2M) access token TTL in seconds. Defaults to 3600.
- `type` (String) Token type: `JWT` or `Default` (opaque). Defaults to `JWT`.
- `user_access_token_expiry_seconds` (Number) User access token TTL in seconds. Defaults to 3600.


<a id="nestedblock--oidc--pkce"></a>
### Nested Schema for `oidc.pkce`

Optional:

- `mandatory` (Boolean) Require PKCE for all authorisation code flows. Defaults to `false`.
- `support_plain_transform_algorithm` (Boolean) Allow the plain (S256-less) code challenge method. Defaults to `false`.


<a id="nestedblock--oidc--refresh_token"></a>
### Nested Schema for `oidc.refresh_token`

Optional:

- `expiry_seconds` (Number) Refresh token TTL in seconds. Defaults to 86400 (24 h).
- `renew_refresh_token` (Boolean) Issue a new refresh token on each use. Defaults to `true`.



<a id="nestedblock--saml"></a>
### Nested Schema for `saml`

Optional:

- `manual_configuration` (Block List) Manually specify SAML metadata fields. (see [below for nested schema](#nestedblock--saml--manual_configuration))

<a id="nestedblock--saml--manual_configuration"></a>
### Nested Schema for `saml.manual_configuration`

Required:

- `assertion_consumer_urls` (List of String) List of assertion consumer service (ACS) URLs.
- `issuer` (String) SAML SP entity ID (issuer).

Optional:

- `default_assertion_consumer_url` (String) Default ACS URL. Must be one of `assertion_consumer_urls`.
- `single_logout` (Block List) Single logout profile settings. (see [below for nested schema](#nestedblock--saml--manual_configuration--single_logout))

<a id="nestedblock--saml--manual_configuration--single_logout"></a>
### Nested Schema for `saml.manual_configuration.single_logout`

Optional:

- `enabled` (Boolean) Enable single logout. Defaults to `true`.
- `logout_request_url` (String)
- `logout_response_url` (String)
