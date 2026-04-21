setup_plugin_test_env() {
  export TEST_TMPDIR
  TEST_TMPDIR="$(mktemp -d)"
  export REPO_ROOT
  REPO_ROOT="$(cd "$(dirname "$BATS_TEST_FILENAME")/../.." && pwd)"

  export BUILDKITE_REPO="https://git.example.com/acme/widgets.git"
  export BUILDKITE_PLUGIN_GIT_CREDENTIAL_BUILDKITE_OIDC_EXCHANGE_URL="https://token-exchange.example.com/api/git-credentials/exchange"
  export BUILDKITE_PLUGIN_GIT_CREDENTIAL_BUILDKITE_OIDC_AUDIENCE="https://token-exchange.example.com"
  export BUILDKITE_PLUGIN_GIT_CREDENTIAL_BUILDKITE_OIDC_AUTHORITY="git.example.com"
  export BUILDKITE_PLUGIN_GIT_CREDENTIAL_BUILDKITE_OIDC_VERSION="v0.0.1"

  unset BUILDKITE_PLUGIN_GIT_CREDENTIAL_BUILDKITE_OIDC_BINARY_PATH
  unset BUILDKITE_PLUGIN_GIT_CREDENTIAL_BUILDKITE_OIDC_DOWNLOAD_DIR
  unset BUILDKITE_PLUGIN_GIT_CREDENTIAL_BUILDKITE_OIDC_CACHE_DIR
  unset BUILDKITE_PLUGIN_GIT_CREDENTIAL_BUILDKITE_OIDC_USERNAME
  unset BUILDKITE_PLUGIN_GIT_CREDENTIAL_BUILDKITE_OIDC_OIDC_LIFETIME
  unset GIT_CONFIG_COUNT
  unset GIT_CONFIG_KEY_0
  unset GIT_CONFIG_VALUE_0
  unset GIT_CONFIG_KEY_1
  unset GIT_CONFIG_VALUE_1
  unset GIT_CONFIG_KEY_2
  unset GIT_CONFIG_VALUE_2
  unset GIT_CONFIG_KEY_3
  unset GIT_CONFIG_VALUE_3
  unset GIT_TERMINAL_PROMPT
}

teardown_plugin_test_env() {
  rm -rf "${TEST_TMPDIR}"
}

plugin_helper_path() {
  bash -c '
    set -euo pipefail
    source "$REPO_ROOT/plugin/lib/shared.bash"
    helper_path
  '
}

setup_download_mocks() {
  export TEST_MOCK_BIN="${TEST_TMPDIR}/mock-bin"
  mkdir -p "${TEST_MOCK_BIN}"
  export PATH="${TEST_MOCK_BIN}:${PATH}"
  export MOCK_ARCHIVE_CHECKSUM="0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"

  cat > "${TEST_MOCK_BIN}/curl" <<'EOF'
#!/usr/bin/env bash
set -euo pipefail

url=""
output=""
while [[ $# -gt 0 ]]; do
  case "$1" in
    -o)
      output="$2"
      shift 2
      ;;
    -*)
      shift
      ;;
    *)
      url="$1"
      shift
      ;;
  esac
done

if [[ -z "$output" || -z "$url" ]]; then
  exit 1
fi

if [[ "$url" == *"checksums.txt" ]]; then
  archive_name=${MOCK_ARCHIVE_NAME:?}
  printf '%s  %s\n' "${MOCK_ARCHIVE_CHECKSUM:?}" "$archive_name" > "$output"
  exit 0
fi

printf 'mock archive' > "$output"
EOF

  cat > "${TEST_MOCK_BIN}/shasum" <<'EOF'
#!/usr/bin/env bash
set -euo pipefail

file=${@: -1}
printf '%s  %s\n' "${MOCK_ARCHIVE_CHECKSUM:?}" "$file"
EOF

  cat > "${TEST_MOCK_BIN}/tar" <<'EOF'
#!/usr/bin/env bash
set -euo pipefail

dest=""
while [[ $# -gt 0 ]]; do
  case "$1" in
    -C)
      dest="$2"
      shift 2
      ;;
    *)
      shift
      ;;
  esac
done

mkdir -p "$dest"
cat > "$dest/git-credential-buildkite-oidc" <<'INNER'
#!/usr/bin/env bash
set -euo pipefail
echo helper
INNER
chmod +x "$dest/git-credential-buildkite-oidc"
EOF

  chmod +x "${TEST_MOCK_BIN}/curl" "${TEST_MOCK_BIN}/shasum" "${TEST_MOCK_BIN}/tar"
}
