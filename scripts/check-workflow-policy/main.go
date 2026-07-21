package main

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"gopkg.in/yaml.v3"
)

var (
	pinnedAction  = regexp.MustCompile(`@[0-9a-f]{40}$`)
	shaExpression = regexp.MustCompile(`(?i)^\$\{\{\s*github\.event\.pull_request\.(head|merge)_sha\s*}}$`)
)

// Workflow is the structural representation used by the policy validators. Values
// that remain expressions are kept as strings, rather than being matched against
// raw YAML, so quoting, flow syntax, and indentation do not affect validation.
type Workflow struct {
	Path             string
	Name             string
	Triggers         map[string]bool
	WorkflowRunNames []string
	Permissions      Permissions
	Env              map[string]string
	Jobs             map[string]Job
}

type Permissions map[string]string

type Job struct {
	Name              string
	If                string
	Needs             []string
	Environment       string
	Permissions       Permissions
	HasJobPermissions bool
	Concurrency       string
	Env               map[string]string
	Steps             []Step
}

type Step struct {
	ID   string
	Name string
	Uses string
	Run  string
	With map[string]string
	Env  map[string]string
}

func main() {
	if err := run(os.Args[1:], os.Stdout); err != nil {
		fmt.Fprintln(os.Stderr, "workflow security check failed:", err)
		os.Exit(1)
	}
}

