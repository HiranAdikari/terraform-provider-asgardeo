package applications

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"

	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"

	"github.com/asgardeo/terraform-provider-asgardeo/asgardeo"
	"github.com/asgardeo/terraform-provider-asgardeo/internal/clients"
)

// Ensure AuthorizedAPIResource satisfies the resource interfaces.
var _ resource.Resource = &AuthorizedAPIResource{}
var _ resource.ResourceWithImportState = &AuthorizedAPIResource{}

// AuthorizedAPIResource manages the asgardeo_application_authorized_api resource.
type AuthorizedAPIResource struct {
	client *clients.AsgardeoClient
}

// NewAuthorizedAPIResource is the factory function registered with the provider.
func NewAuthorizedAPIResource() resource.Resource {
	return &AuthorizedAPIResource{}
}

// authorizedAPIModel is the Terraform state model for
// asgardeo_application_authorized_api.
type authorizedAPIModel struct {
	ID               types.String `tfsdk:"id"`
	ApplicationID    types.String `tfsdk:"application_id"`
	Identifier       types.String `tfsdk:"identifier"`
	Scopes           types.Set    `tfsdk:"scopes"`
	PolicyIdentifier types.String `tfsdk:"policy_identifier"`
	APIResourceID    types.String `tfsdk:"api_resource_id"`
	DisplayName      types.String `tfsdk:"display_name"`
	Type             types.String `tfsdk:"type"`
}

// ─── Metadata ─────────────────────────────────────────────────────────────────

func (r *AuthorizedAPIResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_application_authorized_api"
}

// ─── Schema ───────────────────────────────────────────────────────────────────

func (r *AuthorizedAPIResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = authorizedAPIResourceSchema()
}

// ─── Configure ────────────────────────────────────────────────────────────────

