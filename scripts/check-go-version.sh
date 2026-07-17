#!/usr/bin/env bash
set -euo pipefail

version=1.26.5
language_version=1.26

fail() {
  printf 'Go version check failed: %s\n' "$*" >&2
  exit 1
}

# Check each authoritative location rather than searching for a version anywhere
# in a file. Historical fixture versions are intentionally outside this list.
go_language=$(awk '$1 == "go" { print $2; exit }' go.mod)
[[ "$go_language" == "$language_version" ]] || fail "go.mod language version is $go_language, want $language_version"
go_toolchain=$(awk '$1 == "toolchain" { print $2; exit }' go.mod)
[[ "$go_toolchain" == "go$version" ]] || fail "go.mod toolchain is $go_toolchain, want go$version"

# The optional digest makes the image immutable without changing the required Go release.
grep -Eq "^FROM docker\.io/golang:${version}-trixie(@sha256:[0-9a-f]{64})? AS builder$" Dockerfile || fail "Dockerfile builder image is not golang:${version}-trixie"
grep -Eq "^FROM docker\.io/golang:${version}-trixie(@sha256:[0-9a-f]{64})? AS builder$" Dockerfile.pwsh || fail "Dockerfile.pwsh builder image is not golang:${version}-trixie"
grep -Eq "^[[:space:]]+name: mirror\.gcr\.io/golang:${version}-trixie$" shared/build/go/eirctl.yaml || fail "shared Go build context is not golang:${version}-trixie"
grep -Fq "Maintained local builds and validation require **Go ${version}**." README.md || fail "README prerequisite does not name Go ${version}"

workflow_count=0
while IFS= read -r -d '' file; do
  while IFS= read -r line; do
    value=${line#*:}
    value=${value%%#*}
    value=${value//[[:space:]]/}
    value=${value#\"}
    value=${value%\"}
    value=${value#\'}
    value=${value%\'}
    [[ "$value" == "$version" ]] || fail "$file has non-exact maintained go-version selector $value"
    workflow_count=$((workflow_count + 1))
  done < <(grep -E '^[[:space:]]+go-version[[:space:]]*:' "$file" || true)
done < <(find .github/workflows -type f \( -name '*.yml' -o -name '*.yaml' \) -print0)

(( workflow_count > 0 )) || fail 'no maintained workflow go-version selectors found'
printf 'all maintained Go pins use exact Go %s\n' "$version"
