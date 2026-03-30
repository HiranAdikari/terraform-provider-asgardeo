# Look up an existing application by name.
data "asgardeo_application" "by_name" {
  name = "My Existing App"
}

# Look up an existing application by ID.
data "asgardeo_application" "by_id" {
  id = "e98e-4a6f-bd81-1234567890ab"
}

output "app_client_id_by_name" {
  value = data.asgardeo_application.by_name.client_id
}

output "app_client_id_by_id" {
  value = data.asgardeo_application.by_id.client_id
}
