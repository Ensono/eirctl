# Historical Outcome

This archived change records the direct-dispatch debug-build replacement implemented in commit `d22ee989cecd`.

The change correctly identified that `GITHUB_TOKEN`-generated label activity did not start the intended `pull_request:labeled` workflow. Replacing that signal with `workflow_dispatch` restored a reliable `/build-debug` command path: trusted broker code selected `main`, passed the validated pull-request number and current full head SHA, and the builder revalidated that identity before execution.

That functional repair did not remove the cache trust problem. GitHub grants `workflow_dispatch` runs authority to create or overwrite caches in the default branch's scope. Read-only repository permissions, non-persistent checkout credentials, an immutable SHA, and disabled `setup-go` caching do not remove the Actions cache service token available to pull-request-controlled code. CodeQL alert 23 (`actions/cache-poisoning/poisonable-step`) therefore superseded the archive's conclusion that the dispatched builder was adequately isolated.

The current corrective design is [`make-debug-build-cache-safe`](../../make-debug-build-cache-safe/). It retains reliable exact-comment authorization but invokes a local reusable builder that inherits the `issue_comment` run's documented read-only default-branch cache boundary, then finalizes provenance on a fresh trusted runner.

The original proposal, design, tasks, checked state, dates, and validation claims remain below their notices as historical evidence; they are not the current recommendation.
