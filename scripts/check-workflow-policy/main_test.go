package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Ensono/eirctl/internal/schema"
)

func TestPrivilegedPRExecutionFixtures(t *testing.T) {
	cases := []struct {
		name string
		want bool
	}{
		{name: "broker", want: false},
		{name: "builder", want: false},
		{name: "publisher", want: false},
		{name: "privileged-checkout-execution", want: true},
		{name: "privileged-cache-poisoning", want: true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			workflow, err := os.ReadFile(filepath.Join("testdata", tc.name+".yml"))
			if err != nil {
				t.Fatal(err)
			}
			if got := hasPrivilegedPRExecution(string(workflow)); got != tc.want {
				t.Fatalf("hasPrivilegedPRExecution() = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestStructuralPrivilegedFlowAnalysisRejectsDynamicExecution(t *testing.T) {
	cases := []struct {
		name     string
		trigger  string
		ref      string
		execute  string
		wantFail bool
	}{
		{name: "workflow-dispatch-input", trigger: "workflow_dispatch", ref: "${{ inputs.ref }}", execute: "- run: go test ./...", wantFail: true},
		{name: "workflow-run-head-sha", trigger: "workflow_run", ref: "${{ github.event.workflow_run.head_sha }}", execute: "- run: go test ./...", wantFail: true},
		{name: "step-output-ref", trigger: "issue_comment", ref: "${{ steps.resolve.outputs.sha }}", execute: "- run: go test ./...", wantFail: true},
		{name: "local-action", trigger: "pull_request_target", ref: "${{ github.event.pull_request.head.sha }}", execute: "- uses: ./actions/build", wantFail: true},
		{name: "pinned-docker-build-action", trigger: "workflow_dispatch", ref: "${{ inputs.ref }}", execute: "- uses: docker://example.invalid/build@sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", wantFail: true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			content := "on: [" + tc.trigger + "]\njobs:\n  build:\n    steps:\n      - uses: actions/checkout@0123456789012345678901234567890123456789\n        with:\n          ref: " + tc.ref + "\n      " + tc.execute + "\n"
			if got := hasPrivilegedPRExecution(content); got != tc.wantFail {
				t.Fatalf("hasPrivilegedPRExecution() = %v, want %v", got, tc.wantFail)
			}
		})
	}
}

func TestPrivilegedFlowRejectsShellCheckout(t *testing.T) {
	cases := []string{
		"git fetch origin ${{ inputs.ref }} && git checkout FETCH_HEAD",
		"git clone $REPOSITORY source",
		"gh pr checkout $PR_NUMBER",
		"git switch $REF",
	}
	for _, command := range cases {
		content := "on: [workflow_dispatch]\njobs:\n  build:\n    steps:\n      - env: {REF: '${{ needs.resolve.outputs.ref }}', REPOSITORY: '${{ inputs.repository }}', PR_NUMBER: '${{ inputs.pull_request }}'}\n        run: " + command + "\n"
		if !hasPrivilegedPRExecution(content) {
			t.Fatalf("hasPrivilegedPRExecution() accepted a dynamic shell checkout: %s", command)
		}
	}
}

func TestPrivilegedFlowRejectsDerivedCheckoutRepositoryAndRef(t *testing.T) {
	cases := []struct {
		name       string
		repository string
		ref        string
		next       string
	}{
		{name: "needs ref", ref: "${{ needs.resolve.outputs.sha }}", next: "- run: go test ./..."},
		{name: "matrix ref", ref: "${{ matrix.sha }}", next: "- run: go test ./..."},
		{name: "dynamic repository", repository: "${{ needs.resolve.outputs.repository }}", ref: "main", next: "- run: go test ./..."},
		{name: "upload consumes checkout", ref: "${{ inputs.sha }}", next: "- uses: actions/upload-artifact@0123456789012345678901234567890123456789\n        with: {path: .}"},
		{name: "setup cache consumes checkout", ref: "${{ inputs.sha }}", next: "- uses: actions/setup-go@0123456789012345678901234567890123456789\n        with: {cache: 'true'}"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			content := "on: [workflow_dispatch]\njobs:\n  build:\n    runs-on: ubuntu-24.04\n    steps:\n      - uses: actions/checkout@0123456789012345678901234567890123456789\n        with:\n          repository: " + tc.repository + "\n          ref: " + tc.ref + "\n      " + tc.next + "\n"
			if !hasPrivilegedPRExecution(content) {
				t.Fatal("hasPrivilegedPRExecution() accepted a derived checkout flow")
			}
		})
	}
}

func TestDynamicRunnerIsPersistentAuthority(t *testing.T) {
	workflow, err := parseWorkflow("fixture.yml", []byte("on: [issue_comment]\njobs: {build: {runs-on: '${{ inputs.runner }}'}}\n"))
	if err != nil {
		t.Fatal(err)
	}
	authority := authorityForJob(map[string]Workflow{workflow.Path: workflow}, workflow, workflow.Jobs.Values["build"], directRootEvents(workflow))
	if !authority.persistentRunner {
		t.Fatal("dynamic runner selector was treated as GitHub-hosted")
	}
}

func TestParseWorkflowCapturesTrustedTopologyFields(t *testing.T) {
	content := `name: Analyzer
on: [workflow_run]
permissions: {contents: read}
env: {WORKFLOW_TOKEN: safe}
jobs:
  analyze:
    concurrency: {group: 'analyzer-${{ github.run_id }}'}
    env: {JOB_TOKEN: safe}
    steps:
      - name: Scan passive inputs
        uses: SonarSource/sonarqube-scan-action@0123456789012345678901234567890123456789
        with: {args: -Dsonar.projectBaseDir=analysis}
        env: {SONAR_TOKEN: '${{ secrets.SONAR_TOKEN }}'}
`
	workflow, err := parseWorkflow("fixture.yml", []byte(content))
	if err != nil {
		t.Fatal(err)
	}
	job := workflow.Jobs.Values["analyze"]
	step := job.Steps[0]
	if scalarValue(workflow.Env["WORKFLOW_TOKEN"]) != "safe" || scalarValue(job.Env["JOB_TOKEN"]) != "safe" ||
		concurrencyGroup(job) != "analyzer-${{ github.run_id }}" || step.Name != "Scan passive inputs" ||
		scalarValue(step.Env["SONAR_TOKEN"]) != "${{ secrets.SONAR_TOKEN }}" || step.With["args"] != "-Dsonar.projectBaseDir=analysis" {
		t.Fatalf("trusted topology fields were not preserved: %#v", workflow)
	}
}

func TestParseWorkflowSupportsEquivalentTriggerSyntax(t *testing.T) {
	cases := []string{
		"on: [workflow_dispatch]\npermissions: {contents: read}\njobs: {build: {runs-on: [self-hosted, linux]}}\n",
		"on:\n  workflow_dispatch:\npermissions:\n  contents: read\njobs:\n  build:\n    runs-on: ubuntu-24.04\n",
		"on: {'workflow_dispatch': {}}\npermissions: {'contents': 'read'}\njobs: {'build': {'runs-on': 'ubuntu-24.04'}}\n",
	}
	for _, content := range cases {
		workflow, err := parseWorkflow("fixture.yml", []byte(content))
		if err != nil {
			t.Fatal(err)
		}
		if !hasTrigger(workflow, "workflow_dispatch") {
			t.Fatal("workflow triggers do not include workflow_dispatch")
		}
	}
}

func TestDefaultBranchCacheWriteEvents(t *testing.T) {
	workflow, err := parseWorkflow("fixture.yml", []byte("on: {push: {branches: [main]}}\njobs: {build: {runs-on: ubuntu-24.04}}\n"))
	if err != nil {
		t.Fatal(err)
	}
	cases := []struct {
		event string
		want  bool
	}{
		{event: "push", want: true},
		{event: "unknown_future_event", want: true},
		{event: "workflow_dispatch", want: true},
		{event: "repository_dispatch", want: true},
		{event: "delete", want: true},
		{event: "registry_package", want: true},
		{event: "page_build", want: true},
		{event: "schedule", want: true},
		{event: "issue_comment", want: false},
		{event: "pull_request_target", want: false},
		{event: "workflow_run", want: false},
		{event: "pull_request", want: false},
		{event: "workflow_call", want: false},
	}
	for _, tc := range cases {
		t.Run(tc.event, func(t *testing.T) {
			if got := eventWritesDefaultBranchCache(workflow, tc.event); got != tc.want {
				t.Fatalf("eventWritesDefaultBranchCache(%q) = %v, want %v", tc.event, got, tc.want)
			}
		})
	}
}

func TestPushCacheAuthorityMatchesDefaultBranchFilters(t *testing.T) {
	cases := []struct {
		name     string
		branches []string
		ignore   []string
		want     bool
	}{
		{name: "unfiltered", want: true},
		{name: "literal main", branches: []string{"main"}, want: true},
		{name: "single wildcard", branches: []string{"*"}, want: true},
		{name: "double wildcard", branches: []string{"**"}, want: true},
		{name: "prefix wildcard", branches: []string{"main*"}, want: true},
		{name: "explicit non-default", branches: []string{"develop"}, want: false},
		{name: "default ignored", ignore: []string{"main"}, want: false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			workflow := Workflow{GithubWorkflow: schema.GithubWorkflow{On: &schema.GithubTriggerEvents{}}}
			workflow.On.Push.Branches = tc.branches
			workflow.On.Push.BranchesIgnore = tc.ignore
			if got := eventWritesDefaultBranchCache(workflow, "push"); got != tc.want {
				t.Fatalf("eventWritesDefaultBranchCache(push) = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestReusableWorkflowInheritsEveryEffectiveRootCallerEvent(t *testing.T) {
	parse := func(path, content string) Workflow {
		t.Helper()
		workflow, err := parseWorkflow(path, []byte(content))
		if err != nil {
			t.Fatal(err)
		}
		return workflow
	}
	workflows := map[string]Workflow{
		".github/workflows/comment.yml":      parse(".github/workflows/comment.yml", "on: [issue_comment]\njobs: {call: {uses: ./.github/workflows/reusable.yml}}\n"),
		".github/workflows/dispatch.yml":     parse(".github/workflows/dispatch.yml", "on: {workflow_dispatch: {}}\njobs: {call: {uses: './.github/workflows/reusable.yml'}}\n"),
		".github/workflows/repository.yml":   parse(".github/workflows/repository.yml", "on: repository_dispatch\njobs: {call: {uses: ./.github/workflows/intermediate.yml}}\n"),
		".github/workflows/push.yml":         parse(".github/workflows/push.yml", "on: {push: {branches: [main]}}\njobs: {call: {uses: ./.github/workflows/reusable.yml}}\n"),
		".github/workflows/intermediate.yml": parse(".github/workflows/intermediate.yml", "on: [workflow_call]\njobs: {call: {uses: ./.github/workflows/reusable.yml}}\n"),
		".github/workflows/reusable.yml":     parse(".github/workflows/reusable.yml", "on:\n  workflow_call:\n    inputs: {commit_sha: {required: true, type: string}}\njobs: {build: {runs-on: ubuntu-24.04}}\n"),
	}
	events := resolveEffectiveRootEvents(workflows)[".github/workflows/reusable.yml"]
	for _, event := range []string{"issue_comment", "workflow_dispatch", "repository_dispatch", "push"} {
		found := false
		for inherited := range events {
			found = found || inherited.Name == event
		}
		if !found {
			t.Fatalf("reusable workflow did not inherit %s: %#v", event, events)
		}
	}
	for inherited := range events {
		if inherited.Name == "workflow_call" {
			t.Fatalf("workflow_call was incorrectly treated as an effective root event: %#v", events)
		}
	}
}

func TestDirectAndReusableExecutionCacheAuthority(t *testing.T) {
	cases := []struct {
		event     string
		trigger   string
		wantWrite bool
	}{
		{event: "issue_comment", trigger: "on: [issue_comment]", wantWrite: false},
		{event: "workflow_dispatch", trigger: "on: {workflow_dispatch: {}}", wantWrite: true},
		{event: "repository_dispatch", trigger: "on: 'repository_dispatch'", wantWrite: true},
		{event: "push", trigger: "on:\n  push:\n    branches: ['main']", wantWrite: true},
	}
	for _, tc := range cases {
		for _, reusable := range []bool{false, true} {
			name := tc.event + "-direct"
			if reusable {
				name = tc.event + "-reusable"
			}
			t.Run(name, func(t *testing.T) {
				buildYAML := "on: [workflow_call]\njobs:\n  build:\n    runs-on: ubuntu-24.04\n    steps:\n      - uses: actions/checkout@0123456789012345678901234567890123456789\n        with: {ref: '${{ inputs.commit_sha }}'}\n      - run: go test ./...\n"
				callerJobs := "jobs:\n  build:\n    runs-on: ubuntu-24.04\n    steps:\n      - uses: actions/checkout@0123456789012345678901234567890123456789\n        with: {ref: '${{ inputs.commit_sha }}'}\n      - run: go test ./...\n"
				if reusable {
					callerJobs = "jobs: {call: {uses: ./.github/workflows/build.yml}}\n"
				}
				caller, err := parseWorkflow(".github/workflows/caller.yml", []byte(tc.trigger+"\n"+callerJobs))
				if err != nil {
					t.Fatal(err)
				}
				workflows := map[string]Workflow{".github/workflows/caller.yml": caller}
				target := caller
				if reusable {
					build, err := parseWorkflow(".github/workflows/build.yml", []byte(buildYAML))
					if err != nil {
						t.Fatal(err)
					}
					workflows[build.Path] = build
					target = build
				}
				events := resolveEffectiveRootEvents(workflows)[target.Path]
				authority := authorityForJob(workflows, target, target.Jobs.Values["build"], events)
				if authority.defaultBranchCacheWrite != tc.wantWrite {
					t.Fatalf("cache-write authority = %v, want %v; events=%#v", authority.defaultBranchCacheWrite, tc.wantWrite, events)
				}
			})
		}
	}
}

func TestReusableExecutionRejectsWritableAndUnknownCallers(t *testing.T) {
	cases := []struct {
		name     string
		trigger  string
		wantFail bool
	}{
		{name: "read-only issue comment", trigger: "on: [issue_comment]", wantFail: false},
		{name: "wildcard push", trigger: "on: {push: {branches: ['**']}}", wantFail: true},
		{name: "non-default push", trigger: "on: {push: {branches: [develop]}}", wantFail: false},
		{name: "unknown event fails closed", trigger: "on: [future_cache_event]", wantFail: true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			caller, err := parseWorkflow(".github/workflows/caller.yml", []byte(tc.trigger+"\njobs: {call: {uses: ./.github/workflows/build.yml}}\n"))
			if err != nil {
				t.Fatal(err)
			}
			build, err := parseWorkflow(".github/workflows/build.yml", []byte("on: [workflow_call]\npermissions: {contents: read}\njobs:\n  build:\n    runs-on: ubuntu-24.04\n    permissions: {contents: read}\n    steps:\n      - uses: actions/checkout@0123456789012345678901234567890123456789\n        with: {ref: '${{ inputs.commit_sha }}', persist-credentials: 'false'}\n      - run: go test ./...\n"))
			if err != nil {
				t.Fatal(err)
			}
			workflows := map[string]Workflow{caller.Path: caller, build.Path: build}
			events := resolveEffectiveRootEvents(workflows)
			err = validatePrivilegedFlow(workflows, build, events[build.Path])
			if (err != nil) != tc.wantFail {
				t.Fatalf("validatePrivilegedFlow() error = %v, want failure %v", err, tc.wantFail)
			}
		})
	}
}

func TestValidateRejectsWritableReusableCallerEndToEnd(t *testing.T) {
	cases := []struct {
		name    string
		trigger string
	}{
		{name: "wildcard default branch push", trigger: "on: {push: {branches: ['**']}}"},
		{name: "unknown future event", trigger: "on: [future_cache_event]"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			root := copyWorkflowFiles(t, nil)
			caller := tc.trigger + `
permissions: {contents: read}
jobs:
  call:
    uses: ./.github/workflows/debug-build.yml
    permissions: {contents: read}
    with: {pull_request: '1', commit_sha: '0123456789012345678901234567890123456789'}
`
			path := filepath.Join(root, ".github", "workflows", "unsafe-caller.yml")
			if err := os.WriteFile(path, []byte(caller), 0o644); err != nil {
				t.Fatal(err)
			}
			if err := Validate(root); err == nil || !strings.Contains(err.Error(), "cache-write authority") {
				t.Fatalf("Validate() error = %v, want cache-write caller rejection", err)
			}
		})
	}
}

func TestValidateRejectsAdditionalReadOnlyDebugCaller(t *testing.T) {
	root := copyWorkflowFiles(t, nil)
	caller := `on: [pull_request]
permissions: {contents: read}
jobs:
  call:
    uses: ./.github/workflows/debug-build.yml
    permissions: {contents: read}
    with: {pull_request: '1', commit_sha: '0123456789012345678901234567890123456789'}
`
	path := filepath.Join(root, ".github", "workflows", "extra-read-only-caller.yml")
	if err := os.WriteFile(path, []byte(caller), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := Validate(root); err == nil || !strings.Contains(err.Error(), "workflow_call-only") {
		t.Fatalf("Validate() error = %v, want additional caller rejection", err)
	}
}

func TestParserResolvesAliasesAndMergedJobs(t *testing.T) {
	content := `
name: Aliases
trigger: &trigger [workflow_dispatch]
base: &base
  runs-on: ubuntu-24.04
  permissions: {contents: read}
  steps:
    - uses: actions/checkout@0123456789012345678901234567890123456789
      with: {ref: '${{ inputs.commit_sha }}'}
on: *trigger
jobs:
  aliased: *base
  merged:
    <<: *base
    name: Merged job
`
	workflow, err := parseWorkflow("fixture.yml", []byte(content))
	if err != nil {
		t.Fatal(err)
	}
	for _, jobID := range []string{"aliased", "merged"} {
		job := workflow.Jobs.Values[jobID]
		if !hasTrigger(workflow, "workflow_dispatch") || len(job.RunsOn) != 1 || job.RunsOn[0] != "ubuntu-24.04" ||
			!samePermissions(job.Permissions, Permissions{"contents": "read"}) || len(job.Steps) != 1 {
			t.Fatalf("alias/merge fields were not preserved for %s: %#v", jobID, job)
		}
	}
}

func TestSupportedDebugTopologyFixtures(t *testing.T) {
	root := filepath.Join("testdata", "supported-debug")
	workflows, err := LoadWorkflows(root)
	if err != nil {
		t.Fatal(err)
	}
	for _, workflow := range workflows {
		if err := validateWorkflow(workflow); err != nil {
			t.Fatalf("validateWorkflow(%s): %v", workflow.Path, err)
		}
	}
	events := resolveEffectiveRootEvents(workflows)
	for path, workflow := range workflows {
		if err := validatePrivilegedFlow(workflows, workflow, events[path]); err != nil {
			t.Fatalf("validatePrivilegedFlow(%s): %v", path, err)
		}
	}
	for _, validate := range []func(map[string]Workflow) error{validateDebugBrokerTopology, validateDebugBuilderTopology, validateDebugPublisherTopology} {
		if err := validate(workflows); err != nil {
			t.Fatal(err)
		}
	}
}

func TestPolicyUsesJobIDForPermissionRules(t *testing.T) {
	content := `name: Broker
on: [issue_comment]
permissions: {contents: read}
jobs:
  authorize:
    name: Human-readable display name
    permissions: {pull-requests: read}
    runs-on: ubuntu-24.04
`
	workflow, err := parseWorkflow(".github/workflows/debug-build-request.yml", []byte(content))
	if err != nil {
		t.Fatal(err)
	}
	if err := validateWorkflow(workflow); err != nil {
		t.Fatalf("validateWorkflow() used the display name instead of the job ID: %v", err)
	}
}

func TestWorkflowTopologyAndPermissions(t *testing.T) {
	root := filepath.Clean(filepath.Join("..", ".."))
	if err := Validate(root); err != nil {
		t.Fatalf("Validate(%q): %v", root, err)
	}
}

func TestTargetWorkflowTopologyAndPermissions(t *testing.T) {
	root := copyWorkflowFiles(t, nil)
	if err := Validate(root); err != nil {
		t.Fatalf("Validate(target fixtures): %v", err)
	}
}

func TestLegacyDebugTransitionRequiresExactReviewedFiles(t *testing.T) {
	root := copyLegacyWorkflowFiles(t)
	path := filepath.Join(root, ".github", "workflows", "debug-build.yml")
	contents, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, append(contents, '\n'), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := Validate(root); err == nil || !strings.Contains(err.Error(), "cache-write authority") {
		t.Fatalf("Validate() error = %v, want exact legacy digest rejection", err)
	}
}

func TestLegacyDebugTransitionRejectsAdditionalCaller(t *testing.T) {
	root := copyLegacyWorkflowFiles(t)
	caller := `on: [pull_request]
permissions: {contents: read}
jobs:
  call:
    uses: ./.github/workflows/debug-build.yml
    permissions: {contents: read}
    with: {pull_request: '1', commit_sha: '0123456789012345678901234567890123456789'}
`
	path := filepath.Join(root, ".github", "workflows", "extra-caller.yml")
	if err := os.WriteFile(path, []byte(caller), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := Validate(root); err == nil || !strings.Contains(err.Error(), "cache-write authority") {
		t.Fatalf("Validate() error = %v, want legacy additional-caller rejection", err)
	}
}

func TestLegacyDebugTransitionAllowsIndependentlyValidatedWorkflow(t *testing.T) {
	root := copyLegacyWorkflowFiles(t)
	extra := "on: [pull_request]\npermissions: {contents: read}\njobs: {observe: {runs-on: ubuntu-24.04}}\n"
	path := filepath.Join(root, ".github", "workflows", "unrelated.yml")
	if err := os.WriteFile(path, []byte(extra), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := Validate(root); err != nil {
		t.Fatalf("Validate() rejected independently safe workflow: %v", err)
	}
}

func copyLegacyWorkflowFiles(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	workflowDir := filepath.Join(root, ".github", "workflows")
	if err := os.MkdirAll(workflowDir, 0o755); err != nil {
		t.Fatal(err)
	}
	baseWorkflowDir := filepath.Join("..", "..", ".github", "workflows")
	entries, err := os.ReadDir(baseWorkflowDir)
	if err != nil {
		t.Fatal(err)
	}
	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".yml" {
			continue
		}
		contents, err := os.ReadFile(filepath.Join(baseWorkflowDir, entry.Name()))
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(workflowDir, entry.Name()), contents, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return root
}

func TestTrustedWorkflowRunRequiresAllGuards(t *testing.T) {
	valid := "github.event.workflow_run.event == 'push' && github.event.workflow_run.head_repository.full_name == github.repository && github.event.workflow_run.head_branch == 'main'"
	if !trustedWorkflowRun(valid) {
		t.Fatal("trustedWorkflowRun rejected the complete guard")
	}
	for _, missing := range []string{
		"github.event.workflow_run.event == 'push' && github.event.workflow_run.head_branch == 'main'",
		"github.event.workflow_run.event == 'push' && github.event.workflow_run.head_repository.full_name == github.repository",
		"github.event.workflow_run.head_repository.full_name == github.repository && github.event.workflow_run.head_branch == 'main'",
	} {
		if trustedWorkflowRun(missing) {
			t.Fatalf("trustedWorkflowRun accepted incomplete guard %q", missing)
		}
	}
}

func TestPolicyRejectsDebugBuilderBeforeValidationAndStepSecret(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(string) string
	}{
		{
			name: "checkout before validation",
			mutate: func(content string) string {
				validationStart := strings.Index(content, "      - name: Revalidate pull-request identity")
				checkoutStart := strings.Index(content, "      - name: Check out validated")
				installStart := strings.Index(content, "      - name: Install GitVersion")
				if validationStart < 0 || checkoutStart < 0 || installStart < 0 {
					t.Fatal("debug builder test markers are missing")
				}
				return content[:validationStart] + content[checkoutStart:installStart] + content[validationStart:checkoutStart] + content[installStart:]
			},
		},
		{
			name: "step scoped secret",
			mutate: func(content string) string {
				return strings.Replace(content, "        env:\n          PULL_REQUEST:", "        env:\n          REVIEW_PROBE_SECRET: ${{ secrets.REVIEW_PROBE_SECRET }}\n          PULL_REQUEST:", 1)
			},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if err := Validate(copyWorkflowRootFile(t, "debug-build.yml", test.mutate)); err == nil {
				t.Fatal("Validate() accepted unsafe debug builder mutation")
			}
		})
	}
}

func TestDebugTopologyRejectsUnsafeMutations(t *testing.T) {
	cases := []struct {
		name    string
		file    string
		oldText string
		newText string
	}{
		{
			name:    "writable reusable caller",
			file:    "debug-build-request.yml",
			oldText: "  build:\n    needs: authorize\n    uses: ./.github/workflows/debug-build.yml\n    permissions:\n      contents: read",
			newText: "  build:\n    needs: authorize\n    uses: ./.github/workflows/debug-build.yml\n    permissions:\n      contents: write",
		},
		{
			name:    "direct dispatched builder",
			file:    "debug-build.yml",
			oldText: "  workflow_call:",
			newText: "  workflow_dispatch:",
		},
		{
			name:    "github token label signaling",
			file:    "debug-build-request.yml",
			oldText: "getCollaboratorPermissionLevel",
			newText: "addLabels",
		},
		{
			name:    "request adds reusable trigger",
			file:    "debug-build-request.yml",
			oldText: "on:\n  issue_comment:",
			newText: "on:\n  workflow_call:\n  issue_comment:",
		},
		{
			name:    "publisher adds reusable trigger",
			file:    "publish-debug-release.yml",
			oldText: "on:\n  workflow_dispatch:",
			newText: "on:\n  workflow_call:\n  workflow_dispatch:",
		},
		{
			name:    "secret inheritance",
			file:    "debug-build-request.yml",
			oldText: "    uses: ./.github/workflows/debug-build.yml\n    permissions:",
			newText: "    uses: ./.github/workflows/debug-build.yml\n    secrets: inherit\n    permissions:",
		},
		{
			name:    "self hosted execution",
			file:    "debug-build.yml",
			oldText: "    runs-on: ubuntu-24.04",
			newText: "    runs-on: [self-hosted, linux]",
		},
		{
			name:    "missing immutable revalidation",
			file:    "debug-build.yml",
			oldText: "pullRequest.head.sha.toLowerCase() !== commitSHA.toLowerCase()",
			newText: "false",
		},
		{
			name:    "same job provenance generation",
			file:    "debug-build.yml",
			oldText: "          go run cmd/main.go run pipeline build:binary \\",
			newText: "          echo debug-build-provenance.json\n          go run cmd/main.go run pipeline build:binary \\",
		},
		{
			name:    "reusable workflow declares secret",
			file:    "debug-build.yml",
			oldText: "  workflow_call:\n    inputs:",
			newText: "  workflow_call:\n    secrets:\n      release_token: {required: true}\n    inputs:",
		},
		{
			name:    "missing builder trigger",
			file:    "debug-build.yml",
			oldText: "on:\n  workflow_call:",
			newText: "on:",
		},
		{
			name:    "authorization shell checkout",
			file:    "debug-build-request.yml",
			oldText: "    steps:\n      - name: Authorize exact debug-build request",
			newText: "    steps:\n      - run: git clone ${{ github.event.issue.pull_request.url }} source\n      - name: Authorize exact debug-build request",
		},
		{
			name:    "authorization local action",
			file:    "debug-build-request.yml",
			oldText: "    steps:\n      - name: Authorize exact debug-build request",
			newText: "    steps:\n      - uses: ./actions/authorize\n      - name: Authorize exact debug-build request",
		},
		{
			name:    "finalizer executes downloaded binary",
			file:    "debug-build-request.yml",
			oldText: "      - name: Upload immutable final debug build artifact",
			newText: "      - run: ./intermediate/bin/eirctl-linux-amd64\n      - name: Upload immutable final debug build artifact",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			root := copyWorkflowFiles(t, func(name string, contents []byte) []byte {
				if name != tc.file {
					return contents
				}
				if !strings.Contains(string(contents), tc.oldText) {
					t.Fatalf("mutation marker is missing from %s: %q", tc.file, tc.oldText)
				}
				return []byte(strings.Replace(string(contents), tc.oldText, tc.newText, 1))
			})
			if err := Validate(root); err == nil {
				t.Fatal("Validate() accepted unsafe debug topology mutation")
			}
		})
	}
}

func TestDebugPublisherRejectsIdentityArtifactAndExecutionMutations(t *testing.T) {
	cases := []struct {
		name    string
		oldText string
		newText string
	}{
		{name: "wrong workflow path", oldText: "run.path !== '.github/workflows/debug-build-request.yml'", newText: "run.path !== '.github/workflows/debug-build.yml'"},
		{name: "wrong root event", oldText: "run.event !== 'issue_comment'", newText: "run.event !== 'workflow_dispatch'"},
		{name: "stale head accepted", oldText: "pr.head.sha.toLowerCase() !== commitSHA.toLowerCase()", newText: "false"},
		{name: "failed run accepted", oldText: "run.conclusion !== 'success'", newText: "false"},
		{name: "expired artifact accepted", oldText: "candidate.name === artifactName && !candidate.expired", newText: "candidate.name === artifactName"},
		{name: "ambiguous artifact accepted", oldText: "matches.length !== 1", newText: "matches.length === 0"},
		{name: "artifact run identity omitted", oldText: "artifact.workflow_run?.id !== runId", newText: "false"},
		{name: "run attempt forged", oldText: "String(run.run_attempt)", newText: "String(1)"},
		{name: "malformed provenance accepted", oldText: ".finalized_by == \"finalize\"", newText: ".finalized_by != null"},
		{name: "alternate ref dispatch", oldText: "    if: github.ref == 'refs/heads/main'", newText: "    if: github.ref != ''"},
		{name: "downloaded content execution", oldText: "          test -f release-assets/debug-build-provenance.json", newText: "          bash release-assets/bin/eirctl-linux-amd64\n          test -f release-assets/debug-build-provenance.json"},
		{name: "downloaded Go execution", oldText: "          test -f release-assets/debug-build-provenance.json", newText: "          go run release-assets/tool.go\n          test -f release-assets/debug-build-provenance.json"},
		{name: "downloaded external action", oldText: "      - name: Publish prerelease assets without executing them", newText: "      - uses: example/action@0123456789012345678901234567890123456789\n      - name: Publish prerelease assets without executing them"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			root := copyWorkflowFiles(t, func(name string, contents []byte) []byte {
				if name != "publish-debug-release.yml" {
					return contents
				}
				if !strings.Contains(string(contents), tc.oldText) {
					t.Fatalf("publisher mutation marker is missing: %q", tc.oldText)
				}
				return []byte(strings.Replace(string(contents), tc.oldText, tc.newText, 1))
			})
			if err := Validate(root); err == nil {
				t.Fatal("Validate() accepted unsafe debug publisher mutation")
			}
		})
	}
}

func TestPolicyRejectsUnguardedDebugPublisher(t *testing.T) {
	root := t.TempDir()
	workflowDir := filepath.Join(root, ".github", "workflows")
	if err := os.MkdirAll(workflowDir, 0o755); err != nil {
		t.Fatal(err)
	}
	baseWorkflowDir := filepath.Join("..", "..", ".github", "workflows")
	entries, err := os.ReadDir(baseWorkflowDir)
	if err != nil {
		t.Fatal(err)
	}
	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".yml" {
			continue
		}
		contents := targetWorkflowContents(t, baseWorkflowDir, entry.Name())
		if entry.Name() == "publish-debug-release.yml" {
			contents = []byte(strings.Replace(string(contents), "    if: github.ref == 'refs/heads/main'\n", "", 1))
		}
		if err := os.WriteFile(filepath.Join(workflowDir, entry.Name()), contents, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	if err := Validate(root); err == nil || !strings.Contains(err.Error(), "debug publication") {
		t.Fatalf("Validate() error = %v, want unguarded publication rejection", err)
	}
}

func TestPolicyRejectsLegacyLabelBroker(t *testing.T) {
	root := t.TempDir()
	workflowDir := filepath.Join(root, ".github", "workflows")
	if err := os.MkdirAll(workflowDir, 0o755); err != nil {
		t.Fatal(err)
	}
	baseWorkflowDir := filepath.Join("..", "..", ".github", "workflows")
	entries, err := os.ReadDir(baseWorkflowDir)
	if err != nil {
		t.Fatal(err)
	}
	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".yml" {
			continue
		}
		contents := targetWorkflowContents(t, baseWorkflowDir, entry.Name())
		if entry.Name() == "debug-build-request.yml" {
			contents = []byte(strings.Replace(string(contents), "    permissions:\n      pull-requests: read", "    permissions:\n      issues: write", 1))
		}
		if err := os.WriteFile(filepath.Join(workflowDir, entry.Name()), contents, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	if err := Validate(root); err == nil || !strings.Contains(err.Error(), "unexpected permissions") {
		t.Fatalf("Validate() error = %v, want legacy-label broker rejection", err)
	}
}

func TestCandidateCannotReplaceTrustedPolicyBoundary(t *testing.T) {
	root := t.TempDir()
	workflowDir := filepath.Join(root, ".github", "workflows")
	if err := os.MkdirAll(workflowDir, 0o755); err != nil {
		t.Fatal(err)
	}
	baseWorkflowDir := filepath.Join("..", "..", ".github", "workflows")
	entries, err := os.ReadDir(baseWorkflowDir)
	if err != nil {
		t.Fatal(err)
	}
	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".yml" {
			continue
		}
		contents, err := os.ReadFile(filepath.Join(baseWorkflowDir, entry.Name()))
		if err != nil {
			t.Fatal(err)
		}
		if entry.Name() == "trusted-workflow-policy.yml" {
			contents = []byte(strings.Replace(string(contents), "        with:\n          fetch-depth: 1", "        with:\n          ref: ${{ github.event.pull_request.head.sha }}\n          fetch-depth: 1", 1))
		}
		if err := os.WriteFile(filepath.Join(workflowDir, entry.Name()), contents, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	if err := Validate(root); err == nil || !strings.Contains(err.Error(), "Scorecard Dangerous-Workflow") {
		t.Fatalf("Validate() error = %v, want Scorecard dangerous-checkout rejection", err)
	}
}

func TestReleaseWorkflowsRequireVerifiedStaticMainCheckout(t *testing.T) {
	cases := []struct {
		name     string
		file     string
		oldValue string
		newValue string
	}{
		{
			name:     "dynamic workflow-run checkout",
			file:     "release.yml",
			oldValue: "ref: main",
			newValue: "ref: ${{ github.event.workflow_run.head_sha }}",
		},
		{
			name:     "dynamic container workflow-run checkout",
			file:     "release_container.yml",
			oldValue: "ref: main",
			newValue: "ref: ${{ github.event.workflow_run.head_sha }}",
		},
		{
			name:     "missing immutable revision verification",
			file:     "release_container.yml",
			oldValue: "run: test \"$(git rev-parse HEAD)\" = \"$VALIDATED_HEAD_SHA\"",
			newValue: "run: echo unverified",
		},
		{
			name:     "persisted release checkout credential",
			file:     "release.yml",
			oldValue: "          persist-credentials: false\n\n      - name: Verify the validated workflow-run revision",
			newValue: "\n      - name: Verify the validated workflow-run revision",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			assertReleaseWorkflowMutationRejected(t, tc.file, tc.oldValue, tc.newValue)
		})
	}
}

func assertReleaseWorkflowMutationRejected(t *testing.T, target, oldValue, newValue string) {
	t.Helper()
	root := copyWorkflowFiles(t, func(name string, contents []byte) []byte {
		if name != target {
			return contents
		}
		return []byte(strings.Replace(string(contents), oldValue, newValue, 1))
	})
	if err := Validate(root); err == nil {
		t.Fatal("Validate() unexpectedly accepted a dangerous release checkout")
	}
}

func copyWorkflowFiles(t *testing.T, mutate func(string, []byte) []byte) string {
	t.Helper()
	root := t.TempDir()
	workflowDir := filepath.Join(root, ".github", "workflows")
	if err := os.MkdirAll(workflowDir, 0o755); err != nil {
		t.Fatal(err)
	}
	baseWorkflowDir := filepath.Join("..", "..", ".github", "workflows")
	entries, err := os.ReadDir(baseWorkflowDir)
	if err != nil {
		t.Fatal(err)
	}
	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".yml" {
			continue
		}
		contents := targetWorkflowContents(t, baseWorkflowDir, entry.Name())
		if mutate != nil {
			contents = mutate(entry.Name(), contents)
		}
		if err := os.WriteFile(filepath.Join(workflowDir, entry.Name()), contents, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return root
}

func targetWorkflowContents(t *testing.T, baseWorkflowDir, name string) []byte {
	t.Helper()
	path := filepath.Join(baseWorkflowDir, name)
	for _, debugWorkflow := range []string{"debug-build.yml", "debug-build-request.yml", "publish-debug-release.yml"} {
		if name == debugWorkflow {
			path = filepath.Join("testdata", "supported-debug", ".github", "workflows", name)
			break
		}
	}
	contents, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return contents
}

func TestMaterializerArchivesOnlyConfigurationData(t *testing.T) {
	contents, err := os.ReadFile(filepath.Join("..", "materialize-policy-candidate"))
	if err != nil {
		t.Fatal(err)
	}
	script := string(contents)
	if !strings.Contains(script, "git archive --format=tar FETCH_HEAD -- .github/workflows eirctl.yaml sonar-project.properties") || strings.Contains(script, "scripts/check-workflow-policy") {
		t.Fatal("candidate materializer must archive only configuration data, not candidate checker code")
	}
}

func TestTrustedSonarCloudAnalyzerPolicy(t *testing.T) {
	const (
		trigger      = "on:\n  workflow_run:\n    workflows: [Lint and Test]\n    types: [completed]"
		materializer = "go run trusted/scripts/materialize-sonar-source/main.go"
		concurrency  = "group: sonar-pr-${{ github.event.workflow_run.pull_requests[0].number || format('{0}-{1}', github.event.workflow_run.head_repository.id, github.event.workflow_run.head_branch) }}"
	)
	insertBefore := func(marker, step string) func(string) string {
		return func(content string) string { return strings.Replace(content, marker, step+"\n\n"+marker, 1) }
	}
	cases := []struct {
		name     string
		mutate   func(string) string
		wantFail bool
	}{
		{name: "valid same-repository analyzer"},
		// Fork provenance changes only the verified API values; the protected
		// workflow topology and bounded helper remain identical.
		{name: "valid fork analyzer"},
		{name: "empty fork association unsupported", mutate: func(content string) string {
			return strings.Replace(content, "(length == 0 or length == 1)", "length == 1", 1)
		}, wantFail: true},
		{name: "fork fallback concurrency missing", mutate: func(content string) string {
			return strings.Replace(content, concurrency, "group: sonar-pr-${{ github.event.workflow_run.pull_requests[0].number }}", 1)
		}, wantFail: true},
		{name: "fork lookup includes closed candidates", mutate: func(content string) string {
			return strings.Replace(content, "-f state=open", "-f state=all", 1)
		}, wantFail: true},
		{name: "fork lookup uses alternate base", mutate: func(content string) string {
			return strings.Replace(content, `-f base="$expected_branch"`, "-f base=develop", 1)
		}, wantFail: true},
		{name: "fork lookup uses unverified head", mutate: func(content string) string {
			return strings.Replace(content, `-f head="${head_owner}:${head_ref}"`, "-f head=attacker:branch", 1)
		}, wantFail: true},
		{name: "fork lookup cannot detect ambiguity", mutate: func(content string) string {
			return strings.Replace(content, "-f per_page=2", "-f per_page=1", 1)
		}, wantFail: true},
		{name: "absent or ambiguous candidate count accepted", mutate: func(content string) string {
			return strings.Replace(content, "if length != 1 then", "if length == 0 then", 1)
		}, wantFail: true},
		{name: "closed pull request accepted", mutate: func(content string) string {
			return strings.Replace(content, `.state == "open"`, "true", 1)
		}, wantFail: true},
		{name: "mismatched pull request head ref accepted", mutate: func(content string) string {
			return strings.Replace(content, ".head.ref == $head_ref", ".head.ref != $head_ref", 1)
		}, wantFail: true},
		{name: "checkout action alias", mutate: insertBefore("      - name: Materialize bounded verified Go source through the Git Data API", "      - name: Alias checkout\n        uses: Actions/Checkout@3d3c42e5aac5ba805825da76410c181273ba90b1\n        with:\n          ref: ${{ github.event.workflow_run.head_sha }}"), wantFail: true},
		{name: "mutable source ref", mutate: func(content string) string {
			return strings.Replace(content, `--head-sha "${{ steps.provenance.outputs.head-sha }}"`, `--head-sha "${{ github.event.workflow_run.head_branch }}"`, 1)
		}, wantFail: true},
		{name: "derived source ref", mutate: func(content string) string {
			return strings.Replace(content, `--head-sha "${{ steps.provenance.outputs.head-sha }}"`, `--head-sha "${{ steps.provenance.outputs.head-ref }}"`, 1)
		}, wantFail: true},
		{name: "forged head repository", mutate: func(content string) string {
			return strings.Replace(content, ".head.repo.full_name == $head_repository", ".head.repo.full_name != $head_repository", 1)
		}, wantFail: true},
		{name: "missing current head check", mutate: func(content string) string {
			return strings.Replace(content, materializer, "go run trusted/scripts/materialize-without-head-recheck/main.go", 1)
		}, wantFail: true},
		{name: "helper permitting truncated tree", mutate: func(content string) string {
			return strings.Replace(content, materializer, "go run trusted/scripts/materialize-truncated-tree/main.go", 1)
		}, wantFail: true},
		{name: "helper omitting blob identity", mutate: func(content string) string {
			return strings.Replace(content, materializer, "go run trusted/scripts/materialize-unverified-blobs/main.go", 1)
		}, wantFail: true},
		{name: "helper permitting unsafe paths", mutate: func(content string) string {
			return strings.Replace(content, materializer, "go run trusted/scripts/materialize-unsafe-paths/main.go", 1)
		}, wantFail: true},
		{name: "missing source bounds", mutate: func(content string) string {
			return strings.Replace(content, "          --bounds tree=384,go=160,path=160,file=131072,total=1048576", "", 1)
		}, wantFail: true},
		{name: "expanded source bounds", mutate: func(content string) string {
			return strings.Replace(content, "tree=384,go=160,path=160,file=131072,total=1048576", "tree=9999,go=9999,path=9999,file=999999,total=9999999", 1)
		}, wantFail: true},
		{name: "generic source archive extraction", mutate: func(content string) string {
			return strings.Replace(content, materializer, "gh api repos/example/repository/tarball/main | tar -x", 1)
		}, wantFail: true},
		{name: "non Go materialization", mutate: func(content string) string {
			return strings.Replace(content, materializer, "cp -R untrusted-repository analysis/source", 1)
		}, wantFail: true},
		{name: "missing provenance check", mutate: func(content string) string {
			return strings.Replace(content, ".head.sha == $sha", ".head.sha == $other", 1)
		}, wantFail: true},
		{name: "wrong artifact run identity", mutate: func(content string) string {
			return strings.Replace(content, ".workflow_run.id == $run_id", ".workflow_run.id != $run_id", 1)
		}, wantFail: true},
		{name: "download by artifact name instead of verified ID", mutate: func(content string) string {
			return strings.Replace(content, "artifact-ids: ${{ steps.provenance.outputs.artifact-id }}", "name: sonar-reports-${{ steps.provenance.outputs.run-id }}-${{ steps.provenance.outputs.run-attempt }}", 1)
		}, wantFail: true},
		{name: "unverified artifact ID", mutate: func(content string) string {
			return strings.Replace(content, "artifact-ids: ${{ steps.provenance.outputs.artifact-id }}", "artifact-ids: ${{ steps.provenance.outputs.run-id }}", 1)
		}, wantFail: true},
		{name: "revision-specific concurrency cannot cancel stale run", mutate: func(content string) string {
			return strings.Replace(content, concurrency, "group: sonar-pr-${{ github.event.workflow_run.pull_requests[0].number }}-${{ github.event.workflow_run.head_sha }}", 1)
		}, wantFail: true},
		{name: "job scoped Sonar secret", mutate: func(content string) string {
			return strings.Replace(content, "    steps:\n", "    env:\n      SONAR_TOKEN: ${{ secrets.SONAR_TOKEN }}\n    steps:\n", 1)
		}, wantFail: true},
		{name: "workflow scoped Sonar secret", mutate: func(content string) string {
			return strings.Replace(content, "jobs:\n", "env:\n  SONAR_TOKEN: ${{ secrets.SONAR_TOKEN }}\njobs:\n", 1)
		}, wantFail: true},
		{name: "untrusted scanner endpoint", mutate: func(content string) string {
			return strings.Replace(content, "-Dsonar.host.url=https://sonarcloud.io", "-Dsonar.host.url=https://attacker.invalid", 1)
		}, wantFail: true},
		{name: "trusted configuration report path suffix drift", mutate: func(content string) string {
			return strings.Replace(content, "sonar.go.coverage.reportPaths=reports/.coverage/out", "sonar.go.coverage.reportPaths=reports/.coverage/out-malicious", 1)
		}, wantFail: true},
		{name: "scanner report path suffix drift", mutate: func(content string) string {
			return strings.Replace(content, "-Dsonar.go.coverage.reportPaths=reports/.coverage/out", "-Dsonar.go.coverage.reportPaths=reports/.coverage/out-malicious", 1)
		}, wantFail: true},
		{name: "duplicate conflicting trusted configuration", mutate: func(content string) string {
			return strings.Replace(content, "          sonar.go.coverage.reportPaths=reports/.coverage/out", "          sonar.go.coverage.reportPaths=reports/.coverage/out\n          sonar.go.coverage.reportPaths=reports/attacker-out", 1)
		}, wantFail: true},
		{name: "duplicate conflicting scanner argument", mutate: func(content string) string {
			return strings.Replace(content, "            -Dsonar.go.coverage.reportPaths=reports/.coverage/out", "            -Dsonar.go.coverage.reportPaths=reports/.coverage/out\n            -Dsonar.go.coverage.reportPaths=reports/attacker-out", 1)
		}, wantFail: true},
		{name: "mutable scanner version", mutate: func(content string) string {
			return strings.Replace(content, "scannerVersion: 8.1.0.6389", "scannerVersion: latest", 1)
		}, wantFail: true},
		{name: "alternate scanner binaries", mutate: func(content string) string {
			return strings.Replace(content, "https://binaries.sonarsource.com/Distribution/sonar-scanner-cli", "https://attacker.invalid/scanner", 1)
		}, wantFail: true},
		{name: "disabled signature verification", mutate: func(content string) string {
			return strings.Replace(content, `skipSignatureVerification: "false"`, `skipSignatureVerification: "true"`, 1)
		}, wantFail: true},
		{name: "post materialization command", mutate: insertBefore("      - name: Scan passive pull-request data with SonarCloud", "      - name: Execute source\n        run: analysis/source/script.sh"), wantFail: true},
		{name: "cache operation", mutate: insertBefore("      - name: Create trusted scanner configuration", "      - name: Restore cache\n        uses: actions/cache@0057852bfaa89a56745cba8c7296529d2fc39830"), wantFail: true},
		{name: "job container", mutate: func(content string) string {
			return strings.Replace(content, "    runs-on: ubuntu-24.04", "    runs-on: ubuntu-24.04\n    container: ubuntu:latest", 1)
		}, wantFail: true},
		{name: "job service", mutate: func(content string) string {
			return strings.Replace(content, "    runs-on: ubuntu-24.04", "    runs-on: ubuntu-24.04\n    services:\n      database:\n        image: postgres:latest", 1)
		}, wantFail: true},
		{name: "alternate scanner action", mutate: func(content string) string {
			return strings.Replace(content, "SonarSource/sonarqube-scan-action@22918119ff8e1ca75a623e15c8296b6ea4fbe28f", "actions/checkout@3d3c42e5aac5ba805825da76410c181273ba90b1", 1)
		}, wantFail: true},
		{name: "equivalent flow trigger syntax", mutate: func(content string) string {
			return strings.Replace(content, trigger, "on: {workflow_run: {workflows: [Lint and Test], types: [completed]}}", 1)
		}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			root := copyWorkflowRoot(t, tc.mutate)
			err := Validate(root)
			if (err != nil) != tc.wantFail {
				t.Fatalf("Validate() error = %v, want failure %v", err, tc.wantFail)
			}
		})
	}
}

func copyWorkflowRoot(t *testing.T, mutate func(string) string) string {
	return copyWorkflowRootFile(t, "trusted-sonarcloud-pr.yml", mutate)
}

func copyWorkflowRootFile(t *testing.T, fileName string, mutate func(string) string) string {
	t.Helper()
	root := t.TempDir()
	workflowDir := filepath.Join(root, ".github", "workflows")
	if err := os.MkdirAll(workflowDir, 0o755); err != nil {
		t.Fatal(err)
	}
	baseWorkflowDir := filepath.Join("..", "..", ".github", "workflows")
	entries, err := os.ReadDir(baseWorkflowDir)
	if err != nil {
		t.Fatal(err)
	}
	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".yml" {
			continue
		}
		contents := targetWorkflowContents(t, baseWorkflowDir, entry.Name())
		if entry.Name() == fileName && mutate != nil {
			contents = []byte(mutate(string(contents)))
		}
		if err := os.WriteFile(filepath.Join(workflowDir, entry.Name()), contents, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return root
}

func TestRunAcceptsCandidateRoot(t *testing.T) {
	directory := t.TempDir()
	workflowDirectory := filepath.Join(directory, ".github", "workflows")
	if err := os.MkdirAll(workflowDirectory, 0o755); err != nil {
		t.Fatal(err)
	}
	// Candidate data intentionally uses a non-repository root. Validation fails
	// because required topology is absent, which proves the flag reaches the data
	// root instead of silently reading the process working directory.
	if err := os.WriteFile(filepath.Join(workflowDirectory, "candidate.yml"), []byte("name: Candidate\non: [push]\npermissions: {contents: read}\njobs: {check: {runs-on: ubuntu-24.04}}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := run([]string{"--candidate-root", directory}, os.Stdout); err == nil {
		t.Fatal("run() unexpectedly accepted an incomplete candidate topology")
	}
}
