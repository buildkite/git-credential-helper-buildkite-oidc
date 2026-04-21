#!/usr/bin/env bats

load test_helper.bash

setup() {
  setup_plugin_test_env
}

teardown() {
  teardown_plugin_test_env
}

@test "pre-checkout accepts a preinstalled helper binary" {
  helper_binary="${TEST_TMPDIR}/bin/git-credential-buildkite-oidc"
  mkdir -p "$(dirname "$helper_binary")"
  printf '#!/usr/bin/env bash\nexit 0\n' > "$helper_binary"
  chmod +x "$helper_binary"
  export BUILDKITE_PLUGIN_GIT_CREDENTIAL_BUILDKITE_OIDC_BINARY_PATH="$helper_binary"

  run bash "$REPO_ROOT/plugin/hooks/pre-checkout"

  [ "$status" -eq 0 ]
}

@test "pre-checkout downloads and installs the helper release artifact" {
  export BUILDKITE_PLUGIN_GIT_CREDENTIAL_BUILDKITE_OIDC_VERSION="v0.0.1"
  export BUILDKITE_PLUGIN_GIT_CREDENTIAL_BUILDKITE_OIDC_DOWNLOAD_DIR="${TEST_TMPDIR}/downloads"
  export MOCK_ARCHIVE_NAME="git-credential-buildkite-oidc_0.0.1_$(bash -c 'source "$REPO_ROOT/plugin/lib/shared.bash" && platform_os')_$(bash -c 'source "$REPO_ROOT/plugin/lib/shared.bash" && platform_arch').tar.gz"
  setup_download_mocks

  helper_binary=$(plugin_helper_path)

  run bash "$REPO_ROOT/plugin/hooks/pre-checkout"

  [ "$status" -eq 0 ]
  [ -x "$helper_binary" ]
}
