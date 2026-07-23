#!/usr/bin/env bash
# Enforce the repository-owned GitVersion configuration and the formatting
# contract used by production release artifacts.
set -euo pipefail

fail() {
	printf 'release versioning check failed: %s\n' "$*" >&2
	exit 1
}

require_text() {
	local pattern=$1
	local file=$2
	grep -Fq -- "$pattern" "$file" ||
		fail "$file must contain: $pattern"
}

policy_only=false
if [[ ${1:-} == --policy-only ]]; then
	policy_only=true
	shift
fi
[[ $# -eq 0 ]] || fail 'usage: check-release-versioning.sh [--policy-only]'

require_text 'workflow: GitHubFlow/v1' GitVersion.yml
require_text 'mode: ContinuousDeployment' GitVersion.yml
require_text 'label: null' GitVersion.yml
require_text '  main:' GitVersion.yml
require_text '    label: null' GitVersion.yml

workflows=(
	.github/workflows/pr.yml
	.github/workflows/release.yml
	.github/workflows/release_container.yml
	.github/workflows/debug-build.yml
)
for workflow in "${workflows[@]}"; do
	require_text 'configFilePath: GitVersion.yml' "$workflow"
done

release=.github/workflows/release.yml
container=.github/workflows/release_container.yml
debug=.github/workflows/debug-build.yml

# Binary metadata and Git/GitHub Release tags own one presentation v.
require_text '--set Version=v${SEMVER}' "$release"
require_text '-f tag="v${SEMVER}"' "$release"
require_text 'tag_name: v${{ needs.set-version-tag.outputs.semVer }}' "$release"

# Production containers retain GitVersion's unprefixed SemVer.
require_text 'eirctl:${{ needs.set-version-tag.outputs.semVer }}' "$container"
pwsh_tag='eirctl:pwsh-${{ needs.set-version-tag.outputs.semVer }}'
require_text "$pwsh_tag" "$container"

# Debug artifacts retain their independent run/provenance identity.
require_text 'name: debug-build-${{ github.run_id }}' "$debug"
require_text '--arg semver "$SEMVER"' "$debug"

if [[ $policy_only == false ]]; then
	semver=${SEMVER:-}
	if [[ ! $semver =~ ^[0-9]+\.[0-9]+\.[0-9]+$ ]]; then
		fail "invalid production SemVer: ${semver:-<empty>}"
	fi
fi

printf 'release versioning checks passed\n'
