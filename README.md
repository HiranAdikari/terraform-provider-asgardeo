# terraform-provider-asgardeo

[![Tests](https://github.com/hiranadikari/terraform-provider-asgardeo/actions/workflows/test.yml/badge.svg)](https://github.com/hiranadikari/terraform-provider-asgardeo/actions/workflows/test.yml)

A Terraform provider for [Asgardeo](https://wso2.com/asgardeo/) — WSO2's cloud-native
customer identity and access management (CIAM) platform.

Manage Asgardeo applications (OIDC, SAML, OAuth2) as Terraform resources, enabling
full infrastructure-as-code automation of your identity federation layer.

> **Registry status:** This provider is **not yet published** to the
> [Terraform Registry](https://registry.terraform.io). You must build it from source
> (see [Installation](#installation) below). Publishing is tracked in
> [#1](https://github.com/hiranadikari/terraform-provider-asgardeo/issues/1).

## Features

- **`asgardeo_application`** resource — create, read, update, delete applications
  - OIDC / OAuth2: grant types, callback URLs, allowed origins, PKCE, token TTLs
  - SAML: manual configuration, SSO profile, single logout
  - Advanced: consent skip, SaaS flag, discoverability
- **`asgardeo_application`** data source — look up existing applications by name or ID
- Computed `client_id` and `client_secret` outputs for downstream use (e.g. Rancher OIDC)

## Requirements

| Tool | Version |
|---|---|
| [Go](https://golang.org/) | ≥ 1.21 |
| [Terraform](https://www.terraform.io/) | ≥ 1.5 |

## Installation

Because the provider is not yet on the Terraform Registry, you must build and
install it locally. This is a one-time setup step.

### 1. Build and install

```bash
git clone https://github.com/hiranadikari/terraform-provider-asgardeo
cd terraform-provider-asgardeo

# Install the binary to $GOPATH/bin (usually ~/go/bin)
go install .
```

### 2. Configure dev_overrides in ~/.terraformrc

Create or edit `~/.terraformrc`:

```hcl
provider_installation {
  dev_overrides {
    "asgardeo/asgardeo" = "/Users/<your-username>/go/bin"
    # add other local providers here if needed
  }
  direct {}
}
```

Replace `<your-username>` with your macOS/Linux username, or run
`echo $(go env GOPATH)/bin` to get the exact path.

### 3. Use the provider — skip terraform init

With `dev_overrides` active, **do not run `terraform init`** for the asgardeo
provider. Terraform loads it directly from the binary path. For any other
providers in the same configuration, lock them separately:

```bash
terraform providers lock registry.terraform.io/rancher/rancher2   # example
terraform plan  -var-file=secrets.tfvars
terraform apply -var-file=secrets.tfvars
```

### Future: Terraform Registry

Once this provider is [published to the registry](#publishing-to-the-terraform-registry),
users will be able to use it with a standard `terraform init`:

```hcl
terraform {
  required_providers {
    asgardeo = {
      source  = "asgardeo/asgardeo"
      version = "~> 0.1"
    }
  }
}
```

No local build or `dev_overrides` will be needed.

## Quick Start

```hcl
terraform {
  required_providers {
    asgardeo = {
      source  = "asgardeo/asgardeo"
      version = "~> 0.1"
    }
  }
}

provider "asgardeo" {
  org_name      = "myorg"
  client_id     = var.asgardeo_client_id
  client_secret = var.asgardeo_client_secret
}

resource "asgardeo_application" "rancher" {
  name       = "rancher-oidc"
  access_url = "https://rancher.example.com/dashboard"

  oidc {
    grant_types          = ["authorization_code", "refresh_token"]
    callback_urls        = ["https://rancher.example.com/verify-auth"]
    allowed_origins      = ["https://rancher.example.com"]
    logout_redirect_urls = ["https://rancher.example.com/dashboard/auth/logout"]

    pkce { mandatory = true }
  }

  advanced {
    skip_login_consent  = true
    skip_logout_consent = true
  }
}

output "rancher_oidc_client_id"     { value = asgardeo_application.rancher.client_id }
output "rancher_oidc_client_secret" {
  value     = asgardeo_application.rancher.client_secret
  sensitive = true
}
```

## Authentication Setup

1. In the **Asgardeo Console**, go to **Applications → New Application → M2M Application**.
2. Authorize it with these API scopes:
   - `internal_application_mgt_create`
   - `internal_application_mgt_view`
   - `internal_application_mgt_update`
   - `internal_application_mgt_delete`
   - `internal_api_resource_view` (required by `asgardeo_application_authorized_api` to resolve an API-resource identifier to its UUID)
3. Copy the **Client ID** and **Client Secret**.

Supply credentials via provider arguments or environment variables:

```bash
export ASGARDEO_ORG_NAME="myorg"
export ASGARDEO_CLIENT_ID="<m2m-client-id>"
export ASGARDEO_CLIENT_SECRET="<m2m-client-secret>"
```

## Local Development

```bash
# 1. Clone
git clone https://github.com/hiranadikari/terraform-provider-asgardeo
cd terraform-provider-asgardeo

# 2. Install tooling
make tools

# 3. Build
make build

# 4. Install to ~/go/bin (used with dev_overrides)
go install .

# 5. Run unit tests
make test

# 6. Run acceptance tests (requires real Asgardeo credentials)
export ASGARDEO_ORG_NAME=myorg
export ASGARDEO_CLIENT_ID=...
export ASGARDEO_CLIENT_SECRET=...
make testacc
```

## Examples

| Example | Description |
|---|---|
| [`examples/generic-oidc/`](examples/generic-oidc/) | Standalone OIDC application |
| [`examples/rancher-oidc/`](examples/rancher-oidc/) | Rancher Manager OIDC federation |

## Publishing to the Terraform Registry

Publishing is planned once the provider reaches a stable v0.1.0 release.
The steps are:

1. **GPG signing key** — generate a GPG key and add the public key to your
   GitHub account. GoReleaser uses it to sign release artifacts.
2. **Signed release** — push a tag (`git tag v0.1.0 && git push --tags`).
   The GitHub Actions [release workflow](.github/workflows/release.yml) will
   run GoReleaser and publish signed binaries.
3. **Registry sign-up** — go to
   [registry.terraform.io](https://registry.terraform.io), sign in with GitHub,
   and click **Publish → Provider**. Select the
   `terraform-provider-asgardeo` repo.
4. **Done** — the registry auto-imports every subsequent GitHub Release as a
   new provider version. Users can then `terraform init` without any
   `dev_overrides`.

Track progress in [#1](https://github.com/hiranadikari/terraform-provider-asgardeo/issues/1).

## Contributing

See [CONTRIBUTING.md](CONTRIBUTING.md). PRs are welcome!

## License

Apache License 2.0 — see [LICENSE](LICENSE).
