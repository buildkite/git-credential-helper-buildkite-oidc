#!/usr/bin/env bats

load test_helper.bash

setup() {
  setup_plugin_test_env
}

teardown() {
  teardown_plugin_test_env
}

@test "pre-exit removes the current job cache directory" {
  cache_dir="${TEST_TMPDIR}/cache/${BUILDKITE_JOB_ID}"
  mkdir -p "$cache_dir"
  printf 'cached' > "$cache_dir/entry.json"

  export BUILDKITE_PLUGIN_GIT_CREDENTIAL_BUILDKITE_OIDC_CACHE_DIR="${TEST_TMPDIR}/cache"

  run bash "$REPO_ROOT/plugin/hooks/pre-exit"

  [ "$status" -eq 0 ]
  [ ! -e "$cache_dir" ]
}
