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
  cat >"$root/out" <<'COVERAGE'
mode: atomic
internal/example.go:10.2,12.3 2 1
COVERAGE
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
  $(stat -c '%a' "$valid/.coverage/report-junit.xml") != 644 ||
  $(sed -n '1p' "$valid/.coverage/out") != 'mode: atomic' ||
  $(sed -n '2p' "$valid/.coverage/out") != 'source/internal/example.go:10.2,12.3 2 1' ]]; then
  printf 'validated reports were not normalized to the protected scanner namespace and paths\n' >&2
  exit 1
fi

empty_coverage="$fixture/empty-coverage"
make_valid "$empty_coverage"
printf 'mode: atomic\n' >"$empty_coverage/out"
expect_reject 'coverage without records' "$empty_coverage"

invalid_mode="$fixture/invalid-mode"
make_valid "$invalid_mode"
sed -i '1s/.*/mode: attacker/' "$invalid_mode/out"
expect_reject 'invalid coverage mode' "$invalid_mode"

malformed_record="$fixture/malformed-record"
make_valid "$malformed_record"
printf 'not-a-coverage-record\n' >>"$malformed_record/out"
expect_reject 'malformed coverage record' "$malformed_record"

unsafe_coverage_path="$fixture/unsafe-coverage-path"
make_valid "$unsafe_coverage_path"
sed -i '2s|internal/example.go|../example.go|' "$unsafe_coverage_path/out"
expect_reject 'unsafe coverage path' "$unsafe_coverage_path"

backslash_coverage_path="$fixture/backslash-coverage-path"
make_valid "$backslash_coverage_path"
sed -i '2s|internal/example.go|internal\\example.go|' "$backslash_coverage_path/out"
expect_reject 'backslash coverage path' "$backslash_coverage_path"

duplicate_separator_path="$fixture/duplicate-separator-path"
make_valid "$duplicate_separator_path"
sed -i '2s|internal/example.go|internal//example.go|' "$duplicate_separator_path/out"
expect_reject 'duplicate path separator' "$duplicate_separator_path"

long_coverage_path="$fixture/long-coverage-path"
make_valid "$long_coverage_path"
long_name=$(printf 'a%.0s' {1..158})
sed -i "2s|internal/example.go|${long_name}.go|" "$long_coverage_path/out"
expect_reject 'overlong coverage path' "$long_coverage_path"

invalid_utf8_coverage="$fixture/invalid-utf8-coverage"
make_valid "$invalid_utf8_coverage"
printf 'internal/\377.go:10.2,12.3 2 1\n' >>"$invalid_utf8_coverage/out"
expect_reject 'invalid UTF-8 coverage' "$invalid_utf8_coverage"

invalid_utf8_junit="$fixture/invalid-utf8-junit"
make_valid "$invalid_utf8_junit"
printf '<testsuite name="\377"/>\n' >"$invalid_utf8_junit/report-junit.xml"
expect_reject 'invalid UTF-8 JUnit' "$invalid_utf8_junit"

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
