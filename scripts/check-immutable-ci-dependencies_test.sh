#!/usr/bin/env bash
set -euo pipefail

script_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
fixture="$(mktemp -d)"
trap 'rm -rf "$fixture"' EXIT
mkdir -p "$fixture/shared/build/go" "$fixture/.github/workflows"

cat >"$fixture/eirctl.yaml" <<'YAML'
contexts:
  bash:
    container:
      name: example.invalid/bash:1.0@sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa
YAML
cat >"$fixture/shared/build/go/eirctl.yaml" <<'YAML'
contexts:
  go1x:
    container:
      name: example.invalid/go:1.0@sha256:bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb
tasks:
  go:vuln:check:
    command: go install golang.org/x/vuln/cmd/govulncheck@v1.6.0
YAML
cat >"$fixture/.github/workflows/test.yml" <<'YAML'
steps:
  - with:
      versionSpec: '6.8.2'
YAML

run_check() {
  (cd "$fixture" && "$script_dir/check-immutable-ci-dependencies.sh")
}

run_check
for mutation in image gitversion govulncheck; do
  cp "$fixture/eirctl.yaml" "$fixture/eirctl.yaml.bak"
  cp "$fixture/shared/build/go/eirctl.yaml" "$fixture/shared/build/go/eirctl.yaml.bak"
  cp "$fixture/.github/workflows/test.yml" "$fixture/.github/workflows/test.yml.bak"
  case "$mutation" in
    image) sed -i 's/@sha256:[a-f0-9]*/ /' "$fixture/eirctl.yaml" ;;
    gitversion) sed -i "s/6.8.2/5.x/" "$fixture/.github/workflows/test.yml" ;;
    govulncheck) sed -i 's/@v1.6.0/@latest/' "$fixture/shared/build/go/eirctl.yaml" ;;
  esac
  if run_check >/dev/null 2>&1; then
    printf 'expected %s mutation to fail validation\n' "$mutation" >&2
    exit 1
  fi
  mv "$fixture/eirctl.yaml.bak" "$fixture/eirctl.yaml"
  mv "$fixture/shared/build/go/eirctl.yaml.bak" "$fixture/shared/build/go/eirctl.yaml"
  mv "$fixture/.github/workflows/test.yml.bak" "$fixture/.github/workflows/test.yml"
done

printf 'immutable CI dependency negative fixtures passed\n'
