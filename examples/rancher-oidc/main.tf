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
  org_name      = "hiranadikari"                                 #var.asgardeo_org_name
  client_id     = "yAuf8LpcVZfLdOMiXlGodIFIeEwa"                 #var.asgardeo_client_id
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
