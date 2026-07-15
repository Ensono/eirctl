#!/usr/bin/env bash
set -euo pipefail

# The policy checker uses the repository's pinned yaml.v3 dependency. It performs
# syntax validation and structural checks without downloading an action or tool.
go run ./scripts/check-workflow-policy