func (r *AuthorizedAPIResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func (r *AuthorizedAPIResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan authorizedAPIModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	appID := plan.ApplicationID.ValueString()
	identifier := plan.Identifier.ValueString()
	policy := plan.PolicyIdentifier.ValueString()

	scopes, diags := scopesFromSet(ctx, plan.Scopes)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Resolve the human-readable identifier to its API-resource UUID.
	apiResource, err := r.client.GetAPIResourceByIdentifier(ctx, identifier)
	if err != nil {
		addAPIResourceLookupError(&resp.Diagnostics, identifier, err)
		return
	}

	tflog.Debug(ctx, "Authorizing API on application", map[string]any{
		"application_id":  appID,
		"identifier":      identifier,
		"api_resource_id": apiResource.ID,
		"scopes":          scopes,
	})

	if err := r.client.AuthorizeAPI(ctx, appID, apiResource.ID, policy, scopes); err != nil {
		resp.Diagnostics.AddError("Error authorizing API", err.Error())
		return
	}

	// Read back the authorized API to populate computed fields.
	authorized, err := r.client.GetAuthorizedAPI(ctx, appID, apiResource.ID)
	if err != nil {
		resp.Diagnostics.AddError("Error reading authorized API after create", err.Error())
		return
	}
	if authorized == nil {
		resp.Diagnostics.AddError(
			"Authorized API not found after create",
			fmt.Sprintf("API resource %q (%s) was not present on application %q after authorization.",
				identifier, apiResource.ID, appID),
		)
		return
	}

	state := flattenAuthorizedAPI(appID, policy, authorized)
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

// ─── Read ─────────────────────────────────────────────────────────────────────

func (r *AuthorizedAPIResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state authorizedAPIModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	appID := state.ApplicationID.ValueString()
	apiResourceID := state.APIResourceID.ValueString()

	authorized, err := r.client.GetAuthorizedAPI(ctx, appID, apiResourceID)
	if err != nil {
		resp.Diagnostics.AddError("Error reading authorized API", err.Error())
		return
	}
	if authorized == nil {
		resp.State.RemoveResource(ctx)
		return
	}

	newState := flattenAuthorizedAPI(appID, state.PolicyIdentifier.ValueString(), authorized)
	resp.Diagnostics.Append(resp.State.Set(ctx, &newState)...)
}

// ─── Update ───────────────────────────────────────────────────────────────────

func (r *AuthorizedAPIResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan, state authorizedAPIModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	appID := state.ApplicationID.ValueString()
	apiResourceID := state.APIResourceID.ValueString()
	policy := plan.PolicyIdentifier.ValueString()

	planScopes, diags := scopesFromSet(ctx, plan.Scopes)
	resp.Diagnostics.Append(diags...)
	stateScopes, diags := scopesFromSet(ctx, state.Scopes)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	added, removed := diffScopes(stateScopes, planScopes)

	if len(added) > 0 || len(removed) > 0 {
		tflog.Debug(ctx, "Patching authorized API scopes", map[string]any{
			"application_id":  appID,
			"api_resource_id": apiResourceID,
			"added":           added,
			"removed":         removed,
		})
		if err := r.client.PatchAuthorizedAPI(ctx, appID, apiResourceID, added, removed); err != nil {
			resp.Diagnostics.AddError("Error updating authorized API scopes", err.Error())
			return
		}
	}

	authorized, err := r.client.GetAuthorizedAPI(ctx, appID, apiResourceID)
	if err != nil {
		resp.Diagnostics.AddError("Error reading authorized API after update", err.Error())
		return
	}
	if authorized == nil {
		resp.State.RemoveResource(ctx)
		return
	}

	newState := flattenAuthorizedAPI(appID, policy, authorized)
	resp.Diagnostics.Append(resp.State.Set(ctx, &newState)...)
}

// ─── Delete ───────────────────────────────────────────────────────────────────

func (r *AuthorizedAPIResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state authorizedAPIModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	appID := state.ApplicationID.ValueString()
	apiResourceID := state.APIResourceID.ValueString()

	tflog.Debug(ctx, "Removing authorized API from application", map[string]any{
		"application_id":  appID,
		"api_resource_id": apiResourceID,
	})

	if err := r.client.DeleteAuthorizedAPI(ctx, appID, apiResourceID); err != nil {
		resp.Diagnostics.AddError("Error deleting authorized API", err.Error())
	}
}

// ─── ImportState ──────────────────────────────────────────────────────────────

// ImportState imports an authorized API using the "<application_id>/<api_resource_id>"
// composite ID. A subsequent Read populates the remaining attributes.
func (r *AuthorizedAPIResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	appID, apiResourceID, ok := strings.Cut(req.ID, "/")
	if !ok || appID == "" || apiResourceID == "" {
		resp.Diagnostics.AddError(
			"Invalid import ID",
			fmt.Sprintf("Expected import ID in the form \"<application_id>/<api_resource_id>\", got %q.", req.ID),
		)
		return
	}

	authorized, err := r.client.GetAuthorizedAPI(ctx, appID, apiResourceID)
	if err != nil {
		resp.Diagnostics.AddError("Error importing authorized API", err.Error())
		return
	}
	if authorized == nil {
		resp.Diagnostics.AddError(
			"Authorized API not found",
			fmt.Sprintf("No authorized API resource %q on application %q.", apiResourceID, appID),
		)
		return
	}

	// policy_identifier is not returned by the API in a form we can map back to
	// the policyIdentifier write value, so default it on import; the next plan
	// shows no diff because the schema default is also "RBAC".
	state := flattenAuthorizedAPI(appID, "RBAC", authorized)
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

// ─── Helpers ──────────────────────────────────────────────────────────────────

// flattenAuthorizedAPI converts an API response into the Terraform state model.
func flattenAuthorizedAPI(appID, policy string, a *asgardeo.AuthorizedAPI) authorizedAPIModel {
	scopeNames := make([]string, 0, len(a.AuthorizedScopes))
	for _, s := range a.AuthorizedScopes {
		scopeNames = append(scopeNames, s.Name)
	}

	scopesSet, _ := types.SetValueFrom(context.Background(), types.StringType, scopeNames)

	policyValue := policy
	if policyValue == "" {
		policyValue = "RBAC"
	}

	return authorizedAPIModel{
		ID:               types.StringValue(appID + "/" + a.ID),
		ApplicationID:    types.StringValue(appID),
		Identifier:       types.StringValue(a.Identifier),
		Scopes:           scopesSet,
		PolicyIdentifier: types.StringValue(policyValue),
		APIResourceID:    types.StringValue(a.ID),
		DisplayName:      types.StringValue(a.DisplayName),
		Type:             types.StringValue(a.Type),
	}
}

// scopesFromSet unwraps a types.Set of strings into a sorted []string. Sorting
// gives the PATCH/POST bodies a deterministic order, which keeps tests stable.
func scopesFromSet(ctx context.Context, set types.Set) ([]string, diag.Diagnostics) {
	var out []string
	diags := set.ElementsAs(ctx, &out, false)
	if diags.HasError() {
		return nil, diags
	}
	sort.Strings(out)
	return out, diags
}

// diffScopes computes the set difference between the old and new scope slices,
// returning scopes to add and scopes to remove.
func diffScopes(oldScopes, newScopes []string) (added, removed []string) {
	have := make(map[string]struct{}, len(oldScopes))
	for _, s := range oldScopes {
		have[s] = struct{}{}
	}
	want := make(map[string]struct{}, len(newScopes))
	for _, s := range newScopes {
		want[s] = struct{}{}
	}
	for _, s := range newScopes {
		if _, ok := have[s]; !ok {
			added = append(added, s)
		}
	}
	for _, s := range oldScopes {
		if _, ok := want[s]; !ok {
			removed = append(removed, s)
		}
	}
	sort.Strings(added)
	sort.Strings(removed)
	return added, removed
}

// addAPIResourceLookupError renders a clear diagnostic for the two distinct
// API-resource lookup failures, hinting at the most common cause (the M2M app
// missing the internal_api_resource_view authorization in the Console).
func addAPIResourceLookupError(diags *diag.Diagnostics, identifier string, err error) {
	switch {
	case errors.Is(err, asgardeo.ErrAPIResourceNotFound):
		diags.AddError(
			"API resource not found",
			fmt.Sprintf("No API resource matched the identifier %q.\n\n"+
				"Verify the identifier is correct (for example \"/scim2/Users\"). "+
				"If the identifier is correct, the M2M application used by this provider "+
				"may lack the \"API Resources / View\" (internal_api_resource_view) "+
				"authorization in the Asgardeo Console, in which case the lookup returns "+
				"no results.", identifier),
		)
	case errors.Is(err, asgardeo.ErrAPIResourceAmbiguous):
		diags.AddError(
			"Ambiguous API resource identifier",
			fmt.Sprintf("More than one API resource matched the identifier %q. %s",
				identifier, err.Error()),
		)
	default:
		diags.AddError("Error resolving API resource identifier", err.Error())
	}
}
