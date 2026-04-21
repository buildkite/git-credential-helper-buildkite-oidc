#!/usr/bin/env bash

readonly BK_PLUGIN_PREFIX="BUILDKITE_PLUGIN_GIT_CREDENTIAL_BUILDKITE_OIDC"

fail() {
  echo "git-credential-buildkite-oidc plugin: $*" >&2
  exit 1
}

plugin_value() {
  local key
  key=${1//-/_}
  key=${key^^}

  local env_name="${BK_PLUGIN_PREFIX}_${key}"
  printf '%s' "${!env_name:-}"
}

required_plugin_value() {
  local value
  value=$(plugin_value "$1")
  [[ -n "$value" ]] || fail "missing required option: $1"
  printf '%s' "$value"
}

plugin_username() {
  local value
  value=$(plugin_value "username")
  if [[ -n "$value" ]]; then
    printf '%s' "$value"
    return
  fi
  printf 'buildkite-agent'
}

resolve_absolute_path() {
  local path=$1
  if [[ "$path" = /* ]]; then
    printf '%s' "$path"
    return
  fi
  printf '%s/%s' "$(pwd)" "$path"
}

default_download_dir() {
  local base=${HOME:-${TMPDIR:-/tmp}}
  printf '%s/.cache/git-credential-buildkite-oidc/downloads' "$base"
}

platform_os() {
  case "$(uname -s)" in
    Linux) printf 'linux' ;;
    Darwin) printf 'darwin' ;;
    *) fail "unsupported operating system: $(uname -s)" ;;
  esac
}

platform_arch() {
  case "$(uname -m)" in
    x86_64) printf 'amd64' ;;
    arm64|aarch64) printf 'arm64' ;;
    *) fail "unsupported architecture: $(uname -m)" ;;
  esac
}

helper_path() {
  local binary_path
  binary_path=$(plugin_value "binary-path")
  if [[ -n "$binary_path" ]]; then
    resolve_absolute_path "$binary_path"
    return
  fi

  local version
  version=$(plugin_value "version")
  [[ -n "$version" ]] || fail "version is required when binary-path is not set"

  local download_dir
  download_dir=$(plugin_value "download-dir")
  if [[ -z "$download_dir" ]]; then
    download_dir=$(default_download_dir)
  fi
  download_dir=$(resolve_absolute_path "$download_dir")

  printf '%s/%s/%s-%s/git-credential-buildkite-oidc' \
    "$download_dir" \
    "$version" \
    "$(platform_os)" \
    "$(platform_arch)"
}

helper_command() {
  local helper_binary=$1
  local exchange_url audience authority username cache_dir oidc_lifetime
  local -a args

  exchange_url=$(required_plugin_value "exchange-url")
  audience=$(required_plugin_value "audience")
  authority=$(required_plugin_value "authority")
  username=$(plugin_username)

  args=(
    "$helper_binary"
    "--exchange-url=$exchange_url"
    "--audience=$audience"
    "--allowed-authority=$authority"
    "--username=$username"
  )

  cache_dir=$(plugin_value "cache-dir")
  if [[ -n "$cache_dir" ]]; then
    args+=("--cache-dir=$(resolve_absolute_path "$cache_dir")")
  fi

  oidc_lifetime=$(plugin_value "oidc-lifetime")
  if [[ -n "$oidc_lifetime" ]]; then
    args+=("--oidc-lifetime=$oidc_lifetime")
  fi

  local command=""
  local arg
  for arg in "${args[@]}"; do
    command+="$(printf '%q ' "$arg")"
  done

  printf '%s' "${command% }"
}

append_git_config() {
  local key=$1
  local value=$2
  local index=${GIT_CONFIG_COUNT:-0}

  export GIT_CONFIG_COUNT=$((index + 1))
  export "GIT_CONFIG_KEY_${index}=${key}"
  export "GIT_CONFIG_VALUE_${index}=${value}"
}

extract_https_authority() {
  local repo_url=$1

  if [[ "$repo_url" =~ ^https://([^/]+)(/.*)?$ ]]; then
    printf '%s' "${BASH_REMATCH[1]}"
    return 0
  fi

  return 1
}

validate_repo_authority() {
  local configured_authority=$1
  local repo_url=${BUILDKITE_REPO:-}

  [[ -n "$repo_url" ]] || fail "BUILDKITE_REPO is not set"

  local repo_authority
  if ! repo_authority=$(extract_https_authority "$repo_url"); then
    fail "BUILDKITE_REPO must be an HTTPS URL for checkout-time token exchange auth"
  fi

  if [[ "$repo_authority" != "$configured_authority" ]]; then
    fail "configured authority '$configured_authority' does not match BUILDKITE_REPO authority '$repo_authority'"
  fi
}

release_base_url() {
  local version
  version=$(required_plugin_value "version")
  printf 'https://github.com/buildkite/git-credential-helper-buildkite-oidc/releases/download/%s' "$version"
}

archive_name() {
  local version
  version=$(required_plugin_value "version")
  printf 'git-credential-buildkite-oidc_%s_%s_%s.tar.gz' "$version" "$(platform_os)" "$(platform_arch)"
}

checksum_name() {
  printf 'checksums.txt'
}

sha256_file() {
  local file=$1
  if command -v shasum >/dev/null 2>&1; then
    shasum -a 256 "$file" | awk '{print $1}'
    return
  fi
  if command -v sha256sum >/dev/null 2>&1; then
    sha256sum "$file" | awk '{print $1}'
    return
  fi
  fail "no sha256 checksum tool found"
}
