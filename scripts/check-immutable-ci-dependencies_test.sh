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
  coverage:cobertura:
    command: go install github.com/boumenot/gocover-cobertura@v1.5.0
YAML
cat >"$fixture/.github/workflows/test.yml" <<'YAML'
steps:
  - with:
      versionSpec: '6.0.5'
YAML
run_check() {
  (cd "$fixture" && "$script_dir/check-immutable-ci-dependencies.sh")
}

run_check
for mutation in image gitversion govulncheck coverage-converter; do
  backup="$(mktemp -d)"
  cp -R "$fixture/." "$backup/"
  case "$mutation" in
    image) sed -i 's/@sha256:[a-f0-9]*/ /' "$fixture/eirctl.yaml" ;;
    gitversion) sed -i "s/6.0.5/5.x/" "$fixture/.github/workflows/test.yml" ;;
    govulncheck) sed -i 's/@v1.6.0/@latest/' "$fixture/shared/build/go/eirctl.yaml" ;;
    coverage-converter) sed -i 's/gocover-cobertura@v1.5.0/gocover-cobertura@latest/' "$fixture/shared/build/go/eirctl.yaml" ;;
  esac
  if run_check >/dev/null 2>&1; then
    printf 'expected %s mutation to fail validation\n' "$mutation" >&2
    exit 1
  fi
  rm -rf "$fixture"
  mkdir -p "$fixture"
  cp -R "$backup/." "$fixture/"
  rm -rf "$backup"
done

printf 'immutable CI dependency negative fixtures passed\n'
