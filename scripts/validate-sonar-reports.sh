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
rejected. Coverage records must use canonical repository-relative Go paths;
protected code prefixes those paths with `source/` to match the isolated scanner
namespace. The files are then normalized under `.coverage/`. Missing or malformed
coverage is a failed preparation result, never a silent skip.
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
iconv -f UTF-8 -t UTF-8 "$coverage" >/dev/null 2>&1 || fail "out must be valid UTF-8"
iconv -f UTF-8 -t UTF-8 "$junit" >/dev/null 2>&1 || fail "report-junit.xml must be valid UTF-8"

# The scanner project base is `analysis`, while API materialization deliberately
# places repository paths beneath `analysis/source`. Go's coverage report names
# files relative to the repository root, so normalize each validated record into
# that isolated scanner namespace before any source is materialized or secret is
# exposed. The fixed-string output cannot execute report-controlled content.
normalized_coverage=$(mktemp "$root/.normalized-coverage.XXXXXX")
trap 'rm -f -- "$normalized_coverage"' EXIT
if ! LC_ALL=C awk '
  NR == 1 {
    if ($0 !~ /^mode: (set|count|atomic)$/) exit 10
    print
    next
  }
  {
    record = $0
    if (match(record, /:[0-9]+\.[0-9]+,[0-9]+\.[0-9]+ [0-9]+ [0-9]+$/) == 0) exit 11
    path = substr(record, 1, RSTART - 1)
    if (path !~ /\.go$/ || length(path) > 160 || path ~ /^\// ||
        path ~ /\\/ || path ~ /\/\// || path ~ /(^|\/)\.\.?($|\/)/ ||
        path ~ /[[:cntrl:]]/) exit 12
    print "source/" record
  }
  END {
    if (NR < 2) exit 13
  }
' "$coverage" >"$normalized_coverage"; then
  fail "out must contain a valid Go coverage mode and canonical repository-relative .go records"
fi
normalized_coverage_size=$(wc -c <"$normalized_coverage")
(( normalized_coverage_size <= max_coverage_bytes )) || fail "normalized out exceeds ${max_coverage_bytes} bytes"
mv -- "$normalized_coverage" "$coverage"

# Preserve the protected scanner's established report paths without broadening
# the accepted artifact contract. Any pre-existing directory was rejected above.
normalized_root="$root/.coverage"
mkdir -m 0755 -- "$normalized_root"
mv -- "$coverage" "$normalized_root/out"
mv -- "$junit" "$normalized_root/report-junit.xml"
chmod 0644 -- "$normalized_root/out" "$normalized_root/report-junit.xml"

printf 'Sonar report artifact contract checks passed; coverage namespace and paths normalized\n'
