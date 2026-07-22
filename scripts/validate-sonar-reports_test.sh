#!/usr/bin/env bash
set -euo pipefail

script_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
fixture="$(mktemp -d)"
trap 'rm -rf "$fixture"' EXIT

make_valid() {
  local root=$1
  mkdir -p "$root/.coverage"
  printf 'mode: set\n' >"$root/.coverage/out"
  printf '<testsuite/>\n' >"$root/.coverage/report-junit.xml"
}

expect_reject() {
  local name=$1
  shift
  if "$script_dir/validate-sonar-reports.sh" "$@" >/dev/null 2>&1; then
    printf 'expected %s fixture to be rejected\n' "$name" >&2
    exit 1
  fi
}

valid="$fixture/valid"
make_valid "$valid"
"$script_dir/validate-sonar-reports.sh" "$valid"

missing_coverage="$fixture/missing-coverage"
mkdir -p "$missing_coverage/.coverage"
printf '<testsuite/>\n' >"$missing_coverage/.coverage/report-junit.xml"
expect_reject 'missing coverage' "$missing_coverage"

symlink="$fixture/symlink"
make_valid "$symlink"
rm "$symlink/.coverage/out"
ln -s /etc/passwd "$symlink/.coverage/out"
expect_reject 'symlink' "$symlink"

special="$fixture/special"
make_valid "$special"
mkfifo "$special/.coverage/report.pipe"
expect_reject 'special file' "$special"

unexpected="$fixture/unexpected"
make_valid "$unexpected"
printf 'not a report\n' >"$unexpected/.coverage/extra.txt"
expect_reject 'unexpected content' "$unexpected"

# An archive traversal payload would materialize content outside its declared
# report path. The validator rejects that materialized unexpected path.
traversal="$fixture/traversal"
make_valid "$traversal"
printf 'escape attempt\n' >"$traversal/escaped-from-artifact"
expect_reject 'traversal-derived path' "$traversal"

oversized="$fixture/oversized"
make_valid "$oversized"
truncate -s $((50 * 1024 * 1024 + 1)) "$oversized/.coverage/out"
expect_reject 'oversized coverage report' "$oversized"

printf 'Sonar report artifact contract negative fixtures passed\n'
