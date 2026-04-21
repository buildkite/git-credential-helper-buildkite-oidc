# Buildkite Plugin

This plugin configures `git-credential-buildkite-oidc` before checkout so Git can fetch an HTTPS repository with short-lived credentials derived from Buildkite OIDC.

The plugin uses two non-vendored hooks:

- `environment` validates `BUILDKITE_REPO`, derives the deterministic helper path, and injects URL-scoped Git config with `GIT_CONFIG_COUNT`
- `pre-checkout` validates or installs the helper binary before clone starts

## Requirements

- Buildkite Agent `v3.108.0+` when consuming this repository as a subdirectory plugin
- HTTPS `BUILDKITE_REPO`
- exact `authority` match against `BUILDKITE_REPO`
- one configured authority per plugin instance

## Example

`exchange-url` and `audience` are intentionally deployment-specific. Replace the example values with the endpoint and Buildkite OIDC audience used by your own token-exchange service.

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

## Modes

### Download Mode

Download mode is the default. It requires a pinned `version`, downloads the matching GitHub Release artifact, verifies the checksum from `checksums.txt`, and installs the helper to a deterministic absolute path under `download-dir`.

### Preinstalled Mode

Set `binary-path` to an existing absolute or relative helper path when the binary is already baked into the agent image or host.

## Notes

- SSH remotes are unsupported and fail fast.
- The plugin does not override checkout in v1.
- `cache-dir` controls credential cache storage. `download-dir` controls release artifact storage.
- Split checkout and command environments are a documented limitation for v1.
