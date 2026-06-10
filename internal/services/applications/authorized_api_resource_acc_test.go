package applications_test

import (
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/providerserver"
	"github.com/hashicorp/terraform-plugin-go/tfprotov6"

	"github.com/asgardeo/terraform-provider-asgardeo/internal/provider"
)

// testAccProtoV6ProviderFactories wires the provider for acceptance tests. It
// lives in the external _test package so it can import internal/provider (which
// imports this service package) without creating an import cycle. Referencing it
// from the scaffold below compiles and vets the provider-factory wiring even
// when TF_ACC is unset.
var testAccProtoV6ProviderFactories = map[string]func() (tfprotov6.ProviderServer, error){
	"asgardeo": providerserver.NewProtocol6WithError(provider.New("test")()),
}

// TestAccAuthorizedAPIResource is a scaffold for the acceptance test. It runs
// only when TF_ACC is set and would additionally require ASGARDEO_* credentials,
// a live application, and a real API resource. The module does not yet vendor
// terraform-plugin-testing, so this stays a compile-checked placeholder that
// validates the provider-factory wiring; replace the Skip with resource.Test
// steps once the acceptance harness is added.
//
// Example acceptance flow to implement:
//  1. Create an asgardeo_application (m2m-application) as a dependency.
//  2. Create asgardeo_application_authorized_api on /scim2/Users with
//     [internal_user_mgt_list, internal_user_mgt_view].
//  3. Assert api_resource_id, display_name, and type are computed.
//  4. Update scopes (drop one, add one) and assert the in-place PATCH.
//  5. Import using "<application_id>/<api_resource_id>" and assert no diff.
func TestAccAuthorizedAPIResource(t *testing.T) {
	t.Skip("acceptance test scaffold: requires TF_ACC, ASGARDEO_* credentials, " +
		"a live application, and terraform-plugin-testing in the module")

	// Reference the factories so the wiring is compiled and vetted.
	_ = testAccProtoV6ProviderFactories
}
