# git-credential-buildkite-oidc

Work in progress: there is no official exchange server for this helper yet. The exchange contract below reflects the interface we are currently building against and may still change.

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
  "/absolute/path/to/git-credential-buildkite-oidc --exchange-url=https://git.example.com/api/v0/auth/buildkite/exchange --audience=https://git.example.com --allowed-authority=git.example.com --username=buildkite-agent"
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
          exchange-url: https://git.example.com/api/v0/auth/buildkite/exchange
          audience: https://git.example.com
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

The helper makes a `POST` request to the configured exchange URL with the Buildkite OIDC token in the `Authorization: Bearer <jwt>` header.

Successful responses must include:

```json
{
  "token": "<short-lived-token>",
  "expires_in": 270,
  "expires_at": 1776740670,
  "token_type": "bearer",
  "allowed_repos": [
    "acme/widgets"
  ]
}
```

The helper uses `token` as the Git password, uses `expires_at` for caching, and requires the requested repo path to be present in `allowed_repos`. `username` defaults to `buildkite-agent` and is configured locally.

## Limitations

- HTTPS remotes only
- exactly one configured authority (`host[:port]`) per helper/plugin configuration
- request path is required for allowed requests
- cross-authority credentials are out of scope
- split checkout and command environments, including Buildkite Agent Stack for Kubernetes-style setups, are documented limitations for v1
