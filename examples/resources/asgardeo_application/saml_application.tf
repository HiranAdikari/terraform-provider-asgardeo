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
