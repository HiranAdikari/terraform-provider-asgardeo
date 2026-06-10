# Authorize an M2M (machine-to-machine) application to call Asgardeo's
# management APIs. This is the Terraform equivalent of the "API Authorization"
# tab of an application in the Asgardeo Console.

# An M2M application that will call the SCIM2 management APIs using the
# client_credentials grant.
resource "asgardeo_application" "service" {
  name        = "user-sync-service"
  description = "Backend service that syncs users and groups"
  template_id = "m2m-application"

  oidc {
    grant_types = ["client_credentials"]
  }
}

# Authorize the application to list and view users via the SCIM2 Users API.
resource "asgardeo_application_authorized_api" "users" {
  application_id = asgardeo_application.service.id
  identifier     = "/scim2/Users"

  scopes = [
    "internal_user_mgt_list",
    "internal_user_mgt_view",
  ]
}

# Authorize the application to view groups via the SCIM2 Groups API.
resource "asgardeo_application_authorized_api" "groups" {
  application_id = asgardeo_application.service.id
  identifier     = "/scim2/Groups"

  scopes = [
    "internal_group_mgt_view",
  ]
}
