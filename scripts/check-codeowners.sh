#!/usr/bin/env bash
# Require maintainers to own executable CI configuration and the ownership policy.
set -euo pipefail

readonly file=.github/CODEOWNERS
readonly owner='@Ensono/digital-tools-maintainers'

fail() {
  printf 'CODEOWNERS validation failed: %s\n' "$*" >&2
  exit 1
}

[[ -f "$file" ]] || fail "missing $file"
for path in '/.github/CODEOWNERS' '/.github/workflows/**' '/sonar-project.properties'; do
  grep -Fxq "$path $owner" "$file" || fail "$path must be owned by $owner"
done

printf 'CODEOWNERS coverage checks passed\n'
