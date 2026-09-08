# Historical Outcome

This archived change records the label-mediated debug-build design implemented in commit `663dfe10465e`.

The change was statically successful: its separation of the privileged comment broker from pull-request execution removed the then-reported CodeQL cache-poisoning data-flow result. It was not successful end to end. The broker removed and re-added the `build-debug` label with the repository `GITHUB_TOKEN`, but GitHub suppresses most workflow-triggering events created by that token. The expected `pull_request:labeled` activity therefore did not start the builder, and no successful live `/build-debug` acceptance run was recorded.

The follow-up change [`2026-07-17-fix-security-review-findings`](../2026-07-17-fix-security-review-findings/outcome.md) replaced the suppressed label signal with direct dispatch to restore functionality. That replacement later proved to retain default-branch cache-write authority. The current corrective design is [`make-debug-build-cache-safe`](../../make-debug-build-cache-safe/), which uses an authorized `issue_comment` caller and a local reusable builder under GitHub's documented read-only cache boundary.

The original proposal, design, tasks, checked state, dates, and validation claims remain below their notices as historical evidence; they are not the current recommendation.
