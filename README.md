# git-credential-buildkite-oidc

`git-credential-buildkite-oidc` is a small Go credential helper for HTTPS Git access inside Buildkite jobs.

It requests a Buildkite OIDC token directly from the Buildkite agent API, exchanges that JWT with a configured token-exchange endpoint for short-lived Git credentials, and returns those credentials to Git through the standard credential-helper protocol.

The repository also ships a Buildkite plugin that wires the helper into checkout-time Git configuration before repository code exists locally.

## What It Does

1. Git invokes `git-credential-buildkite-oidc get` with repository context on stdin.
2. The helper validates that the request is HTTPS, matches one configured authority exactly, and includes a path.
3. The helper reuses a short-lived per-job cached credential when possible.
4. On cache miss, the helper requests a Buildkite OIDC token from the agent API.
5. The helper exchanges that JWT for short-lived HTTPS Git credentials.
6. The helper returns `username`, `password`, and `password_expiry_utc` to Git.

## Install

### mise

```toml
[tools]
"github:buildkite/git-credential-helper-buildkite-oidc" = "latest"
```

For local development:

```sh
export MISE_GITHUB_TOKEN=$(gh auth token)
mise install
```

### Go

```sh
go install github.com/buildkite/git-credential-helper-buildkite-oidc/cmd/git-credential-buildkite-oidc@latest
```

### GitHub Releases

Download a binary from the repository's GitHub Releases page.

## Helper Usage

The helper is intended to be configured for one HTTPS authority at a time.

`--exchange-url` and `--audience` are deployment-specific values. Replace the example values below with the token-exchange endpoint and Buildkite OIDC audience that your exchange service expects.

```sh
git config credential."https://git.example.com".helper \
  "/absolute/path/to/git-credential-buildkite-oidc --exchange-url=https://token-exchange.example.com/api/git-credentials/exchange --audience=https://token-exchange.example.com --allowed-authority=git.example.com --username=buildkite-agent"
git config credential."https://git.example.com".useHttpPath true
git config credential."https://git.example.com".interactive false
```

Required environment variables for `get`:

- `BUILDKITE_AGENT_ACCESS_TOKEN`
- `BUILDKITE_JOB_ID`
- `BUILDKITE_AGENT_ENDPOINT` is optional and defaults to `https://agent-edge.buildkite.com/v3`

The helper only supports HTTPS credential requests in v1. SSH remotes, pathless requests, and authority mismatches fail fast with stderr output and a non-zero exit code.

## Plugin Usage

The plugin lives in [`plugin/`](plugin/) and can be used as a subdirectory plugin on Buildkite Agent `v3.108.0+`.

Like the helper flags, `exchange-url` and `audience` must be set to the values for your own token-exchange deployment.

```yaml
steps:
  - command: git remote -v
    plugins:
      - github.com/buildkite/git-credential-helper-buildkite-oidc/plugin#v0.0.1:
          exchange-url: https://token-exchange.example.com/api/git-credentials/exchange
          audience: https://token-exchange.example.com
          authority: git.example.com
          version: v0.0.1
```

If you do not want deployment-specific values checked into a pipeline definition, keep the plugin configuration in a generated pipeline or another internal configuration layer and pass the final strings into the plugin there.

The plugin:

- validates that `BUILDKITE_REPO` is HTTPS and matches the configured authority exactly
- injects URL-scoped credential config using `GIT_CONFIG_COUNT`
- ensures the helper binary exists before checkout

Download mode requires a pinned release version and verifies the published checksum before installing the helper binary.

## Exchange Contract

The helper sends a token-exchange request like:

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

The helper treats `password_expiry_utc` as authoritative for caching. Missing expiry is an error.

## Limitations

- HTTPS remotes only
- exactly one configured authority (`host[:port]`) per helper/plugin configuration
- request path is required for allowed requests
- cross-authority credentials are out of scope
- split checkout and command environments, including Buildkite Agent Stack for Kubernetes-style setups, are documented limitations for v1
