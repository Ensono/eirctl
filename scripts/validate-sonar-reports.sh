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

The artifact must contain exactly these regular files at its root:
  out               (at most 50 MiB)
  report-junit.xml  (at most 10 MiB)

Symlinks, special files, traversal-derived paths, and any other content are
rejected. After validation, the files are normalized under `.coverage/` for the
trusted scanner. Missing coverage is a failed preparation result, never a silent
skip.
USAGE
}

if [[ $# -ne 1 ]]; then
  usage
  exit 2
fi

root=$1
[[ -d "$root" && ! -L "$root" ]] || fail "artifact root must be a real directory"

# upload-artifact strips the selected files' common .coverage parent, so the
# extracted tree must contain only the two declared root-level regular report
# files. `find -P` never follows a malicious symlink.
while IFS=$'\t' read -r type path; do
  case "$type:$path" in
    f:out|f:report-junit.xml)
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

coverage="$root/out"
junit="$root/report-junit.xml"
[[ -f "$coverage" && ! -L "$coverage" ]] || fail "missing regular coverage report out"
[[ -f "$junit" && ! -L "$junit" ]] || fail "missing regular JUnit report report-junit.xml"

coverage_size=$(wc -c <"$coverage")
junit_size=$(wc -c <"$junit")
(( coverage_size <= max_coverage_bytes )) || fail "out exceeds ${max_coverage_bytes} bytes"
(( junit_size <= max_junit_bytes )) || fail "report-junit.xml exceeds ${max_junit_bytes} bytes"

# Preserve the protected scanner's established report paths without broadening
# the accepted artifact contract. Any pre-existing directory was rejected above.
normalized_root="$root/.coverage"
mkdir -m 0755 -- "$normalized_root"
mv -- "$coverage" "$normalized_root/out"
mv -- "$junit" "$normalized_root/report-junit.xml"
chmod 0644 -- "$normalized_root/out" "$normalized_root/report-junit.xml"

printf 'Sonar report artifact contract checks passed and paths normalized\n'
