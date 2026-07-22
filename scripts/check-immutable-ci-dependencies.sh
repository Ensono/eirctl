#!/usr/bin/env bash
# Reject mutable selections in the maintained CI/release task contexts and
# workflow-installed tools. Keep this list scoped to configuration executed by
# the repository's lint, test, and release paths.
set -euo pipefail

fail() {
  printf 'immutable CI dependency check failed: %s\n' "$*" >&2
  exit 1
}

context_files=(eirctl.yaml shared/build/go/eirctl.yaml)
for file in "${context_files[@]}"; do
  while IFS= read -r image; do
    [[ "$image" =~ :.+@sha256:[0-9a-f]{64}$ ]] || fail "$file has a tag-only or malformed context image: $image"
  done < <(awk '
    /^contexts:/ { in_contexts=1; next }
    /^(tasks|pipelines):/ { in_contexts=0 }
    in_contexts && /^[[:space:]]+name:[[:space:]]/ { print $2 }
  ' "$file")
done

while IFS= read -r entry; do
  selector="${entry##*:}"
  [[ "$selector" =~ ^[0-9]+\.[0-9]+\.[0-9]+$ ]] || fail "floating GitVersion selector: $entry"
done < <(grep -RHE "versionSpec:[[:space:]]*['\"]?" .github/workflows | sed -E "s/.*versionSpec:[[:space:]]*['\"]?([^'\"]+).*/GitVersion:\1/")

if grep -RInE 'govulncheck@latest|govulncheck@v?[0-9]+\.[0-9]+$' eirctl.yaml shared/build/go/eirctl.yaml >/dev/null; then
  fail "govulncheck must use an exact reviewed module version"
fi
if ! grep -RInE 'govulncheck@v[0-9]+\.[0-9]+\.[0-9]+' eirctl.yaml shared/build/go/eirctl.yaml >/dev/null; then
  fail "missing exact govulncheck module version"
fi

# GitHub Actions must use the official pinned scanner action rather than the
# container task, whose non-root scanner cannot create .scannerwork on runners.
if grep -RIn --include='*.yml' --include='*.yaml' 'sonar:scanner:cli' .github/workflows >/dev/null; then
  fail "GitHub Actions must not invoke the container-backed sonar:scanner:cli task"
fi
for workflow in .github/workflows/pr.yml .github/workflows/trusted-sonarcloud-pr.yml; do
  [[ -f "$workflow" ]] || fail "missing SonarCloud workflow: $workflow"
  grep -Fq 'SonarSource/sonarqube-scan-action@22918119ff8e1ca75a623e15c8296b6ea4fbe28f # v8.2.1' "$workflow" ||
    fail "$workflow must use the reviewed immutable official scanner action"
  grep -Eq 'scannerVersion:[[:space:]]*8\.1\.0\.6389$' "$workflow" ||
    fail "$workflow must pin the reviewed SonarScanner CLI version"
  grep -Eq 'scannerBinariesUrl:[[:space:]]*https://binaries\.sonarsource\.com/Distribution/sonar-scanner-cli$' "$workflow" ||
    fail "$workflow must use the reviewed SonarScanner binaries URL"
  grep -Eq 'skipSignatureVerification:[[:space:]]*"?false"?$' "$workflow" ||
    fail "$workflow must keep SonarScanner signature verification enabled"
done

printf 'immutable CI dependency checks passed\n'
