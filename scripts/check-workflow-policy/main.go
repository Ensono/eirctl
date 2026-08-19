package main

import (
	"crypto/sha256"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/Ensono/eirctl/internal/schema"
	"gopkg.in/yaml.v3"
)

const (
	checkoutAction                  = "actions/checkout"
	checkoutActionPrefix            = checkoutAction + "@"
	persistCredentialsField         = "persist-credentials"
	secretExpressionMarker          = "secrets."
	sonarTokenName                  = "SONAR_TOKEN"
	sonarTokenExpression            = "${{ secrets.SONAR_TOKEN }}"
	trustedSonarWorkflowPath        = ".github/workflows/trusted-sonarcloud-pr.yml"
	debugReleaseValidateJob         = "validate-build"
	githubScriptActionPrefix        = "actions/github-script@"
	trustedSonarScannerAction       = "SonarSource/sonarqube-scan-action@22918119ff8e1ca75a623e15c8296b6ea4fbe28f"
	trustedSonarReviewedBounds      = "tree=384,go=160,path=160,file=131072,total=1048576"
	trustedSonarMaterializerPath    = "trusted/scripts/materialize-sonar-source/main.go"
	trustedSonarExpectedConcurrency = "sonar-pr-${{ github.event.workflow_run.pull_requests[0].number || format('{0}-{1}', github.event.workflow_run.head_repository.id, github.event.workflow_run.head_branch) }}"
	debugAuthorizeScriptSHA256      = "e7305c7b816eae30ff48df8753a890a056fd589bc5b7bf7830598063f67cfd32"
	debugBuilderValidateSHA256      = "fd7cbba9697ffaf09134f2dbf91a1e63555c25d5ccdde0e158750c883614952f"
	debugFinalizeValidateSHA256     = "7d7adf685f6e2671749ce89b8db1cab64ddc33b431aed0bf33c7702e8dbddc1d"
	debugArtifactLayoutSHA256       = "aa03ed77bdac4da73f3db9b6242108cdc3272ae8e07603a27f0f3ad1d98169c2"
	debugProvenanceCreateSHA256     = "9814dc1d493ea793ee2cea450d630498d81b2dedfbfaa6ee2062d718b2034142"
	debugPublishValidateSHA256      = "1627ebea3c9c742051dfa75dee11fc125ed9c12617eb6622c73c73aeec22002c"
	debugProvenanceValidateSHA256   = "e3bd42f55585329e0b0003d67dc4e375198b913004d9a0d76f8b8ab15d146e49"
	debugProvenanceRecheckSHA256    = "948040e50d7d7b07589b449c9649d13bfdf5333a8954703503f2f94ba5d044dc"
	legacyDebugRequestSHA256        = "335f46155d347ce80d2fd22b74b7afcb8cb9932bcba9dcca5d2481bea4557d87"
	legacyDebugBuilderSHA256        = "52035aa6741a6f9f518e8ff36b88e9f440165e213fefd09f69db18731d6b77e5"
	legacyDebugPublisherSHA256      = "9fd0a519c99dcf01e55cd1eb8dc1960601beac09283b94fadc3f75360b13874e"
)

var pinnedAction = regexp.MustCompile(`@[0-9a-f]{40}$`)

// Workflow adds source metadata to the shared, typed GitHub Actions model.
// Policy validation consumes schema.GithubWorkflow, GithubJob, and GithubStep
// directly instead of maintaining a second representation of workflow YAML.
type Workflow struct {
	Path         string
	SourceSHA256 string
	schema.GithubWorkflow
}

