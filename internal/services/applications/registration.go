// Package applications registers the asgardeo_application resource and data source.
// To add a new application-related resource or data source, add its factory function
// to Resources() or DataSources() respectively — the provider picks them up automatically.
package applications

import (
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/resource"
)

// Resources returns the factory functions for all managed resources in this service.
func Resources() []func() resource.Resource {
	return []func() resource.Resource{
		NewApplicationResource,
		NewAuthorizedAPIResource,
	}
}

// DataSources returns the factory functions for all data sources in this service.
func DataSources() []func() datasource.DataSource {
	return []func() datasource.DataSource{
		NewApplicationDataSource,
	}
}
