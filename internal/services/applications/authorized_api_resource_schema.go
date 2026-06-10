package applications

import (
	"github.com/hashicorp/terraform-plugin-framework-validators/setvalidator"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringdefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

// authorizedAPIResourceSchema returns the schema for the
// asgardeo_application_authorized_api resource.
func authorizedAPIResourceSchema() schema.Schema {
	return schema.Schema{
		MarkdownDescription: "Authorizes an application to call an Asgardeo **API resource** " +
			"(such as the SCIM2 Users management API) with a set of scopes.\n\n" +
			"This is the Terraform equivalent of the **API Authorization** tab of an " +
			"application in the Asgardeo Console. It is most commonly used to grant an " +
			"M2M application the management scopes it needs (for example " +
			"`internal_user_mgt_list` on `/scim2/Users`).\n\n" +
			"The API resource is referenced by its human-readable `identifier`; the " +
			"provider resolves it to the internal UUID automatically.",

		Attributes: map[string]schema.Attribute{
			// ── Computed identity ────────────────────────────────────────────
			"id": schema.StringAttribute{
				MarkdownDescription: "Composite identifier in the form " +
					"`<application_id>/<api_resource_id>`.",
				Computed:      true,
				PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"api_resource_id": schema.StringAttribute{
				MarkdownDescription: "UUID of the resolved API resource, assigned by Asgardeo.",
				Computed:            true,
				PlanModifiers:       []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"display_name": schema.StringAttribute{
				MarkdownDescription: "Display name of the authorized API resource.",
				Computed:            true,
				PlanModifiers:       []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"type": schema.StringAttribute{
				MarkdownDescription: "Type of the API resource (e.g. `MGT` for management APIs).",
				Computed:            true,
				PlanModifiers:       []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},

			// ── Required ──────────────────────────────────────────────────────
			"application_id": schema.StringAttribute{
				MarkdownDescription: "ID of the application to authorize the API on. " +
					"Changing this forces a new resource.",
				Required:      true,
				PlanModifiers: []planmodifier.String{stringplanmodifier.RequiresReplace()},
			},
			"identifier": schema.StringAttribute{
				MarkdownDescription: "Identifier of the API resource to authorize, e.g. " +
					"`/scim2/Users`. The provider resolves this to the internal API-resource " +
					"UUID. Changing this forces a new resource.",
				Required:      true,
				PlanModifiers: []planmodifier.String{stringplanmodifier.RequiresReplace()},
			},
			"scopes": schema.SetAttribute{
				MarkdownDescription: "Scopes to grant on the API resource, e.g. " +
					"`[\"internal_user_mgt_list\", \"internal_user_mgt_view\"]`. " +
					"At least one scope is required. Scopes can be updated in place " +
					"without replacing the resource.",
				Required:    true,
				ElementType: types.StringType,
				Validators: []validator.Set{
					setvalidator.SizeAtLeast(1),
				},
			},

			// ── Optional + Computed ───────────────────────────────────────────
			"policy_identifier": schema.StringAttribute{
				MarkdownDescription: "Authorization policy applied to the granted scopes. " +
					"Defaults to `RBAC`. Changing this forces a new resource.",
				Optional:      true,
				Computed:      true,
				Default:       stringdefault.StaticString("RBAC"),
				PlanModifiers: []planmodifier.String{stringplanmodifier.RequiresReplace()},
			},
		},
	}
}