type Permissions = map[string]string

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
	effectiveEvents := resolveEffectiveRootEvents(workflows)
	legacyDebugTopology := isExactLegacyDebugTopology(workflows)
	for path, workflow := range workflows {
		if legacyDebugTopology && path == ".github/workflows/debug-build.yml" && legacyBuilderHasNoAdditionalCallers(effectiveEvents[path]) {
			continue
		}
		if err := validatePrivilegedFlow(workflows, workflow, effectiveEvents[path]); err != nil {
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
	var definition schema.GithubWorkflow
	if err := yaml.Unmarshal(content, &definition); err != nil {
		return Workflow{}, fmt.Errorf("invalid YAML in %s: %w", path, err)
	}
	if definition.Jobs.Values == nil {
		return Workflow{}, fmt.Errorf("%s needs jobs", path)
	}
	digest := sha256.Sum256(content)
	return Workflow{Path: path, SourceSHA256: fmt.Sprintf("%x", digest), GithubWorkflow: definition}, nil
}

func hasTrigger(workflow Workflow, name string) bool {
	return workflow.On.Has(name)
}

func hasOnlyTriggers(workflow Workflow, expected ...string) bool {
	configured := workflow.On.ConfiguredNames()
	if len(configured) != len(expected) {
		return false
	}
	for _, name := range expected {
		if !workflow.On.Has(name) {
			return false
		}
	}
	return true
}

func hasOnlyEffectiveDebugCaller(workflows map[string]Workflow) bool {
	const builder = ".github/workflows/debug-build.yml"
	events := resolveEffectiveRootEvents(workflows)[builder]
	if len(events) != 1 {
		return false
	}
	for event := range events {
		return event.Name == "issue_comment" && event.SourcePath == ".github/workflows/debug-build-request.yml"
	}
	return false
}

func concurrencyGroup(job schema.GithubJob) string {
	if job.Concurrency == nil {
		return ""
	}
	return job.Concurrency.Group
}

func workflowConcurrencyGroup(workflow Workflow) string {
	if workflow.Concurrency == nil {
		return ""
	}
	return workflow.Concurrency.Group
}

func scalarValue(value any) string {
	if value == nil {
		return ""
	}
	if text, ok := value.(string); ok {
		return text
	}
	return fmt.Sprint(value)
}

func validateWorkflow(workflow Workflow) error {
	if !samePermissions(workflow.Permissions, expectedWorkflowPermissions(workflow.Path)) {
		return fmt.Errorf("%s has unexpected workflow permissions: %#v", workflow.Path, workflow.Permissions)
	}
	for jobID, job := range workflow.Jobs.Values {
		effective := workflow.Permissions
		if job.Has("permissions") {
			effective = job.Permissions
		}
		if !samePermissions(effective, expectedJobPermissions(workflow, jobID)) {
			return fmt.Errorf("%s job %s has unexpected permissions: %#v", workflow.Path, jobID, effective)
		}
		if err := validateActions(workflow.Path, job); err != nil {
			return err
		}
	}
	return rejectScorecardDangerousCheckouts(workflow)
}

func validateActions(path string, job schema.GithubJob) error {
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

func rejectScorecardDangerousCheckouts(workflow Workflow) error {
	if !hasTrigger(workflow, "pull_request_target") && !hasTrigger(workflow, "workflow_run") {
		return nil
	}
	for jobID, job := range workflow.Jobs.Values {
		for _, step := range job.Steps {
			if !actionUses(step.Uses, checkoutAction) {
				continue
			}
			ref := step.With["ref"]
			if strings.Contains(ref, "github.event.pull_request") || strings.Contains(ref, "github.event.workflow_run") {
				return fmt.Errorf("%s job %s uses a Scorecard Dangerous-Workflow dynamic checkout ref", workflow.Path, jobID)
			}
		}
	}
	return nil
}

type jobAuthority struct {
	protectedDefaultBranchSource bool
	repositoryOrSecretPrivilege  bool
	persistentRunner             bool
	defaultBranchCacheWrite      bool
}

var knownReadOnlyRootEvents = map[string]struct{}{
	"pull_request": {}, "pull_request_target": {}, "issue_comment": {}, "workflow_run": {},
}

type effectiveRootEvent struct {
	Name       string
	SourcePath string
}

// GitHub documents these events as the only events that can create or overwrite
// caches in the default branch's scope. Reusable workflows inherit the effective
// root caller event instead of receiving independent workflow_call authority.
// https://docs.github.com/en/actions/using-workflows/caching-dependencies-to-speed-up-workflows#cache-access-for-low-trust-workflow-triggers
func eventWritesDefaultBranchCache(workflow Workflow, event string) bool {
	switch event {
	case "workflow_dispatch", "repository_dispatch", "delete", "registry_package", "page_build", "schedule":
		return true
	case "push":
		return pushCanTargetDefaultBranch(workflow)
	case "workflow_call":
		return false
	default:
		_, documentedReadOnly := knownReadOnlyRootEvents[event]
		return !documentedReadOnly
	}
}

func pushCanTargetDefaultBranch(workflow Workflow) bool {
	if workflow.On == nil {
		return false
	}
	if len(workflow.On.Push.Branches) > 0 {
		included := false
		for _, pattern := range workflow.On.Push.Branches {
			negative := strings.HasPrefix(pattern, "!")
			pattern = strings.TrimPrefix(pattern, "!")
			matched, err := filepath.Match(strings.ReplaceAll(pattern, "**", "*"), "main")
			if err != nil {
				return true
			}
			if matched {
				included = !negative
			}
		}
		return included
	}
	ignored := false
	for _, pattern := range workflow.On.Push.BranchesIgnore {
		matched, err := filepath.Match(strings.ReplaceAll(pattern, "**", "*"), "main")
		if err != nil {
			return true
		}
		if matched {
			ignored = true
		}
	}
	return !ignored
}

func eventUsesProtectedDefaultBranchSource(workflow Workflow, event string) bool {
	if event == "pull_request" {
		return false
	}
	if event == "push" {
		return pushCanTargetDefaultBranch(workflow)
	}
	return true
}

func resolveEffectiveRootEvents(workflows map[string]Workflow) map[string]map[effectiveRootEvent]struct{} {
	events := make(map[string]map[effectiveRootEvent]struct{}, len(workflows))
	for path, workflow := range workflows {
		events[path] = directRootEvents(workflow)
	}
	changed := true
	for changed {
		changed = false
		for callerPath, caller := range workflows {
			for _, job := range caller.Jobs.Values {
				calledPath, ok := localReusableWorkflowPath(job.Uses)
				if !ok {
					continue
				}
				if _, exists := workflows[calledPath]; !exists {
					continue
				}
				for event := range events[callerPath] {
					if _, exists := events[calledPath][event]; !exists {
						events[calledPath][event] = struct{}{}
						changed = true
					}
				}
			}
		}
	}
	return events
}

func directRootEvents(workflow Workflow) map[effectiveRootEvent]struct{} {
	events := map[effectiveRootEvent]struct{}{}
	for _, event := range workflow.On.ConfiguredNames() {
		if event != "workflow_call" {
			events[effectiveRootEvent{Name: event, SourcePath: workflow.Path}] = struct{}{}
		}
	}
	return events
}

func localReusableWorkflowPath(uses string) (string, bool) {
	if !strings.HasPrefix(uses, "./.github/workflows/") || strings.Contains(uses, "@") {
		return "", false
	}
	return strings.TrimPrefix(uses, "./"), true
}

func authorityForJob(workflows map[string]Workflow, workflow Workflow, job schema.GithubJob, events map[effectiveRootEvent]struct{}) jobAuthority {
	authority := jobAuthority{
		repositoryOrSecretPrivilege: jobHasRepositoryOrSecretPrivilege(workflow, job),
		persistentRunner:            runnerMayPersist(job),
	}
	for event := range events {
		source, ok := workflows[event.SourcePath]
		if !ok {
			source = workflow
		}
		authority.protectedDefaultBranchSource = authority.protectedDefaultBranchSource || eventUsesProtectedDefaultBranchSource(source, event.Name)
		authority.defaultBranchCacheWrite = authority.defaultBranchCacheWrite || eventWritesDefaultBranchCache(source, event.Name)
	}
	return authority
}

func runnerMayPersist(job schema.GithubJob) bool {
	if len(job.RunsOn) != 1 {
		return len(job.RunsOn) > 0
	}
	runner := job.RunsOn[0]
	return !strings.HasPrefix(runner, "ubuntu-") && !strings.HasPrefix(runner, "windows-") && !strings.HasPrefix(runner, "macos-")
}

func jobHasRepositoryOrSecretPrivilege(workflow Workflow, job schema.GithubJob) bool {
	permissions := workflow.Permissions
	if job.Has("permissions") {
		permissions = job.Permissions
	}
	for name, value := range permissions {
		if value == "write" || name == "id-token" && value != "none" {
			return true
		}
	}
	return job.Environment != "" || hasSecretInMap(workflow.Env) || hasSecretInMap(job.Env) ||
		hasSecretInStringMap(job.With) || hasSecretReference(job) || jobSecretsConfigured(job) ||
		containsSecretMarker(job.Container) || containsSecretMarker(job.Services)
}

func hasSecretInStringMap(values map[string]string) bool {
	for _, value := range values {
		if strings.Contains(value, secretExpressionMarker) {
			return true
		}
	}
	return false
}

func containsSecretMarker(value any) bool {
	return value != nil && strings.Contains(fmt.Sprintf("%v", value), secretExpressionMarker)
}

func jobSecretsConfigured(job schema.GithubJob) bool {
	if !job.Has("secrets") || job.Secrets == nil {
		return false
	}
	if value, ok := job.Secrets.(string); ok {
		return strings.TrimSpace(value) != ""
	}
	return true
}

func validatePrivilegedFlow(workflows map[string]Workflow, workflow Workflow, events map[effectiveRootEvent]struct{}) error {
	for jobID, job := range workflow.Jobs.Values {
		authority := authorityForJob(workflows, workflow, job, events)
		if _, reusable := localReusableWorkflowPath(job.Uses); reusable && (authority.repositoryOrSecretPrivilege || authority.persistentRunner) {
			return fmt.Errorf("%s job %s calls a reusable workflow with privileged credentials, secrets, environment, or persistent runner authority", workflow.Path, jobID)
		}
		if !jobHasPrivilegedPRExecution(workflow, job) {
			continue
		}
		switch {
		case authority.defaultBranchCacheWrite:
			return fmt.Errorf("%s job %s executes pull-request-controlled content with default-branch cache-write authority", workflow.Path, jobID)
		case authority.repositoryOrSecretPrivilege:
			return fmt.Errorf("%s job %s executes pull-request-controlled content with repository, secret, or environment privilege", workflow.Path, jobID)
		case authority.persistentRunner:
			return fmt.Errorf("%s job %s executes pull-request-controlled content on a persistent runner", workflow.Path, jobID)
		}
	}
	return nil
}

func checkoutStep(steps []*schema.GithubStep) (int, int) {
	for index, step := range steps {
		if actionUses(step.Uses, checkoutAction) {
			return index, index + 1
		}
	}
	return -1, -1
}

func hasUntrustedShellCheckout(step *schema.GithubStep) bool {
	command := strings.ToLower(step.Run)
	checkout := false
	for _, marker := range []string{"git checkout", "git fetch", "git clone", "git switch", "gh pr checkout"} {
		checkout = checkout || strings.Contains(command, marker)
	}
	if !checkout {
		return false
	}
	if strings.Contains(command, "${{") || strings.Contains(command, "$") {
		return true
	}
	for _, value := range step.Env {
		if strings.Contains(scalarValue(value), "${{") {
			return true
		}
	}
	return false
}

func actionUses(value, action string) bool {
	at := strings.LastIndex(value, "@")
	return at > 0 && strings.EqualFold(value[:at], action)
}

func isUntrustedCheckout(step *schema.GithubStep, workflow Workflow, job schema.GithubJob) bool {
	if !actionUses(step.Uses, checkoutAction) {
		return false
	}
	repository := strings.TrimSpace(step.With["repository"])
	ref := strings.TrimSpace(step.With["ref"])
	if strings.Contains(repository, "${{") {
		return true
	}
	if ref == "" || ref == "${{ github.event.pull_request.base.sha }}" {
		return false
	}
	if strings.Contains(ref, "github.event.workflow_run.head_sha") {
		return !trustedWorkflowRun(job.If)
	}
	return strings.Contains(ref, "${{")
}

func trustedWorkflowRun(condition string) bool {
	return strings.Contains(condition, "github.event.workflow_run.event == 'push'") &&
		strings.Contains(condition, "github.event.workflow_run.head_repository.full_name == github.repository") &&
		strings.Contains(condition, "github.event.workflow_run.head_branch == 'main'")
}

func executesWorkspace(step *schema.GithubStep) bool {
	if step.Run != "" || strings.HasPrefix(step.Uses, "./") {
		return true
	}
	if step.Uses == "" || strings.HasPrefix(step.Uses, checkoutActionPrefix) {
		return false
	}
	// A pin proves action identity, not that an action cannot consume workspace
	// content. Only setup-go with caching explicitly disabled is treated as inert.
	return !strings.HasPrefix(step.Uses, "actions/setup-go@") || step.With["cache"] != "false"
}

// hasPrivilegedPRExecution is retained as a narrow, testable fixture helper. It
// does not inspect raw text; it parses the same structural workflow model used by
// Validate and treats any execution after an untrusted checkout as unsafe.
func hasPrivilegedPRExecution(content string) bool {
	workflow, err := parseWorkflow("fixture.yml", []byte(content))
	if err != nil {
		return true
	}
	events := directRootEvents(workflow)
	workflows := map[string]Workflow{workflow.Path: workflow}
	for _, job := range workflow.Jobs.Values {
		authority := authorityForJob(workflows, workflow, job, events)
		if jobHasPrivilegedPRExecution(workflow, job) &&
			(authority.protectedDefaultBranchSource || authority.defaultBranchCacheWrite || authority.repositoryOrSecretPrivilege || authority.persistentRunner) {
			return true
		}
	}
	return false
}

func jobHasPrivilegedPRExecution(workflow Workflow, job schema.GithubJob) bool {
	return executesAfterUntrustedCheckout(workflow, job) || jobHasUntrustedShellCheckout(job)
}

func executesAfterUntrustedCheckout(workflow Workflow, job schema.GithubJob) bool {
	for checkout, step := range job.Steps {
		if !isUntrustedCheckout(step, workflow, job) {
			continue
		}
		for _, subsequent := range job.Steps[checkout+1:] {
			if executesWorkspace(subsequent) {
				return true
			}
		}
	}
	return false
}

func jobHasUntrustedShellCheckout(job schema.GithubJob) bool {
	for _, step := range job.Steps {
		if hasUntrustedShellCheckout(step) {
			return true
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
	return Permissions{"contents": "read"}
}

func expectedJobPermissions(workflow Workflow, job string) Permissions {
	if workflow.Path == ".github/workflows/debug-build-request.yml" && workflow.SourceSHA256 == legacyDebugRequestSHA256 && job == "request" {
		return Permissions{"actions": "write", "pull-requests": "read"}
	}
	allowed := map[string]map[string]Permissions{
		".github/workflows/debug-build-request.yml": {
			"authorize": {"pull-requests": "read"},
			"finalize":  {"actions": "read", "contents": "read"},
		},
		".github/workflows/pr.yml": {
			"report": {"contents": "read", "checks": "write"},
		},
		".github/workflows/publish-debug-release.yml": {
			debugReleaseValidateJob: {"actions": "read", "contents": "read"},
			"publish":               {"actions": "read", "contents": "write"},
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
	}
	if jobs, ok := allowed[workflow.Path]; ok {
		if permissions, ok := jobs[job]; ok {
			return permissions
		}
	}
	return Permissions{"contents": "read"}
}

func validateRepositoryTopology(workflows map[string]Workflow) error {
	if err := validateProtectedPolicyTopology(workflows); err != nil {
		return err
	}
	if isExactLegacyDebugTopology(workflows) {
		return validateNonDebugRepositoryTopology(workflows)
	}
	if err := validateDebugBrokerTopology(workflows); err != nil {
		return err
	}
	if err := validateDebugBuilderTopology(workflows); err != nil {
		return err
	}
	if err := validateDebugPublisherTopology(workflows); err != nil {
		return err
	}
	return validateNonDebugRepositoryTopology(workflows)
}

func legacyBuilderHasNoAdditionalCallers(events map[effectiveRootEvent]struct{}) bool {
	if len(events) != 1 {
		return false
	}
	for event := range events {
		return event.Name == "workflow_dispatch" && event.SourcePath == ".github/workflows/debug-build.yml"
	}
	return false
}

func isExactLegacyDebugTopology(workflows map[string]Workflow) bool {
	expected := map[string]string{
		".github/workflows/debug-build-request.yml":   legacyDebugRequestSHA256,
		".github/workflows/debug-build.yml":           legacyDebugBuilderSHA256,
		".github/workflows/publish-debug-release.yml": legacyDebugPublisherSHA256,
	}
	for path, digest := range expected {
		workflow, ok := workflows[path]
		if !ok || workflow.SourceSHA256 != digest {
			return false
		}
	}
	return true
}

func validateNonDebugRepositoryTopology(workflows map[string]Workflow) error {
	if err := validateReleaseTopology(workflows); err != nil {
		return err
	}
	if err := validateScorecardTopology(workflows); err != nil {
		return err
	}
	return validateTrustedSonarCloudTopology(workflows)
}

func validateProtectedPolicyTopology(workflows map[string]Workflow) error {
	policy, err := requiredWorkflow(workflows, ".github/workflows/trusted-workflow-policy.yml")
	if err != nil {
		return err
	}
	policyJob, ok := policy.Jobs.Values["policy"]
	if !ok || !hasTrigger(policy, "pull_request_target") || len(policyJob.Steps) < 3 ||
		!isProtectedBaseCheckout(policyJob.Steps[0]) ||
		!strings.Contains(policyJob.Steps[2].Run, "scripts/materialize-policy-candidate") ||
		!strings.Contains(policyJob.Steps[2].Run, "go run ./scripts/check-workflow-policy --candidate-root") {
		return errors.New("trusted workflow policy must check out only the implicit protected base revision and inspect candidate configuration as data")
	}
	return nil
}

func validateDebugBrokerTopology(workflows map[string]Workflow) error {
	broker, err := requiredWorkflow(workflows, ".github/workflows/debug-build-request.yml")
	if err != nil {
		return err
	}
	authorize, hasAuthorize := broker.Jobs.Values["authorize"]
	build, hasBuild := broker.Jobs.Values["build"]
	finalize, hasFinalize := broker.Jobs.Values["finalize"]
	if len(broker.Jobs.Values) != 3 || !hasAuthorize || !hasBuild || !hasFinalize || !hasOnlyTriggers(broker, "issue_comment") ||
		workflowConcurrencyGroup(broker) != "debug-build-request-${{ github.event.issue.number }}" || broker.Concurrency == nil || scalarValue(broker.Concurrency.CancelInProgress) != "false" {
		return errors.New("debug build request must be an issue_comment-only workflow serialized per pull request without cancelling an immutable build")
	}
	if strings.Join(strings.Fields(authorize.If), " ") != "github.event.issue.pull_request != null && github.event.comment.body == '/build-debug'" ||
		hasCheckout(authorize) || jobHasUntrustedShellCheckout(authorize) || hasLocalAction(authorize) ||
		!samePermissions(authorize.Permissions, Permissions{"pull-requests": "read"}) ||
		len(authorize.Steps) != 1 || !jobUses(authorize, githubScriptActionPrefix) ||
		!stepDigestMatches(authorize, "Authorize exact debug-build request", "script", debugAuthorizeScriptSHA256) ||
		!stepWithContains(authorize, githubScriptActionPrefix, "script", "getCollaboratorPermissionLevel") ||
		!stepWithContains(authorize, githubScriptActionPrefix, "script", "['write', 'maintain', 'admin']") ||
		!stepWithContains(authorize, githubScriptActionPrefix, "script", "pullRequest.base.repo.full_name !== repository") ||
		authorize.Outputs["pull_request"] != "${{ steps.authorize.outputs.pull_request }}" || authorize.Outputs["commit_sha"] != "${{ steps.authorize.outputs.commit_sha }}" {
		return errors.New("debug build authorization must validate the exact command, maintainer, base repository, pull request, and full current head SHA without checkout")
	}
	if build.Uses != "./.github/workflows/debug-build.yml" || !containsNeed(build.Needs, "authorize") ||
		build.With["pull_request"] != "${{ needs.authorize.outputs.pull_request }}" || build.With["commit_sha"] != "${{ needs.authorize.outputs.commit_sha }}" ||
		!samePermissions(build.Permissions, Permissions{"contents": "read"}) || jobSecretsConfigured(build) || build.Environment != "" || len(build.Steps) != 0 || len(build.RunsOn) != 0 {
		return errors.New("debug build request must pass only validated identity to the local read-only reusable builder without secrets or authority elevation")
	}
	validation := githubScriptIndexContaining(finalize, "github.rest.pulls.get")
	download := actionIndex(finalize, "actions/download-artifact")
	if !containsNeed(finalize.Needs, "authorize") || !containsNeed(finalize.Needs, "build") ||
		!exactDebugFinalizerSteps(finalize) || validation == -1 || download == -1 || validation > download ||
		!isGithubHosted(finalize) || !samePermissions(finalize.Permissions, Permissions{"actions": "read", "contents": "read"}) ||
		!stepDigestMatches(finalize, "Revalidate immutable identity before finalization", "script", debugFinalizeValidateSHA256) ||
		!stepDigestMatches(finalize, "Validate bounded binary layout without execution", "run", debugArtifactLayoutSHA256) ||
		!stepDigestMatches(finalize, "Create trusted clean-runner provenance", "run", debugProvenanceCreateSHA256) ||
		jobHasEnvironment(finalize) || hasSecretReference(finalize) || hasCheckout(finalize) ||
		!stepWithContains(finalize, "actions/download-artifact@", "name", "debug-build-intermediate-${{ github.run_id }}-${{ github.run_attempt }}") ||
		!jobRunContains(finalize, "Validate bounded binary layout without execution", "lstat().st_mode") ||
		!jobRunContains(finalize, "Create trusted clean-runner provenance", "finalized_by") ||
		!stepWithContains(finalize, "actions/upload-artifact@", "name", "debug-build-${{ github.run_id }}") {
		return errors.New("debug build request must finalize bounded opaque binaries and trusted provenance on a fresh read-only runner")
	}
	return nil
}

func validateDebugBuilderTopology(workflows map[string]Workflow) error {
	build, err := requiredWorkflow(workflows, ".github/workflows/debug-build.yml")
	if err != nil {
		return err
	}
	buildJob, ok := build.Jobs.Values["build"]
	if build.On == nil || !ok {
		return errors.New("debug build must define a workflow_call trigger and build job")
	}
	pullInput := build.On.WorkflowCall.Inputs["pull_request"]
	shaInput := build.On.WorkflowCall.Inputs["commit_sha"]
	checkout := checkoutIndexForRef(buildJob, "${{ inputs.commit_sha }}")
	validation := githubScriptIndexContaining(buildJob, "github.rest.pulls.get")
	if len(build.Jobs.Values) != 1 || !hasOnlyTriggers(build, "workflow_call") || !hasOnlyEffectiveDebugCaller(workflows) ||
		len(build.On.WorkflowCall.Secrets) != 0 ||
		!pullInput.Required || pullInput.Type != "string" || !shaInput.Required || shaInput.Type != "string" ||
		checkout == -1 || validation == -1 || validation > checkout || !isGithubHosted(buildJob) ||
		!samePermissions(buildJob.Permissions, Permissions{"contents": "read"}) ||
		!stepDigestMatches(buildJob, "Revalidate pull-request identity before checkout", "script", debugBuilderValidateSHA256) ||
		!stepWithContains(buildJob, githubScriptActionPrefix, "script", "pullRequest.base.repo.full_name !== repository") ||
		!stepWithContains(buildJob, githubScriptActionPrefix, "script", "pullRequest.head.sha.toLowerCase() !== commitSHA.toLowerCase()") ||
		!checkoutDisablesCredentials(buildJob, checkout) || !stepWithContains(buildJob, "actions/setup-go@", "cache", "false") ||
		jobHasEnvironment(buildJob) || hasSecretReference(buildJob) || jobSecretsConfigured(buildJob) ||
		!stepWithContains(buildJob, "actions/upload-artifact@", "name", "debug-build-intermediate-${{ github.run_id }}-${{ github.run_attempt }}") ||
		jobRunContains(buildJob, "", "debug-build-provenance.json") || stepWithContains(buildJob, "actions/upload-artifact@", "name", "debug-build-${{ github.run_id }}") {
		return errors.New("debug build must be a workflow_call-only, GitHub-hosted, read-only builder that revalidates immutable PR identity and uploads only intermediate binaries")
	}
	return nil
}

func validateDebugPublisherTopology(workflows map[string]Workflow) error {
	publish, err := requiredWorkflow(workflows, ".github/workflows/publish-debug-release.yml")
	if err != nil {
		return err
	}
	validate, hasValidate := publish.Jobs.Values[debugReleaseValidateJob]
	publishJob, hasPublish := publish.Jobs.Values["publish"]
	if !hasOnlyTriggers(publish, "workflow_dispatch") || !hasValidate || !hasPublish ||
		validate.If != "github.ref == 'refs/heads/main'" || publishJob.If != "github.ref == 'refs/heads/main'" ||
		!samePermissions(validate.Permissions, Permissions{"actions": "read", "contents": "read"}) || jobHasEnvironment(validate) ||
		!samePermissions(publishJob.Permissions, Permissions{"actions": "read", "contents": "write"}) ||
		publishJob.Environment != "debug-release" || !containsNeed(publishJob.Needs, debugReleaseValidateJob) ||
		!stepDigestMatches(validate, "Validate selected request run, artifact, and PR revision", "script", debugPublishValidateSHA256) ||
		!stepDigestMatches(validate, "Verify clean-runner provenance before publication", "run", debugProvenanceValidateSHA256) ||
		!stepDigestMatches(publishJob, "Recheck provenance on the protected publish runner", "run", debugProvenanceRecheckSHA256) ||
		!stepWithContains(validate, githubScriptActionPrefix, "script", "run.path !== '.github/workflows/debug-build-request.yml'") ||
		!stepWithContains(validate, githubScriptActionPrefix, "script", "run.event !== 'issue_comment'") ||
		!stepWithContains(validate, githubScriptActionPrefix, "script", "run.conclusion !== 'success'") ||
		!stepWithContains(validate, githubScriptActionPrefix, "script", "pr.head.sha.toLowerCase() !== commitSHA.toLowerCase()") ||
		!stepWithContains(validate, githubScriptActionPrefix, "script", "!candidate.expired") ||
		!stepWithContains(validate, githubScriptActionPrefix, "script", "matches.length !== 1") ||
		!stepWithContains(validate, githubScriptActionPrefix, "script", "artifact.workflow_run?.id !== runId") ||
		!stepWithContains(validate, githubScriptActionPrefix, "script", "String(run.run_attempt)") ||
		!stepWithContains(validate, "actions/download-artifact@", "artifact-ids", "${{ steps.validate.outputs.artifact_id }}") ||
		!validDebugProvenanceCheck(validate, "Verify clean-runner provenance before publication") ||
		!stepWithContains(publishJob, "actions/download-artifact@", "artifact-ids", "${{ needs.validate-build.outputs.artifact_id }}") ||
		!validDebugProvenanceCheck(publishJob, "Recheck provenance on the protected publish runner") ||
		!publisherUsesOnlyExpectedBinaries(publishJob) || hasCheckout(validate) || hasCheckout(publishJob) ||
		executesDownloadedContent(validate) || executesDownloadedContent(publishJob) {
		return errors.New("debug publication must validate the successful issue_comment request run and exact clean-runner artifact before its isolated debug-release contents-write job")
	}
	return nil
}

func validateReleaseTopology(workflows map[string]Workflow) error {
	for _, file := range []string{".github/workflows/release.yml", ".github/workflows/release_container.yml"} {
		workflow, err := requiredWorkflow(workflows, file)
		if err != nil {
			return err
		}
		for jobID, job := range workflow.Jobs.Values {
			if !trustedWorkflowRun(job.If) || !hasVerifiedStaticMainCheckout(job) {
				return fmt.Errorf("%s job %s must require a successful trusted push and verify a static protected-main checkout against its workflow-run SHA", file, jobID)
			}
		}
	}
	return nil
}

func validateScorecardTopology(workflows map[string]Workflow) error {
	scorecard, err := requiredWorkflow(workflows, ".github/workflows/scorecard.yml")
	if err != nil {
		return err
	}
	analysis, ok := scorecard.Jobs.Values["analysis"]
	if !ok || !hasCheckoutWithoutCredentials(analysis) {
		return errors.New("scorecard must use job-scoped permissions and a checkout without credentials")
	}
	return nil
}

func validateTrustedSonarCloudTopology(workflows map[string]Workflow) error {
	workflow, err := requiredWorkflow(workflows, trustedSonarWorkflowPath)
	if err != nil {
		return err
	}
	if err := validateTrustedSonarWorkflowEnvelope(workflow); err != nil {
		return err
	}
	job, err := validateTrustedSonarJob(workflow)
	if err != nil {
		return err
	}
	steps, err := parseTrustedSonarSteps(job)
	if err != nil {
		return err
	}
	if err := validateTrustedSonarCheckout(job, steps.checkout); err != nil {
		return err
	}
	if err := validateTrustedSonarProvenance(steps.provenance); err != nil {
		return err
	}
	if err := validateTrustedSonarReports(steps.download, steps.validateReports); err != nil {
		return err
	}
	if err := validateTrustedSonarConfiguration(steps.configure); err != nil {
		return err
	}
	if err := validateTrustedSonarMaterializer(steps.materialize); err != nil {
		return err
	}
	if err := validateTrustedSonarScanner(steps.scanner); err != nil {
		return err
	}
	if !onlyScannerReceivesSonarToken(workflow, job, 6) {
		return errors.New("trusted SonarCloud analyzer must scope SONAR_TOKEN to the approved scanner step")
	}
	return nil
}

type trustedSonarSteps struct {
	checkout        *schema.GithubStep
	provenance      *schema.GithubStep
	download        *schema.GithubStep
	validateReports *schema.GithubStep
	configure       *schema.GithubStep
	materialize     *schema.GithubStep
	scanner         *schema.GithubStep
}

func validateTrustedSonarWorkflowEnvelope(workflow Workflow) error {
	if len(workflow.Jobs.Values) != 1 || !hasTrigger(workflow, "workflow_run") || len(workflow.On.WorkflowRun.Workflows) != 1 || workflow.On.WorkflowRun.Workflows[0] != "Lint and Test" ||
		!samePermissions(workflow.Permissions, Permissions{"contents": "read"}) {
		return errors.New("trusted SonarCloud analyzer must use only the expected read-only workflow_run topology")
	}
	return nil
}

func validateTrustedSonarJob(workflow Workflow) (schema.GithubJob, error) {
	job, ok := workflow.Jobs.Values["analyze"]
	if !ok || job.Environment != "" || job.Has("permissions") || job.Has("container") || job.Has("services") ||
		!strings.Contains(job.If, "github.event.workflow_run.conclusion == 'success'") ||
		!strings.Contains(job.If, "github.event.workflow_run.event == 'pull_request'") ||
		!strings.Contains(job.If, "github.event.workflow_run.repository.full_name == github.repository") ||
		concurrencyGroup(job) != trustedSonarExpectedConcurrency {
		return schema.GithubJob{}, errors.New("trusted SonarCloud analyzer must require a successful PR run, no container or services, and stale-revision-cancelling same-repository or fork concurrency")
	}
	if containsCache(job) || hasLocalAction(job) || hasSecretInMap(workflow.Env) || hasSecretInMap(job.Env) {
		return schema.GithubJob{}, errors.New("trusted SonarCloud analyzer must not use caches, local actions, or workflow/job-scoped secrets")
	}
	return job, nil
}

func parseTrustedSonarSteps(job schema.GithubJob) (trustedSonarSteps, error) {
	if len(job.Steps) != 7 {
		return trustedSonarSteps{}, errors.New("trusted SonarCloud analyzer must use only the seven reviewed helper, provenance, report, materializer, and scanner steps")
	}
	return trustedSonarSteps{
		checkout: job.Steps[0], provenance: job.Steps[1], download: job.Steps[2],
		validateReports: job.Steps[3], configure: job.Steps[4], materialize: job.Steps[5], scanner: job.Steps[6],
	}, nil
}

func validateTrustedSonarCheckout(job schema.GithubJob, checkout *schema.GithubStep) error {
	if checkout.Name != "Check out trusted analyzer helpers" || !actionUses(checkout.Uses, checkoutAction) ||
		checkout.With["ref"] != "main" || checkout.With["fetch-depth"] != "1" ||
		checkout.With[persistCredentialsField] != "false" || checkout.With["path"] != "trusted" ||
		!strings.Contains(checkout.With["sparse-checkout"], "scripts/materialize-sonar-source/main.go") ||
		!strings.Contains(checkout.With["sparse-checkout"], "scripts/validate-sonar-reports.sh") || checkoutCount(job) != 1 {
		return errors.New("trusted SonarCloud analyzer may check out only the protected main helper and report validator")
	}
	return nil
}

func validateTrustedSonarProvenance(provenance *schema.GithubStep) error {
	if provenance.ID != "provenance" || provenance.Name != "Resolve immutable upstream provenance" ||
		!strings.Contains(provenance.Run, "expected_workflow='Lint and Test'") ||
		!strings.Contains(provenance.Run, "expected_event='pull_request'") ||
		!strings.Contains(provenance.Run, "expected_branch='main'") ||
		!strings.Contains(provenance.Run, "actions/runs/${RUN_ID}") ||
		!strings.Contains(provenance.Run, ".head_repository.full_name") ||
		!strings.Contains(provenance.Run, ".head_repository.owner.login") ||
		!strings.Contains(provenance.Run, ".head_branch") ||
		!strings.Contains(provenance.Run, "(length == 0 or length == 1)") ||
		!strings.Contains(provenance.Run, "associated_pr_count == 1") ||
		!strings.Contains(provenance.Run, "gh api --method GET \"repos/${repository}/pulls\"") ||
		!strings.Contains(provenance.Run, "-f state=open") ||
		!strings.Contains(provenance.Run, "-f base=\"$expected_branch\"") ||
		!strings.Contains(provenance.Run, "-f head=\"${head_owner}:${head_ref}\"") ||
		!strings.Contains(provenance.Run, "-f per_page=2") ||
		!strings.Contains(provenance.Run, "if length != 1 then") ||
		!strings.Contains(provenance.Run, "expected exactly one open pull request for verified workflow run head") ||
		!strings.Contains(provenance.Run, "pull request candidate does not match verified workflow run head") ||
		!strings.Contains(provenance.Run, ".state == \"open\"") ||
		!strings.Contains(provenance.Run, ".base.repo.full_name == $repository") ||
		!strings.Contains(provenance.Run, ".base.ref == $branch") ||
		!strings.Contains(provenance.Run, ".head.repo.full_name == $head_repository") ||
		!strings.Contains(provenance.Run, ".head.ref == $head_ref") ||
		!strings.Contains(provenance.Run, ".head.sha == $sha") ||
		!strings.Contains(provenance.Run, "actions/runs/${RUN_ID}/artifacts") ||
		!strings.Contains(provenance.Run, ".workflow_run.id == $run_id") ||
		!strings.Contains(provenance.Run, ".workflow_run.head_sha == $sha") ||
		!strings.Contains(provenance.Run, "expected exactly one current Sonar report artifact") ||
		!strings.Contains(provenance.Run, "head-repository=%s") {
		return errors.New("trusted SonarCloud analyzer must bind workflow, run, attempt, PR, base/head repositories, revision, and report artifact provenance")
	}
	return nil
}

func validateTrustedSonarReports(download, validateReports *schema.GithubStep) error {
	if download.Name != "Download only the verified Sonar report artifact" || !actionUses(download.Uses, "actions/download-artifact") ||
		len(download.With) != 5 || download.With["repository"] != "${{ github.repository }}" ||
		download.With["artifact-ids"] != "${{ steps.provenance.outputs.artifact-id }}" ||
		download.With["run-id"] != "${{ steps.provenance.outputs.run-id }}" ||
		download.With["github-token"] != "${{ github.token }}" || download.With["path"] != "analysis/reports" ||
		validateReports.Name != "Validate bounded passive report artifact" || strings.TrimSpace(validateReports.Run) != "trusted/scripts/validate-sonar-reports.sh analysis/reports" {
		return errors.New("trusted SonarCloud analyzer must download and validate only the exact verified passive report artifact")
	}
	return nil
}

func validateTrustedSonarConfiguration(configure *schema.GithubStep) error {
	expected := strings.TrimSpace(`
mkdir -p analysis
cat >analysis/sonar-project.properties <<'PROPERTIES'
sonar.host.url=https://sonarcloud.io
sonar.organization=ensono
sonar.projectKey=Ensono_eirctl
sonar.scm.provider=git
sonar.sources=source
sonar.tests=source
sonar.inclusions=source/**/*.go
sonar.exclusions=source/**/*_test.go,source/**/*_windows.go,source/**/*_generated*.go,source/**/*_generated/**,source/**/vendor/**,source/**/examples/**
sonar.test.inclusions=source/**/*_test.go
sonar.test.exclusions=source/**/*_generated*.go,source/**/*_generated/**,source/**/vendor/**
sonar.sourceEncoding=UTF-8
sonar.go.coverage.reportPaths=reports/.coverage/out
sonar.go.tests.reportPaths=reports/.coverage/report-junit.xml
sonar.qualitygate.wait=true
PROPERTIES`)
	legacyExpected := strings.Replace(expected, "sonar.scm.provider=git\n", "", 1)
	if configure.Name != "Create trusted scanner configuration" ||
		(strings.TrimSpace(configure.Run) != expected && strings.TrimSpace(configure.Run) != legacyExpected) {
		return errors.New("trusted SonarCloud analyzer must create only the exact forced scanner configuration outside the passive source root")
	}
	return nil
}

func validateTrustedSonarMaterializer(materialize *schema.GithubStep) error {
	expected := strings.Join(strings.Fields("go run "+trustedSonarMaterializerPath+`
		--base-repository "${GITHUB_REPOSITORY}"
		--base-branch main
		--head-repository "${{ steps.provenance.outputs.head-repository }}"
		--head-sha "${{ steps.provenance.outputs.head-sha }}"
		--pull-request "${{ steps.provenance.outputs.pr-number }}"
		--output analysis/source
		--bounds `+trustedSonarReviewedBounds), " ")
	if materialize.Name != "Materialize bounded verified Go source through the Git Data API" || materialize.Uses != "" ||
		strings.Join(strings.Fields(materialize.Run), " ") != expected ||
		len(materialize.Env) != 1 || scalarValue(materialize.Env["GH_TOKEN"]) != "${{ github.token }}" {
		return errors.New("trusted SonarCloud analyzer must use only the protected bounded Git Data API materializer with the verified head repository and SHA")
	}
	return nil
}

func validateTrustedSonarScanner(scanner *schema.GithubStep) error {
	expectedArgs := strings.Join(strings.Fields(`
-Dsonar.projectBaseDir=analysis
-Dsonar.host.url=https://sonarcloud.io
-Dsonar.organization=ensono
-Dsonar.projectKey=Ensono_eirctl
-Dsonar.scm.provider=git
-Dsonar.sources=source
-Dsonar.tests=source
-Dsonar.go.coverage.reportPaths=reports/.coverage/out
-Dsonar.go.tests.reportPaths=reports/.coverage/report-junit.xml
-Dsonar.pullrequest.key=${{ steps.provenance.outputs.pr-number }}
-Dsonar.pullrequest.branch=${{ steps.provenance.outputs.head-ref }}
-Dsonar.pullrequest.base=main
-Dsonar.scm.revision=${{ steps.provenance.outputs.head-sha }}
-Dsonar.qualitygate.wait=true`), " ")
	legacyExpectedArgs := strings.Replace(expectedArgs, "-Dsonar.scm.provider=git ", "", 1)
	args := strings.Join(strings.Fields(scanner.With["args"]), " ")
	if scanner.Name != "Scan passive pull-request data with SonarCloud" || scanner.Uses != trustedSonarScannerAction ||
		scanner.With["scannerVersion"] != "8.1.0.6389" ||
		scanner.With["scannerBinariesUrl"] != "https://binaries.sonarsource.com/Distribution/sonar-scanner-cli" ||
		scanner.With["skipSignatureVerification"] != "false" ||
		(args != expectedArgs && args != legacyExpectedArgs) {
		return errors.New("trusted SonarCloud analyzer must end with only the approved immutable scanner, runtime, endpoint, project, report, PR, revision, and quality-gate settings")
	}
	return nil
}

func checkoutCount(job schema.GithubJob) int {
	count := 0
	for _, step := range job.Steps {
		if actionUses(step.Uses, checkoutAction) {
			count++
		}
	}
	return count
}

func isProtectedBaseCheckout(step *schema.GithubStep) bool {
	return actionUses(step.Uses, checkoutAction) && step.With["ref"] == "" &&
		step.With["fetch-depth"] == "1" && step.With[persistCredentialsField] == "false"
}

func hasVerifiedStaticMainCheckout(job schema.GithubJob) bool {
	if len(job.Steps) < 2 {
		return false
	}
	checkout := job.Steps[0]
	verify := job.Steps[1]
	return actionUses(checkout.Uses, checkoutAction) && checkout.With["ref"] == "main" &&
		checkout.With[persistCredentialsField] == "false" &&
		verify.Name == "Verify the validated workflow-run revision" && len(verify.Env) == 1 &&
		scalarValue(verify.Env["VALIDATED_HEAD_SHA"]) == "${{ github.event.workflow_run.head_sha }}" &&
		strings.TrimSpace(verify.Run) == `test "$(git rev-parse HEAD)" = "$VALIDATED_HEAD_SHA"`
}

func requiredWorkflow(workflows map[string]Workflow, path string) (Workflow, error) {
	workflow, ok := workflows[path]
	if !ok {
		return Workflow{}, fmt.Errorf("required workflow %s is missing", path)
	}
	return workflow, nil
}

func containsCache(job schema.GithubJob) bool {
	for _, step := range job.Steps {
		if strings.HasPrefix(step.Uses, "actions/cache@") {
			return true
		}
	}
	return false
}

func hasLocalAction(job schema.GithubJob) bool {
	for _, step := range job.Steps {
		if strings.HasPrefix(step.Uses, "./") {
			return true
		}
	}
	return false
}

func hasSecretInMap(values map[string]any) bool {
	for _, value := range values {
		if strings.Contains(scalarValue(value), secretExpressionMarker) {
			return true
		}
	}
	return false
}

func onlyScannerReceivesSonarToken(workflow Workflow, job schema.GithubJob, scannerIndex int) bool {
	if hasSecretInMap(workflow.Env) || hasSecretInMap(job.Env) {
		return false
	}
	for index, step := range job.Steps {
		if !scannerStepSecretScopeValid(step, index, scannerIndex) {
			return false
		}
	}
	return true
}

func scannerStepSecretScopeValid(step *schema.GithubStep, index, scannerIndex int) bool {
	for _, value := range step.Env {
		text := scalarValue(value)
		if !strings.Contains(text, sonarTokenName) && !strings.Contains(text, secretExpressionMarker) {
			continue
		}
		if index != scannerIndex || scalarValue(step.Env[sonarTokenName]) != sonarTokenExpression || len(step.Env) != 1 {
			return false
		}
	}
	return !stepContainsForbiddenSonarReference(step)
}

func stepContainsForbiddenSonarReference(step *schema.GithubStep) bool {
	if strings.Contains(step.Run, sonarTokenName) || strings.Contains(step.Uses, sonarTokenName) {
		return true
	}
	for _, value := range step.With {
		if strings.Contains(value, sonarTokenName) || strings.Contains(value, secretExpressionMarker) {
			return true
		}
	}
	return false
}

func jobUses(job schema.GithubJob, prefix string) bool {
	for _, step := range job.Steps {
		if strings.HasPrefix(step.Uses, prefix) {
			return true
		}
	}
	return false
}

func hasCheckout(job schema.GithubJob) bool { _, after := checkoutStep(job.Steps); return after != -1 }

func stepWithContains(job schema.GithubJob, usesPrefix, key, expected string) bool {
	for _, step := range job.Steps {
		if strings.HasPrefix(step.Uses, usesPrefix) && strings.Contains(step.With[key], expected) {
			return true
		}
	}
	return false
}

func checkoutIndexForRef(job schema.GithubJob, ref string) int {
	for index, step := range job.Steps {
		if strings.HasPrefix(step.Uses, checkoutActionPrefix) && step.With["ref"] == ref {
			return index
		}
	}
	return -1
}

func actionIndex(job schema.GithubJob, action string) int {
	for index, step := range job.Steps {
		if actionUses(step.Uses, action) {
			return index
		}
	}
	return -1
}

func checkoutDisablesCredentials(job schema.GithubJob, index int) bool {
	return index >= 0 && index < len(job.Steps) && job.Steps[index].With[persistCredentialsField] == "false"
}

func isGithubHosted(job schema.GithubJob) bool {
	return len(job.RunsOn) == 1 && strings.HasPrefix(job.RunsOn[0], "ubuntu-") && !containsString(job.RunsOn, "self-hosted")
}

func exactDebugFinalizerSteps(job schema.GithubJob) bool {
	if len(job.Steps) != 5 {
		return false
	}
	return stepDigestMatches(job, "Revalidate immutable identity before finalization", "script", debugFinalizeValidateSHA256) &&
		job.Steps[1].Name == "Download only the run-specific intermediate artifact" && actionUses(job.Steps[1].Uses, "actions/download-artifact") &&
		stepDigestMatches(job, "Validate bounded binary layout without execution", "run", debugArtifactLayoutSHA256) &&
		stepDigestMatches(job, "Create trusted clean-runner provenance", "run", debugProvenanceCreateSHA256) &&
		job.Steps[4].Name == "Upload immutable final debug build artifact" && actionUses(job.Steps[4].Uses, "actions/upload-artifact")
}

func stepDigestMatches(job schema.GithubJob, stepName, field, expected string) bool {
	for _, step := range job.Steps {
		if step.Name != stepName {
			continue
		}
		value := step.Run
		if field != "run" {
			value = step.With[field]
		}
		digest := sha256.Sum256([]byte(value))
		return fmt.Sprintf("%x", digest) == expected
	}
	return false
}

func jobRunContains(job schema.GithubJob, stepName, expected string) bool {
	for _, step := range job.Steps {
		if (stepName == "" || step.Name == stepName) && strings.Contains(step.Run, expected) {
			return true
		}
	}
	return false
}

func executesDownloadedContent(job schema.GithubJob) bool {
	download := actionIndex(job, "actions/download-artifact")
	if download == -1 {
		return false
	}
	for _, step := range job.Steps[download+1:] {
		if step.Uses != "" && !actionUses(step.Uses, "softprops/action-gh-release") {
			return true
		}
		command := strings.ToLower(step.Run)
		for _, marker := range []string{
			"source ", "bash ", "sh ", "chmod +x", "go run ", "python ", "python3 ", "exec ",
			"$github_path", "$github_env", "./release-assets", "./validated-artifact",
		} {
			if strings.Contains(command, marker) {
				return true
			}
		}
	}
	return false
}

func validDebugProvenanceCheck(job schema.GithubJob, stepName string) bool {
	for _, step := range job.Steps {
		if step.Name != stepName {
			continue
		}
		for _, marker := range []string{
			".repository == $repository", ".event == $event", ".workflow_path == $workflow_path",
			".pull_request == $pull_request", ".commit_sha | ascii_downcase", ".workflow_run_id == $workflow_run_id",
			".workflow_run_attempt == $workflow_run_attempt", ".final_artifact == $final_artifact",
			".finalized_by == \"finalize\"", "debug-build-intermediate-", ".semver | test(",
		} {
			if !strings.Contains(step.Run, marker) {
				return false
			}
		}
		return true
	}
	return false
}

func publisherUsesOnlyExpectedBinaries(job schema.GithubJob) bool {
	const releaseAction = "softprops/action-gh-release"
	expected := []string{
		"release-assets/bin/eirctl-windows-amd64.exe",
		"release-assets/bin/eirctl-windows-386.exe",
		"release-assets/bin/eirctl-windows-arm64.exe",
		"release-assets/bin/eirctl-darwin-amd64",
		"release-assets/bin/eirctl-darwin-arm64",
		"release-assets/bin/eirctl-linux-arm64",
		"release-assets/bin/eirctl-linux-amd64",
	}
	for _, step := range job.Steps {
		if !actionUses(step.Uses, releaseAction) {
			continue
		}
		files := strings.Fields(step.With["files"])
		if len(files) != len(expected) {
			return false
		}
		actual := make(map[string]struct{}, len(files))
		for _, file := range files {
			actual[file] = struct{}{}
		}
		if len(actual) != len(expected) {
			return false
		}
		for _, file := range expected {
			if _, ok := actual[file]; !ok {
				return false
			}
		}
		return true
	}
	return false
}

func githubScriptIndexContaining(job schema.GithubJob, expected string) int {
	for index, step := range job.Steps {
		if strings.HasPrefix(step.Uses, githubScriptActionPrefix) && strings.Contains(step.With["script"], expected) {
			return index
		}
	}
	return -1
}

func jobHasEnvironment(job schema.GithubJob) bool { return job.Environment != "" }

func hasSecretReference(job schema.GithubJob) bool {
	for _, step := range job.Steps {
		if strings.Contains(step.Run, secretExpressionMarker) || strings.Contains(step.Uses, secretExpressionMarker) {
			return true
		}
		for _, value := range step.With {
			if strings.Contains(value, secretExpressionMarker) {
				return true
			}
		}
		for _, value := range step.Env {
			if strings.Contains(scalarValue(value), secretExpressionMarker) {
				return true
			}
		}
	}
	return false
}

func containsString(values []string, expected string) bool {
	for _, value := range values {
		if value == expected {
			return true
		}
	}
	return false
}

func containsNeed(needs []string, expected string) bool {
	return containsString(needs, expected)
}

func hasCheckoutWithoutCredentials(job schema.GithubJob) bool {
	for _, step := range job.Steps {
		if strings.HasPrefix(step.Uses, checkoutActionPrefix) && step.With[persistCredentialsField] == "false" {
			return true
		}
	}
	return false
}
