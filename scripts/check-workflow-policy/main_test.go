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
	job := workflow.Jobs["analyze"]
	step := job.Steps[0]
	if workflow.Env["WORKFLOW_TOKEN"] != "safe" || job.Env["JOB_TOKEN"] != "safe" ||
		job.Concurrency != "analyzer-${{ github.run_id }}" || step.Name != "Scan passive inputs" ||
		step.Env["SONAR_TOKEN"] != "${{ secrets.SONAR_TOKEN }}" || step.With["args"] != "-Dsonar.projectBaseDir=analysis" {
		t.Fatalf("trusted topology fields were not preserved: %#v", workflow)
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
				if entry.Name() == tc.file {
					contents = []byte(strings.Replace(string(contents), tc.oldValue, tc.newValue, 1))
				}
				if err := os.WriteFile(filepath.Join(workflowDir, entry.Name()), contents, 0o644); err != nil {
					t.Fatal(err)
				}
			}
			if err := Validate(root); err == nil {
				t.Fatal("Validate() unexpectedly accepted a dangerous release checkout")
			}
		})
	}
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
			return strings.Replace(content, "group: sonar-pr-${{ github.event.workflow_run.pull_requests[0].number }}", "group: sonar-pr-${{ github.event.workflow_run.pull_requests[0].number }}-${{ github.event.workflow_run.head_sha }}", 1)
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
		if entry.Name() == "trusted-sonarcloud-pr.yml" && mutate != nil {
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
