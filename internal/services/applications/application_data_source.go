package applications

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/asgardeo/terraform-provider-asgardeo/internal/clients"
)

// Ensure ApplicationDataSource satisfies the datasource.DataSource interface.
var _ datasource.DataSource = &ApplicationDataSource{}

// ApplicationDataSource provides a read-only view of an Asgardeo application.
type ApplicationDataSource struct {
	client *clients.AsgardeoClient
}

// NewApplicationDataSource is the factory function registered with the provider.
func NewApplicationDataSource() datasource.DataSource {
	return &ApplicationDataSource{}
}

// ─── Metadata ─────────────────────────────────────────────────────────────────

func (d *ApplicationDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_application"
}

// ─── Schema ───────────────────────────────────────────────────────────────────

func (d *ApplicationDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = applicationDataSourceSchema()
}

// ─── Configure ────────────────────────────────────────────────────────────────

func (d *ApplicationDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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
	d.client = client
}

// ─── Read ─────────────────────────────────────────────────────────────────────

// applicationDataSourceModel is the state model for the data source.
type applicationDataSourceModel struct {
	ID                   types.String   `tfsdk:"id"`
	Name                 types.String   `tfsdk:"name"`
	Description          types.String   `tfsdk:"description"`
	AccessURL            types.String   `tfsdk:"access_url"`
	LogoutReturnURL      types.String   `tfsdk:"logout_return_url"`
	ApplicationEnabled   types.Bool     `tfsdk:"application_enabled"`
	ClientID             types.String   `tfsdk:"client_id"`
	InboundProtocolTypes []types.String `tfsdk:"inbound_protocol_types"`
}

func (d *ApplicationDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var cfg applicationDataSourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &cfg)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Require at least one of id or name.
	if cfg.ID.IsNull() && cfg.Name.IsNull() {
		resp.Diagnostics.AddError(
			"Missing lookup key",
			"Specify at least one of `id` or `name` to look up the application.",
		)
		return
	}

	var app interface{ GetID() string } // resolved below

	if !cfg.ID.IsNull() && cfg.ID.ValueString() != "" {
		a, err := d.client.GetApplication(ctx, cfg.ID.ValueString())
		if err != nil {
			resp.Diagnostics.AddError("Error reading application", err.Error())
			return
		}
		if a == nil {
			resp.Diagnostics.AddError("Application not found", fmt.Sprintf("No application with ID %q", cfg.ID.ValueString()))
			return
		}
		_ = app
		// Flatten directly.
		state := applicationDataSourceModel{
			ID:                 types.StringValue(a.ID),
			Name:               types.StringValue(a.Name),
			Description:        types.StringValue(a.Description),
			AccessURL:          types.StringValue(a.AccessURL),
			LogoutReturnURL:    types.StringValue(a.LogoutReturnURL),
			ApplicationEnabled: types.BoolValue(a.ApplicationEnabled),
			ClientID:           types.StringValue(""),
		}
		for _, p := range a.InboundProtocols {
			state.InboundProtocolTypes = append(state.InboundProtocolTypes, types.StringValue(p.Type))
			if p.Type == "oidc" {
				oidcCfg, err := d.client.GetOIDCConfig(ctx, a.ID)
				if err == nil && oidcCfg != nil {
					state.ClientID = types.StringValue(oidcCfg.ClientID)
				}
			}
		}
		resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
		return
	}

	// Lookup by name.
	a, err := d.client.GetApplicationByName(ctx, cfg.Name.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Error listing applications", err.Error())
		return
	}
	if a == nil {
		resp.Diagnostics.AddError("Application not found", fmt.Sprintf("No application named %q", cfg.Name.ValueString()))
		return
	}

	state := applicationDataSourceModel{
		ID:                 types.StringValue(a.ID),
		Name:               types.StringValue(a.Name),
		Description:        types.StringValue(a.Description),
		AccessURL:          types.StringValue(a.AccessURL),
		LogoutReturnURL:    types.StringValue(a.LogoutReturnURL),
		ApplicationEnabled: types.BoolValue(a.ApplicationEnabled),
		ClientID:           types.StringValue(""),
	}
	for _, p := range a.InboundProtocols {
		state.InboundProtocolTypes = append(state.InboundProtocolTypes, types.StringValue(p.Type))
		if p.Type == "oidc" {
			oidcCfg, err := d.client.GetOIDCConfig(ctx, a.ID)
			if err == nil && oidcCfg != nil {
				state.ClientID = types.StringValue(oidcCfg.ClientID)
			}
		}
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}
