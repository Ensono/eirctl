#!/usr/bin/env bash
# Validate the bounded, passive Sonar report artifact before a trusted workflow
# lets the scanner parse it. This must run on a protected base revision before
# any untrusted source is materialized.
set -euo pipefail

readonly max_coverage_bytes=$((50 * 1024 * 1024))
readonly max_junit_bytes=$((10 * 1024 * 1024))

fail() {
  printf 'Sonar report artifact validation failed: %s\n' "$*" >&2
  exit 1
}

usage() {
  cat >&2 <<'USAGE'
Usage: validate-sonar-reports.sh <extracted-artifact-directory>

The artifact must contain exactly these regular files:
  .coverage/out               (at most 50 MiB)
  .coverage/report-junit.xml  (at most 10 MiB)

Symlinks, special files, traversal-derived paths, and any other content are
rejected. Missing coverage is a failed preparation result, never a silent skip.
USAGE
}

if [[ $# -ne 1 ]]; then
  usage
  exit 2
fi

root=$1
[[ -d "$root" && ! -L "$root" ]] || fail "artifact root must be a real directory"

# The extracted tree must contain only the two declared regular report files and
# their containing directory. `find -P` never follows a malicious symlink.
while IFS=$'\t' read -r type path; do
  case "$type:$path" in
    d:.coverage)
      ;;
    f:.coverage/out|f:.coverage/report-junit.xml)
      ;;
    l:*|b:*|c:*|p:*|s:*)
      fail "artifact contains forbidden $type entry: $path"
      ;;
    *)
      # This includes names produced by traversal attempts after extraction,
      # nested paths, and any unexpected report or executable-looking content.
      fail "artifact contains unexpected entry: $path"
      ;;
  esac
done < <(find -P "$root" -mindepth 1 -printf '%y\t%P\n' | LC_ALL=C sort)

coverage="$root/.coverage/out"
junit="$root/.coverage/report-junit.xml"
[[ -f "$coverage" && ! -L "$coverage" ]] || fail "missing regular coverage report .coverage/out"
[[ -f "$junit" && ! -L "$junit" ]] || fail "missing regular JUnit report .coverage/report-junit.xml"

coverage_size=$(wc -c <"$coverage")
junit_size=$(wc -c <"$junit")
(( coverage_size <= max_coverage_bytes )) || fail ".coverage/out exceeds ${max_coverage_bytes} bytes"
(( junit_size <= max_junit_bytes )) || fail ".coverage/report-junit.xml exceeds ${max_junit_bytes} bytes"

printf 'Sonar report artifact contract checks passed\n'
