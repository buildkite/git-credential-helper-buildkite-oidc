# Buildkite Plugin

This plugin configures `git-credential-buildkite-oidc` before checkout so Git can fetch an HTTPS repository with short-lived credentials derived from Buildkite OIDC.

It uses three non-vendored hooks:

- `environment` validates `BUILDKITE_REPO`, derives the deterministic helper path, and injects URL-scoped Git config with `GIT_CONFIG_COUNT`
- `pre-checkout` validates or installs the helper binary before clone starts
- `pre-exit` removes the per-job helper cache directory as best-effort cleanup

## Requirements

- Buildkite Agent `v3.108.0+` when consuming this repository as a subdirectory plugin
- HTTPS `BUILDKITE_REPO`
- `authority` must match `BUILDKITE_REPO` case-insensitively
- one configured authority per plugin instance

## Example

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

## Modes

Download mode is the default. It requires a pinned release tag, downloads the matching release artifact, verifies `checksums.txt`, and installs the helper under `download-dir`.

Preinstalled mode uses `binary-path` instead.

## Notes

- SSH remotes are unsupported and fail fast.
- The plugin does not override checkout in v1.
- `cache-dir` controls credential cache storage.
- `download-dir` controls where downloaded release artifacts are installed.
- Split checkout and command environments are a documented limitation for v1.
