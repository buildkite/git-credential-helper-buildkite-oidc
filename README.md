# git-credential-buildkite-oidc

`git-credential-buildkite-oidc` is a Git credential helper for HTTPS checkouts inside Buildkite jobs.

It requests a Buildkite OIDC token from the agent API, exchanges that JWT for short-lived Git credentials, and returns them to Git through the standard credential-helper protocol.

The repository also ships a Buildkite plugin that installs the helper before checkout and configures Git to use it.

## Install

Build from source with `mise run build`, install with Go, or download an archive from GitHub Releases.

```sh
go install github.com/buildkite/git-credential-helper-buildkite-oidc/cmd/git-credential-buildkite-oidc@latest
```

## Helper

The helper supports one HTTPS authority per configuration.

```sh
git config credential."https://git.example.com".helper \
  "/absolute/path/to/git-credential-buildkite-oidc --exchange-url=https://token-exchange.example.com/api/git-credentials/exchange --audience=https://token-exchange.example.com --allowed-authority=git.example.com --username=buildkite-agent"
git config credential."https://git.example.com".useHttpPath true
git config credential."https://git.example.com".interactive false
```

Required environment variables:

- `BUILDKITE_AGENT_ACCESS_TOKEN`
- `BUILDKITE_JOB_ID`

Optional environment variables:

- `BUILDKITE_AGENT_ENDPOINT`, which defaults to `https://agent-edge.buildkite.com/v3`

`get` validates that the Git request is HTTPS, matches the configured authority, and includes a path. `store` is a no-op. `erase` removes the matching cache entry.

## Plugin

The plugin lives in [`plugin/`](plugin/) and can be used as a subdirectory plugin on Buildkite Agent `v3.108.0+`.

```yaml
steps:
  - command: git remote -v
    plugins:
      - github.com/buildkite/git-credential-helper-buildkite-oidc/plugin#vX.Y.Z:
          exchange-url: https://token-exchange.example.com/api/git-credentials/exchange
          audience: https://token-exchange.example.com
          authority: git.example.com
          version: vX.Y.Z
```

The plugin:

- validates that `BUILDKITE_REPO` is an HTTPS URL for the configured authority
- injects URL-scoped Git config with `GIT_CONFIG_COUNT`
- installs the helper before checkout
- removes the per-job cache directory in `pre-exit`

Download mode is the default. It requires a pinned release tag and verifies the published checksum before installing the helper.

## Token Exchange Contract

The helper makes a token exchange request with the Buildkite OIDC token in the `Authorization: Bearer <jwt>` header and this JSON body:

```json
{
  "protocol": "https",
  "authority": "git.example.com",
  "path": "acme/widgets.git"
}
```

Successful responses must include:

```json
{
  "password": "<short-lived-token>",
  "password_expiry_utc": 1776744306
}
```

The helper returns `username`, `password`, and `password_expiry_utc` to Git. `username` defaults to `buildkite-agent` and is configured locally, not returned by the exchange service.

## Limitations

- HTTPS remotes only
- exactly one configured authority (`host[:port]`) per helper/plugin configuration
- request path is required for allowed requests
- cross-authority credentials are out of scope
- split checkout and command environments, including Buildkite Agent Stack for Kubernetes-style setups, are documented limitations for v1
