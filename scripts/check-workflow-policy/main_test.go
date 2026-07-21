package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
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

func TestParseWorkflowSupportsEquivalentTriggerSyntax(t *testing.T) {
	cases := []string{
		"on: [workflow_dispatch]\npermissions: {contents: read}\njobs: {build: {runs-on: ubuntu-24.04}}\n",
		"on:\n  workflow_dispatch:\npermissions:\n  contents: read\njobs:\n  build:\n    runs-on: ubuntu-24.04\n",
	}
	for _, content := range cases {
		workflow, err := parseWorkflow("fixture.yml", []byte(content))
		if err != nil {
			t.Fatal(err)
		}
		if !workflow.Triggers["workflow_dispatch"] {
			t.Fatalf("workflow triggers = %#v, want workflow_dispatch", workflow.Triggers)
		}
	}
}

func TestWorkflowTopologyAndPermissions(t *testing.T) {
	root := filepath.Clean(filepath.Join("..", ".."))
	if err := Validate(root); err != nil {
		t.Fatalf("Validate(%q): %v", root, err)
	}
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
		contents, err := os.ReadFile(filepath.Join(baseWorkflowDir, entry.Name()))
		if err != nil {
			t.Fatal(err)
		}
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
		contents, err := os.ReadFile(filepath.Join(baseWorkflowDir, entry.Name()))
		if err != nil {
			t.Fatal(err)
		}
		if entry.Name() == "debug-build-request.yml" {
			contents = []byte(strings.Replace(string(contents), "actions: write\n      pull-requests: read", "issues: write", 1))
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
			contents = []byte(strings.Replace(string(contents), "ref: ${{ github.event.pull_request.base.sha }}", "ref: ${{ github.event.pull_request.head.sha }}", 1))
		}
		if err := os.WriteFile(filepath.Join(workflowDir, entry.Name()), contents, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	if err := Validate(root); err == nil || !strings.Contains(err.Error(), "privileged trigger") {
		t.Fatalf("Validate() error = %v, want trusted-boundary rejection", err)
	}
}

func TestMaterializerArchivesOnlyConfigurationData(t *testing.T) {
	contents, err := os.ReadFile(filepath.Join("..", "materialize-policy-candidate"))
	if err != nil {
		t.Fatal(err)
	}
	script := string(contents)
	if !strings.Contains(script, "git archive --format=tar FETCH_HEAD -- .github/workflows eirctl.yaml") || strings.Contains(script, "scripts/check-workflow-policy") {
		t.Fatal("candidate materializer must archive only configuration data, not candidate checker code")
	}
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
