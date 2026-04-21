# Repository Guidance

## Core Commands

- `mise run build` builds the helper binary.
- `mise run test` runs the Go tests and the plugin Bats tests.
- `mise run lint` runs `golangci-lint`.
- `mise run vet` runs `go vet`.
- `mise run format` auto-fixes lint issues via `golangci-lint --fix`.

## Releasing

- `GITHUB_TOKEN=$(gh auth token) mise exec -- goreleaser release --clean` cuts a release from the current tag.

## Notes

- Prefer the `mise` tasks above instead of invoking individual tools directly when verifying changes.
- The plugin test suite depends on `bats`, which is installed via `mise`.