func run(args []string, stdout io.Writer) error {
	flags := flag.NewFlagSet("check-workflow-policy", flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	candidateRoot := flags.String("candidate-root", ".", "root containing candidate workflow data")
	if err := flags.Parse(args); err != nil {
		return err
	}
	if flags.NArg() != 0 {
		return fmt.Errorf("unexpected arguments: %s", strings.Join(flags.Args(), " "))
	}
	if err := Validate(*candidateRoot); err != nil {
		return err
	}
	_, err := fmt.Fprintln(stdout, "workflow YAML syntax and security policy checks passed")
	return err
}

// Validate evaluates workflow files below root as data. Trusted callers can point
// candidate-root at a separately materialized pull-request tree without changing
// the checker executable, module graph, or working directory.
func Validate(root string) error {
	workflows, err := LoadWorkflows(root)
	if err != nil {
		return err
	}
	if len(workflows) == 0 {
		return errors.New("no workflow files found")
	}
	for _, workflow := range workflows {
		if err := validateWorkflow(workflow); err != nil {
			return err
		}
	}
	return validateRepositoryTopology(workflows)
}

func LoadWorkflows(root string) (map[string]Workflow, error) {
	workflowDir := filepath.Join(root, ".github", "workflows")
	entries, err := os.ReadDir(workflowDir)
	if err != nil {
		return nil, fmt.Errorf("read workflow directory %s: %w", workflowDir, err)
	}
	workflows := make(map[string]Workflow, len(entries))
	for _, entry := range entries {
		if entry.IsDir() || (filepath.Ext(entry.Name()) != ".yml" && filepath.Ext(entry.Name()) != ".yaml") {
			continue
		}
		path := filepath.Join(workflowDir, entry.Name())
		content, err := os.ReadFile(path)
		if err != nil {
			return nil, fmt.Errorf("read %s: %w", path, err)
		}
		workflow, err := parseWorkflow(".github/workflows/"+entry.Name(), content)
		if err != nil {
			return nil, err
		}
		workflows[workflow.Path] = workflow
	}
	return workflows, nil
}

func parseWorkflow(path string, content []byte) (Workflow, error) {
	var document yaml.Node
	if err := yaml.Unmarshal(content, &document); err != nil {
		return Workflow{}, fmt.Errorf("invalid YAML in %s: %w", path, err)
	}
	if len(document.Content) != 1 || document.Content[0].Kind != yaml.MappingNode {
		return Workflow{}, fmt.Errorf("%s must contain a mapping", path)
	}
	root := mapping(document.Content[0])
	workflow := Workflow{
		Path:             path,
		Name:             scalar(root["name"]),
		Triggers:         triggers(root["on"]),
		WorkflowRunNames: workflowRunNames(root["on"]),
		Permissions:      permissions(root["permissions"]),
		Env:              stringMap(root["env"]),
		Jobs:             map[string]Job{},
	}
	jobs := root["jobs"]
	if jobs == nil || jobs.Kind != yaml.MappingNode {
		return Workflow{}, fmt.Errorf("%s needs jobs", path)
	}
	for name, rawJob := range mapping(jobs) {
		if rawJob.Kind != yaml.MappingNode {
			return Workflow{}, fmt.Errorf("%s job %s is not a mapping", path, name)
		}
		values := mapping(rawJob)
		job := Job{
			Name:              name,
			If:                scalar(values["if"]),
			Needs:             stringsValue(values["needs"]),
			Environment:       environment(values["environment"]),
			HasJobPermissions: values["permissions"] != nil,
			Concurrency:       concurrency(values["concurrency"]),
			Env:               stringMap(values["env"]),
			Steps:             steps(values["steps"]),
		}
		if job.HasJobPermissions {
			job.Permissions = permissions(values["permissions"])
		}
		workflow.Jobs[name] = job
	}
	return workflow, nil
}

func mapping(node *yaml.Node) map[string]*yaml.Node {
	result := map[string]*yaml.Node{}
	if node == nil || node.Kind != yaml.MappingNode {
		return result
	}
	for index := 0; index+1 < len(node.Content); index += 2 {
		result[node.Content[index].Value] = node.Content[index+1]
	}
	return result
}

func scalar(node *yaml.Node) string {
	if node == nil || node.Kind != yaml.ScalarNode {
		return ""
	}
	return node.Value
}

func stringsValue(node *yaml.Node) []string {
	if node == nil {
		return nil
	}
	if node.Kind == yaml.ScalarNode {
		return []string{node.Value}
	}
	if node.Kind != yaml.SequenceNode {
		return nil
	}
	result := make([]string, 0, len(node.Content))
	for _, value := range node.Content {
		if value.Kind == yaml.ScalarNode {
			result = append(result, value.Value)
		}
	}
	return result
}

func triggers(node *yaml.Node) map[string]bool {
	result := map[string]bool{}
	if node == nil {
		return result
	}
	switch node.Kind {
	case yaml.ScalarNode:
		result[node.Value] = true
	case yaml.SequenceNode:
		for _, value := range node.Content {
			if value.Kind == yaml.ScalarNode {
				result[value.Value] = true
			}
		}
	case yaml.MappingNode:
		for name := range mapping(node) {
			result[name] = true
		}
	}
	return result
}

func workflowRunNames(node *yaml.Node) []string {
	workflowRun := mapping(node)["workflow_run"]
	return stringsValue(mapping(workflowRun)["workflows"])
}

func permissions(node *yaml.Node) Permissions {
	return Permissions(stringMap(node))
}

func stringMap(node *yaml.Node) map[string]string {
	result := map[string]string{}
	for name, value := range mapping(node) {
		if value.Kind == yaml.ScalarNode {
			result[name] = value.Value
		}
	}
	return result
}

func environment(node *yaml.Node) string {
	if node == nil {
		return ""
	}
	if node.Kind == yaml.ScalarNode {
		return node.Value
	}
	return scalar(mapping(node)["name"])
}

func concurrency(node *yaml.Node) string {
	if node == nil {
		return ""
	}
	if node.Kind == yaml.ScalarNode {
		return node.Value
	}
	return scalar(mapping(node)["group"])
}

func steps(node *yaml.Node) []Step {
	if node == nil || node.Kind != yaml.SequenceNode {
		return nil
	}
	result := make([]Step, 0, len(node.Content))
	for _, rawStep := range node.Content {
		values := mapping(rawStep)
		with := map[string]string{}
		for key, value := range mapping(values["with"]) {
			with[key] = scalar(value)
		}
		result = append(result, Step{
			ID:   scalar(values["id"]),
			Name: scalar(values["name"]),
			Uses: scalar(values["uses"]),
			Run:  scalar(values["run"]),
			With: with,
			Env:  stringMap(values["env"]),
		})
	}
	return result
}

func validateWorkflow(workflow Workflow) error {
	if !samePermissions(workflow.Permissions, expectedWorkflowPermissions(workflow.Path)) {
		return fmt.Errorf("%s has unexpected workflow permissions: %#v", workflow.Path, workflow.Permissions)
	}
	for _, job := range workflow.Jobs {
		effective := workflow.Permissions
		if job.HasJobPermissions {
			effective = job.Permissions
		}
		if !samePermissions(effective, expectedJobPermissions(workflow.Path, job.Name)) {
			return fmt.Errorf("%s job %s has unexpected permissions: %#v", workflow.Path, job.Name, effective)
		}
		if err := validateActions(workflow.Path, job); err != nil {
			return err
		}
	}
	return validatePrivilegedFlow(workflow)
}

func validateActions(path string, job Job) error {
	for _, step := range job.Steps {
		if step.Uses == "" || strings.HasPrefix(step.Uses, "./") {
			continue
		}
		if strings.HasPrefix(step.Uses, "docker://") {
			if !strings.Contains(step.Uses, "@sha256:") {
				return fmt.Errorf("%s has an unpinned Docker action: %s", path, step.Uses)
			}
			continue
		}
		if !pinnedAction.MatchString(step.Uses) {
			return fmt.Errorf("%s has an unpinned action: %s", path, step.Uses)
		}
	}
	return nil
}

func validatePrivilegedFlow(workflow Workflow) error {
	if !isPrivilegedTrigger(workflow.Triggers) {
		return nil
	}
	for _, job := range workflow.Jobs {
		checkout, afterCheckout := checkoutStep(job.Steps)
		if checkout == -1 || !isUntrustedCheckout(job.Steps[checkout], workflow, job) {
			continue
		}
		// pull_request_target executes with the base repository's trust context even
		// when its declared token is read-only, so it must never execute an
		// untrusted checkout. Other privileged paths require write authority or a
		// protected environment before this stricter execution rule applies.
		if !workflow.Triggers["pull_request_target"] && !jobCanWrite(job, workflow) {
			continue
		}
		for _, step := range job.Steps[afterCheckout:] {
			if executesWorkspace(step) {
				return fmt.Errorf("%s job %s checks out and executes pull-request-controlled content from a privileged trigger", workflow.Path, job.Name)
			}
		}
	}
	return nil
}

func isPrivilegedTrigger(values map[string]bool) bool {
	for _, trigger := range []string{"issue_comment", "pull_request_target", "workflow_run", "repository_dispatch", "workflow_dispatch"} {
		if values[trigger] {
			return true
		}
	}
	return false
}

func checkoutStep(steps []Step) (int, int) {
	for index, step := range steps {
		if strings.HasPrefix(step.Uses, "actions/checkout@") {
			return index, index + 1
		}
	}
	return -1, -1
}

func isUntrustedCheckout(step Step, workflow Workflow, job Job) bool {
	ref := strings.TrimSpace(step.With["ref"])
	if ref == "" || shaExpression.MatchString(ref) || ref == "${{ github.event.pull_request.base.sha }}" {
		return false
	}
	if strings.Contains(ref, "github.event.workflow_run.head_sha") {
		return !trustedWorkflowRun(job.If)
	}
	return strings.Contains(ref, "github.event.") || strings.Contains(ref, "inputs.") || strings.Contains(ref, "steps.")
}

func trustedWorkflowRun(condition string) bool {
	return strings.Contains(condition, "github.event.workflow_run.event == 'push'") &&
		strings.Contains(condition, "github.event.workflow_run.head_repository.full_name == github.repository") &&
		strings.Contains(condition, "github.event.workflow_run.head_branch == 'main'")
}

func jobCanWrite(job Job, workflow Workflow) bool {
	permissions := workflow.Permissions
	if job.HasJobPermissions {
		permissions = job.Permissions
	}
	for _, value := range permissions {
		if value == "write" {
			return true
		}
	}
	return job.Environment != ""
}

func executesWorkspace(step Step) bool {
	if step.Run != "" || strings.HasPrefix(step.Uses, "./") {
		return true
	}
	if step.Uses == "" || strings.HasPrefix(step.Uses, "actions/checkout@") {
		return false
	}
	// Actions after an untrusted checkout are fail-closed unless they are known to
	// be inert setup/reporting actions. A pin proves action identity, not that it
	// cannot consume workspace content.
	return !strings.HasPrefix(step.Uses, "actions/setup-go@") &&
		!strings.HasPrefix(step.Uses, "actions/cache@") &&
		!strings.HasPrefix(step.Uses, "actions/upload-artifact@")
}

// hasPrivilegedPRExecution is retained as a narrow, testable fixture helper. It
// does not inspect raw text; it parses the same structural workflow model used by
// Validate and treats any execution after an untrusted checkout as unsafe.
func hasPrivilegedPRExecution(content string) bool {
	workflow, err := parseWorkflow("fixture.yml", []byte(content))
	if err != nil || !isPrivilegedTrigger(workflow.Triggers) {
		return err != nil
	}
	for _, job := range workflow.Jobs {
		checkout, afterCheckout := checkoutStep(job.Steps)
		if checkout == -1 || !isUntrustedCheckout(job.Steps[checkout], workflow, job) {
			continue
		}
		for _, step := range job.Steps[afterCheckout:] {
			if executesWorkspace(step) {
				return true
			}
		}
	}
	return false
}

func samePermissions(actual, expected Permissions) bool {
	if len(actual) != len(expected) {
		return false
	}
	for name, value := range expected {
		if actual[name] != value {
			return false
		}
	}
	return true
}

func expectedWorkflowPermissions(path string) Permissions {
	if path == ".github/workflows/trusted-sonarcloud-pr.yml" {
		return Permissions{"actions": "read", "contents": "read", "pull-requests": "read"}
	}
	return Permissions{"contents": "read"}
}

func expectedJobPermissions(path, job string) Permissions {
	allowed := map[string]map[string]Permissions{
		".github/workflows/debug-build-request.yml": {
			"request": {"actions": "write", "pull-requests": "read"},
		},
		".github/workflows/pr.yml": {
			"report": {"contents": "read", "checks": "write"},
		},
		".github/workflows/publish-debug-release.yml": {
			"validate-build": {"actions": "read", "contents": "read"},
			"publish":        {"actions": "read", "contents": "write"},
		},
		".github/workflows/release.yml": {
			"release": {"contents": "write"},
		},
		".github/workflows/release_container.yml": {
			"build-and-push": {"contents": "read", "packages": "write"},
		},
		".github/workflows/scorecard.yml": {
			"analysis": {"contents": "read", "security-events": "write", "id-token": "write"},
		},
		".github/workflows/trusted-sonarcloud-pr.yml": {
			"analyze": {"actions": "read", "contents": "read", "pull-requests": "read"},
		},
	}
	if jobs, ok := allowed[path]; ok {
		if permissions, ok := jobs[job]; ok {
			return permissions
		}
	}
	return Permissions{"contents": "read"}
}

func validateRepositoryTopology(workflows map[string]Workflow) error {
	broker, err := requiredWorkflow(workflows, ".github/workflows/debug-build-request.yml")
	if err != nil {
		return err
	}
	request, ok := broker.Jobs["request"]
	if !ok || !broker.Triggers["issue_comment"] || request.Concurrency != "debug-build-request-${{ github.event.issue.number }}" ||
		!strings.Contains(request.If, "github.event.comment.body == '/build-debug'") || hasCheckout(request) {
		return errors.New("debug build broker must authorize exact requests, serialize them per PR, and never check out code")
	}
	if !samePermissions(request.Permissions, Permissions{"actions": "write", "pull-requests": "read"}) || !jobUses(request, "actions/github-script@") ||
		!stepWithContains(request, "actions/github-script@", "script", "createWorkflowDispatch") ||
		!stepWithContains(request, "actions/github-script@", "script", "workflow_id: 'debug-build.yml'") ||
		!stepWithContains(request, "actions/github-script@", "script", "ref: 'main'") {
		return errors.New("debug build broker must have only pull-request read and workflow-dispatch authority")
	}

	build, err := requiredWorkflow(workflows, ".github/workflows/debug-build.yml")
	if err != nil {
		return err
	}
	buildJob, ok := build.Jobs["build"]
	if !ok || !build.Triggers["workflow_dispatch"] ||
		!hasCheckoutRef(buildJob, "${{ inputs.commit_sha }}") || !stepWithContains(buildJob, "actions/github-script@", "script", "github.rest.pulls.get") ||
		!stepWithContains(buildJob, "actions/github-script@", "script", "pullRequest.head.sha") ||
		jobHasEnvironment(buildJob) || hasSecretReference(buildJob) {
		return errors.New("debug build must validate dispatched pull-request identity before an immutable read-only checkout without environment or secrets")
	}

	publish, err := requiredWorkflow(workflows, ".github/workflows/publish-debug-release.yml")
	if err != nil {
		return err
	}
	validate, hasValidate := publish.Jobs["validate-build"]
	publishJob, hasPublish := publish.Jobs["publish"]
	if !publish.Triggers["workflow_dispatch"] || !hasValidate || !hasPublish ||
		validate.If != "github.ref == 'refs/heads/main'" || publishJob.If != "github.ref == 'refs/heads/main'" ||
		!samePermissions(validate.Permissions, Permissions{"actions": "read", "contents": "read"}) || jobHasEnvironment(validate) ||
		!samePermissions(publishJob.Permissions, Permissions{"actions": "read", "contents": "write"}) ||
		publishJob.Environment != "debug-release" || !containsNeed(publishJob.Needs, "validate-build") ||
		!stepWithContains(validate, "actions/github-script@", "script", "run.event !== 'workflow_dispatch'") ||
		hasCheckout(validate) || hasCheckout(publishJob) {
		return errors.New("debug publication must validate read-only data before its isolated debug-release contents-write job")
	}

	for _, file := range []string{".github/workflows/release.yml", ".github/workflows/release_container.yml"} {
		workflow, err := requiredWorkflow(workflows, file)
		if err != nil {
			return err
		}
		for _, job := range workflow.Jobs {
			if !trustedWorkflowRun(job.If) {
				return fmt.Errorf("%s job %s must require a successful trusted push from this repository on main", file, job.Name)
			}
		}
	}

	scorecard, err := requiredWorkflow(workflows, ".github/workflows/scorecard.yml")
	if err != nil {
		return err
	}
	analysis, ok := scorecard.Jobs["analysis"]
	if !ok || !hasCheckoutWithoutCredentials(analysis) {
		return errors.New("scorecard must use job-scoped permissions and a checkout without credentials")
	}
	return validateTrustedSonarCloudTopology(workflows)
}

func validateTrustedSonarCloudTopology(workflows map[string]Workflow) error {
	const path = ".github/workflows/trusted-sonarcloud-pr.yml"
	workflow, err := requiredWorkflow(workflows, path)
	if err != nil {
		return err
	}
	if len(workflow.Jobs) != 1 || !workflow.Triggers["workflow_run"] || len(workflow.WorkflowRunNames) != 1 || workflow.WorkflowRunNames[0] != "Lint and Test" ||
		!samePermissions(workflow.Permissions, Permissions{"actions": "read", "contents": "read", "pull-requests": "read"}) {
		return errors.New("trusted SonarCloud analyzer must use only the expected read-only workflow_run topology")
	}

	job, ok := workflow.Jobs["analyze"]
	if !ok || job.Environment != "" || (job.HasJobPermissions && !samePermissions(job.Permissions, workflow.Permissions)) ||
		!strings.Contains(job.If, "github.event.workflow_run.conclusion == 'success'") ||
		!strings.Contains(job.If, "github.event.workflow_run.event == 'pull_request'") ||
		!strings.Contains(job.If, "github.event.workflow_run.repository.full_name == github.repository") ||
		!strings.Contains(job.Concurrency, "github.event.workflow_run.pull_requests[0].number") ||
		!strings.Contains(job.Concurrency, "github.event.workflow_run.head_sha") {
		return errors.New("trusted SonarCloud analyzer must require a successful PR run and serialize each PR revision")
	}
	if containsCache(job) || hasLocalAction(job) || hasSecretInMap(workflow.Env) || hasSecretInMap(job.Env) {
		return errors.New("trusted SonarCloud analyzer must not use caches, local actions, or workflow/job-scoped secrets")
	}

	provenance, ok := stepByID(job, "provenance")
	if !ok || !strings.Contains(provenance.Run, "expected_workflow='Lint and Test'") ||
		!strings.Contains(provenance.Run, "expected_event='pull_request'") ||
		!strings.Contains(provenance.Run, "expected_branch='main'") ||
		!strings.Contains(provenance.Run, "actions/runs/${RUN_ID}") ||
		!strings.Contains(provenance.Run, ".base.repo.full_name == $repository") ||
		!strings.Contains(provenance.Run, ".base.ref == $branch") ||
		!strings.Contains(provenance.Run, ".head.sha == $sha") ||
		!strings.Contains(provenance.Run, "actions/runs/${RUN_ID}/artifacts") ||
		!strings.Contains(provenance.Run, "expected exactly one current Sonar report artifact") {
		return errors.New("trusted SonarCloud analyzer must resolve immutable workflow, PR, revision, and artifact provenance")
	}

	trustedCheckout, trustedCheckoutIndex := stepNamed(job, "Check out trusted report validator")
	download, downloadIndex := stepNamed(job, "Download only the verified Sonar report artifact")
	validateReports, validateIndex := stepNamed(job, "Validate bounded passive report artifact")
	configure, configureIndex := stepNamed(job, "Create trusted scanner configuration")
	sourceCheckout, sourceCheckoutIndex := stepNamed(job, "Materialize verified pull-request source as passive data")
	scanner, scannerIndex := stepNamed(job, "Scan passive pull-request data with SonarCloud")
	if trustedCheckoutIndex != 0 || !strings.HasPrefix(trustedCheckout.Uses, "actions/checkout@") || trustedCheckout.With["ref"] != "main" ||
		trustedCheckout.With["persist-credentials"] != "false" || trustedCheckout.With["path"] != "trusted" ||
		downloadIndex <= trustedCheckoutIndex || !strings.HasPrefix(download.Uses, "actions/download-artifact@") ||
		download.With["repository"] != "${{ github.repository }}" || !strings.Contains(download.With["run-id"], "steps.provenance.outputs.run-id") ||
		validateIndex != downloadIndex+1 || !strings.Contains(validateReports.Run, "trusted/scripts/validate-sonar-reports.sh analysis/reports") ||
		configureIndex != validateIndex+1 || !strings.Contains(configure.Run, "analysis/sonar-project.properties") ||
		sourceCheckoutIndex != configureIndex+1 || !strings.HasPrefix(sourceCheckout.Uses, "actions/checkout@") ||
		sourceCheckout.With["ref"] != "${{ steps.provenance.outputs.head-sha }}" || sourceCheckout.With["persist-credentials"] != "false" || sourceCheckout.With["path"] != "analysis/source" ||
		scannerIndex != sourceCheckoutIndex+1 || scannerIndex != len(job.Steps)-1 || scanner.Uses != "SonarSource/sonarqube-scan-action@22918119ff8e1ca75a623e15c8296b6ea4fbe28f" ||
		!strings.Contains(scanner.With["args"], "-Dsonar.projectBaseDir=analysis") || !strings.Contains(scanner.With["args"], "-Dsonar.host.url=https://sonarcloud.io") ||
		!strings.Contains(scanner.With["args"], "-Dsonar.organization=ensono") || !strings.Contains(scanner.With["args"], "-Dsonar.projectKey=Ensono_eirctl") ||
		!strings.Contains(scanner.With["args"], "-Dsonar.pullrequest.key=${{ steps.provenance.outputs.pr-number }}") ||
		!strings.Contains(scanner.With["args"], "-Dsonar.pullrequest.base=main") || !strings.Contains(scanner.With["args"], "-Dsonar.scm.revision=${{ steps.provenance.outputs.head-sha }}") ||
		!strings.Contains(scanner.With["args"], "-Dsonar.qualitygate.wait=true") {
		return errors.New("trusted SonarCloud analyzer must validate passive inputs before its isolated immutable checkout and approved scanner")
	}
	if !onlyScannerReceivesSonarToken(workflow, job, scannerIndex) {
		return errors.New("trusted SonarCloud analyzer must scope SONAR_TOKEN to the approved scanner step")
	}
	return nil
}

func requiredWorkflow(workflows map[string]Workflow, path string) (Workflow, error) {
	workflow, ok := workflows[path]
	if !ok {
		return Workflow{}, fmt.Errorf("required workflow %s is missing", path)
	}
	return workflow, nil
}

func stepByID(job Job, id string) (Step, bool) {
	for _, step := range job.Steps {
		if step.ID == id {
			return step, true
		}
	}
	return Step{}, false
}

func stepNamed(job Job, name string) (Step, int) {
	for index, step := range job.Steps {
		if step.Name == name {
			return step, index
		}
	}
	return Step{}, -1
}

func containsCache(job Job) bool {
	for _, step := range job.Steps {
		if strings.HasPrefix(step.Uses, "actions/cache@") {
			return true
		}
	}
	return false
}

func hasLocalAction(job Job) bool {
	for _, step := range job.Steps {
		if strings.HasPrefix(step.Uses, "./") {
			return true
		}
	}
	return false
}

func hasSecretInMap(values map[string]string) bool {
	for _, value := range values {
		if strings.Contains(value, "secrets.") {
			return true
		}
	}
	return false
}

func onlyScannerReceivesSonarToken(workflow Workflow, job Job, scannerIndex int) bool {
	if hasSecretInMap(workflow.Env) || hasSecretInMap(job.Env) {
		return false
	}
	for index, step := range job.Steps {
		for _, value := range step.Env {
			if strings.Contains(value, "SONAR_TOKEN") || strings.Contains(value, "secrets.") {
				if index != scannerIndex || step.Env["SONAR_TOKEN"] != "${{ secrets.SONAR_TOKEN }}" || len(step.Env) != 1 {
					return false
				}
			}
		}
		if strings.Contains(step.Run, "SONAR_TOKEN") || strings.Contains(step.Uses, "SONAR_TOKEN") {
			return false
		}
		for _, value := range step.With {
			if strings.Contains(value, "SONAR_TOKEN") || strings.Contains(value, "secrets.") {
				return false
			}
		}
	}
	return true
}

func jobUses(job Job, prefix string) bool {
	for _, step := range job.Steps {
		if strings.HasPrefix(step.Uses, prefix) {
			return true
		}
	}
	return false
}

func hasCheckout(job Job) bool { _, after := checkoutStep(job.Steps); return after != -1 }

func stepWithContains(job Job, usesPrefix, key, expected string) bool {
	for _, step := range job.Steps {
		if strings.HasPrefix(step.Uses, usesPrefix) && strings.Contains(step.With[key], expected) {
			return true
		}
	}
	return false
}

func hasCheckoutRef(job Job, ref string) bool {
	for _, step := range job.Steps {
		if strings.HasPrefix(step.Uses, "actions/checkout@") && step.With["ref"] == ref {
			return true
		}
	}
	return false
}

func jobHasEnvironment(job Job) bool { return job.Environment != "" }

func hasSecretReference(job Job) bool {
	for _, step := range job.Steps {
		if strings.Contains(step.Run, "secrets.") || strings.Contains(step.Uses, "secrets.") {
			return true
		}
		for _, value := range step.With {
			if strings.Contains(value, "secrets.") {
				return true
			}
		}
	}
	return false
}

func containsNeed(needs []string, expected string) bool {
	for _, need := range needs {
		if need == expected {
			return true
		}
	}
	return false
}

func hasCheckoutWithoutCredentials(job Job) bool {
	for _, step := range job.Steps {
		if strings.HasPrefix(step.Uses, "actions/checkout@") && step.With["persist-credentials"] == "false" {
			return true
		}
	}
	return false
}
