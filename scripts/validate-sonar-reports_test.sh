#!/usr/bin/env bash
set -euo pipefail

script_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
fixture="$(mktemp -d)"
trap 'rm -rf "$fixture"' EXIT

make_valid() {
  local root=$1
  mkdir -p "$root"
  # upload-artifact strips the common .coverage parent from the two selected
  # files, so the downloaded artifact contract has exactly these root entries.
  printf 'mode: set\n' >"$root/out"
  printf '<testsuite/>\n' >"$root/report-junit.xml"
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
if [[ -e "$valid/out" || -e "$valid/report-junit.xml" ||
  ! -f "$valid/.coverage/out" || ! -f "$valid/.coverage/report-junit.xml" ||
  $(stat -c '%a' "$valid/.coverage/out") != 644 ||
  $(stat -c '%a' "$valid/.coverage/report-junit.xml") != 644 ]]; then
  printf 'validated reports were not normalized to protected scanner paths\n' >&2
  exit 1
fi

missing_coverage="$fixture/missing-coverage"
mkdir -p "$missing_coverage"
printf '<testsuite/>\n' >"$missing_coverage/report-junit.xml"
expect_reject 'missing coverage' "$missing_coverage"

symlink="$fixture/symlink"
make_valid "$symlink"
rm "$symlink/out"
ln -s /etc/passwd "$symlink/out"
expect_reject 'expected-file symlink' "$symlink"

symlink_sibling="$fixture/symlink-sibling"
make_valid "$symlink_sibling"
ln -s /etc/passwd "$symlink_sibling/unexpected-link"
expect_reject 'symlink sibling' "$symlink_sibling"

special="$fixture/special"
make_valid "$special"
mkfifo "$special/report.pipe"
expect_reject 'special file' "$special"

directory="$fixture/directory"
make_valid "$directory"
mkdir "$directory/unexpected"
expect_reject 'unexpected directory' "$directory"

unexpected="$fixture/unexpected"
make_valid "$unexpected"
printf 'not a report\n' >"$unexpected/extra.txt"
expect_reject 'unexpected content' "$unexpected"

# An archive traversal payload would materialize content outside its declared
# report path. The validator rejects that materialized unexpected path.
traversal="$fixture/traversal"
make_valid "$traversal"
printf 'escape attempt\n' >"$traversal/escaped-from-artifact"
expect_reject 'traversal-derived path' "$traversal"

oversized="$fixture/oversized"
make_valid "$oversized"
truncate -s $((50 * 1024 * 1024 + 1)) "$oversized/out"
expect_reject 'oversized coverage report' "$oversized"

oversized_junit="$fixture/oversized-junit"
make_valid "$oversized_junit"
truncate -s $((10 * 1024 * 1024 + 1)) "$oversized_junit/report-junit.xml"
expect_reject 'oversized JUnit report' "$oversized_junit"

printf 'Sonar report artifact contract negative fixtures passed\n'
