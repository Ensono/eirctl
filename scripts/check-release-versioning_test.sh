#!/usr/bin/env bash
set -euo pipefail

script_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
root="$(cd "$script_dir/.." && pwd)"
fixture="$(mktemp -d)"
trap 'rm -rf "$fixture"' EXIT

checker=check-release-versioning.sh
cp -R "$root/GitVersion.yml" "$root/.github" "$fixture/"
cp "$script_dir/$checker" "$fixture/$checker"
chmod +x "$fixture/$checker"

run_check() {
	(cd "$fixture" && "$@")
}

expect_failure() {
	local name=$1
	shift
	if run_check "$@" >/dev/null 2>&1; then
		printf 'expected %s to fail validation\n' "$name" >&2
		exit 1
	fi
}

# CI supplies this value from the pinned GitVersion 6.2.0 execution.
run_check env SEMVER=0.11.5 "./$checker"
expect_failure missing-semver "./$checker"
expect_failure numeric-prerelease \
	env SEMVER=0.11.5-13 "./$checker"

for mutation in missing-main-label binary-prefix container-prefix; do
	backup="$(mktemp -d)"
	cp -R "$fixture/." "$backup/"
	case "$mutation" in
		missing-main-label)
			sed -i 's/^    label: null$/    label: ""/' \
				"$fixture/GitVersion.yml"
			;;
		binary-prefix)
			release="$fixture/.github/workflows/release.yml"
			sed -i 's/Version=v${SEMVER}/Version=${SEMVER}/' "$release"
			;;
		container-prefix)
			container="$fixture/.github/workflows/release_container.yml"
			plain='eirctl:${{ needs.set-version-tag.outputs.semVer }}'
			prefixed='eirctl:v${{ needs.set-version-tag.outputs.semVer }}'
			sed -i "s#$plain#$prefixed#" "$container"
			;;
	esac
	expect_failure "$mutation" "./$checker" --policy-only
	rm -rf "$fixture"
	mkdir -p "$fixture"
	cp -R "$backup/." "$fixture/"
	rm -rf "$backup"
done

printf 'release versioning negative fixtures passed\n'
