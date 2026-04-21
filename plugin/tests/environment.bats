#!/usr/bin/env bats

load test_helper.bash

setup() {
  setup_plugin_test_env
}

teardown() {
  teardown_plugin_test_env
}

@test "environment injects scoped git credential config" {
  run bash -c '. "$REPO_ROOT/plugin/hooks/environment" >/dev/null; env | sort'

  [ "$status" -eq 0 ]
  [[ "$output" == *"GIT_TERMINAL_PROMPT=0"* ]]
  [[ "$output" == *"GIT_CONFIG_COUNT=3"* ]]
  [[ "$output" == *"GIT_CONFIG_KEY_0=credential.https://git.example.com.helper"* ]]
  [[ "$output" == *"GIT_CONFIG_KEY_1=credential.https://git.example.com.useHttpPath"* ]]
  [[ "$output" == *"GIT_CONFIG_KEY_2=credential.https://git.example.com.interactive"* ]]
  [[ "$output" == *"--exchange-url=https://token-exchange.example.com/api/git-credentials/exchange"* ]]
  [[ "$output" == *"--audience=https://token-exchange.example.com"* ]]
  [[ "$output" == *"--allowed-authority=git.example.com"* ]]
}

@test "environment appends to existing git config" {
  run bash -c '
    export GIT_CONFIG_COUNT=1
    export GIT_CONFIG_KEY_0=credential.helper
    export GIT_CONFIG_VALUE_0=existing-helper
    . "$REPO_ROOT/plugin/hooks/environment" >/dev/null
    env | sort
  '

  [ "$status" -eq 0 ]
  [[ "$output" == *"GIT_CONFIG_COUNT=4"* ]]
  [[ "$output" == *"GIT_CONFIG_KEY_0=credential.helper"* ]]
  [[ "$output" == *"GIT_CONFIG_VALUE_0=existing-helper"* ]]
  [[ "$output" == *"GIT_CONFIG_KEY_1=credential.https://git.example.com.helper"* ]]
}

@test "environment fails fast for non-https repos" {
  export BUILDKITE_REPO="git@git.example.com:acme/widgets.git"

  run bash "$REPO_ROOT/plugin/hooks/environment"

  [ "$status" -ne 0 ]
  [[ "$output" == *"BUILDKITE_REPO must be an HTTPS URL"* ]]
}

@test "environment matches authority case-insensitively" {
  export BUILDKITE_REPO="https://Git.EXAMPLE.com/acme/widgets.git"

  run bash -c '. "$REPO_ROOT/plugin/hooks/environment" >/dev/null; env | sort'

  [ "$status" -eq 0 ]
  [[ "$output" == *"GIT_CONFIG_KEY_0=credential.https://git.example.com.helper"* ]]
}
