package main

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"
)

var pinnedAction = regexp.MustCompile(`@[0-9a-f]{40}$`)

func fail(format string, args ...any) error { return fmt.Errorf(format, args...) }

func asMap(value any) map[string]any {
	result, _ := value.(map[string]any)
	return result
}

func stringValue(values map[string]any, key string) string {
	value, _ := values[key].(string)
	return value
}

func walkActions(file string, value any) error {
	switch value := value.(type) {
	case map[string]any:
		for key, child := range value {
			if key == "uses" {
				use, ok := child.(string)
				if ok && !strings.HasPrefix(use, "./") && !pinnedAction.MatchString(use) {
					return fail("%s has an unpinned action: %s", file, use)
				}
			}
			if err := walkActions(file, child); err != nil {
				return err
			}
		}
	case []any:
		for _, child := range value {
			if err := walkActions(file, child); err != nil {
				return err
			}
		}
	}
	return nil
}

func permissions(value any) map[string]any { return asMap(value) }

func permissionIs(values map[string]any, key, expected string) bool {
	return stringValue(values, key) == expected
}

func allRead(values map[string]any) bool {
	for _, value := range values {
		if value != "read" {
			return false
		}
	}
	return true
}

func hasWrite(values map[string]any) bool {
	for _, value := range values {
		if value == "write" {
			return true
		}
	}
	return false
}

func samePermissions(actual, expected map[string]any) bool {
	if len(actual) != len(expected) {
		return false
	}
	for key, value := range expected {
		if actual[key] != value {
			return false
		}
	}
	return true
}

func expectedJobPermissions(file, job string) map[string]any {
	allowed := map[string]map[string]map[string]any{
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
	}
	if workflowJobs, ok := allowed[file]; ok {
		if permissions, ok := workflowJobs[job]; ok {
			return permissions
		}
	}
	return map[string]any{"contents": "read"}
}

func main() {
	files, err := filepath.Glob(".github/workflows/*.yml")
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	if yamlFiles, _ := filepath.Glob(".github/workflows/*.yaml"); len(yamlFiles) > 0 {
		files = append(files, yamlFiles...)
	}
	sort.Strings(files)
	if len(files) == 0 {
		fmt.Fprintln(os.Stderr, "workflow security check failed: no workflow files found")
		os.Exit(1)
	}
	documents := map[string]map[string]any{}
	for _, file := range files {
		content, readErr := os.ReadFile(file)
		if readErr != nil {
			fmt.Fprintln(os.Stderr, readErr)
			os.Exit(1)
		}
		doc := map[string]any{}
		if parseErr := yaml.Unmarshal(content, &doc); parseErr != nil {
			fmt.Fprintf(os.Stderr, "workflow security check failed: invalid YAML in %s: %v\n", file, parseErr)
			os.Exit(1)
		}
		rootPermissions, ok := doc["permissions"]
		if !ok || permissions(rootPermissions) == nil {
			fmt.Fprintf(os.Stderr, "workflow security check failed: %s needs an explicit permissions block\n", file)
			os.Exit(1)
		}
		rootPermissionMap := permissions(rootPermissions)
		if !samePermissions(rootPermissionMap, map[string]any{"contents": "read"}) {
			fmt.Fprintf(os.Stderr, "workflow security check failed: %s must declare exactly contents: read at workflow scope\n", file)
			os.Exit(1)
		}
		jobs, ok := doc["jobs"].(map[string]any)
		if !ok || len(jobs) == 0 {
			fmt.Fprintf(os.Stderr, "workflow security check failed: %s needs jobs\n", file)
			os.Exit(1)
		}
		for jobName, rawJob := range jobs {
			job, ok := rawJob.(map[string]any)
			if !ok {
				fmt.Fprintf(os.Stderr, "workflow security check failed: %s job %s is not a mapping\n", file, jobName)
				os.Exit(1)
			}
			effective := rootPermissionMap
			if rawPermissions, hasJobPermissions := job["permissions"]; hasJobPermissions {
				effective = permissions(rawPermissions)
			}
			if !samePermissions(effective, expectedJobPermissions(file, jobName)) {
				fmt.Fprintf(os.Stderr, "workflow security check failed: %s job %s has unexpected permissions: %#v\n", file, jobName, effective)
				os.Exit(1)
			}
		}
		if policyErr := walkActions(file, doc); policyErr != nil {
			fmt.Fprintln(os.Stderr, "workflow security check failed:", policyErr)
			os.Exit(1)
		}
		documents[file] = doc
	}

	buildText := readFile(".github/workflows/debug-build.yml")
	publishText := readFile(".github/workflows/publish-debug-release.yml")
	build := documents[".github/workflows/debug-build.yml"]
	publish := documents[".github/workflows/publish-debug-release.yml"]
	buildJob := asMap(asMap(build["jobs"])["build"])
	buildPermissions := permissions(buildJob["permissions"])
	if buildPermissions == nil {
		buildPermissions = permissions(build["permissions"])
	}
	if !allRead(buildPermissions) || buildJob["environment"] != nil {
		stop("debug build must be read-only and must not receive an environment")
	}
	for _, check := range []string{"pr.data.head.sha", "ref: ${{ steps.pull-request.outputs.sha }}", "debug-build-provenance.json"} {
		if !strings.Contains(buildText, check) {
			stop("debug build is missing %s", check)
		}
	}
	if !regexp.MustCompile(`actions/upload-artifact@[0-9a-f]{40}`).MatchString(buildText) || strings.Contains(buildText, "softprops/") {
		stop("debug build must upload a pinned artifact and never publish a release")
	}

	jobs := asMap(publish["jobs"])
	validateJob := asMap(jobs["validate-build"])
	publishJob := asMap(jobs["publish"])
	validatePermissions := permissions(validateJob["permissions"])
	if len(validatePermissions) != 2 || !permissionIs(validatePermissions, "actions", "read") || !permissionIs(validatePermissions, "contents", "read") || validateJob["environment"] != nil {
		stop("debug publication validation must have read-only contents/artifact access and no environment")
	}
	if publishJob["needs"] != "validate-build" || stringValue(permissions(publishJob["permissions"]), "contents") != "write" || hasWrite(validatePermissions) {
		stop("debug publication must validate before its isolated contents write job")
	}
	if publishJob["environment"] != "debug-release" {
		stop("debug publication must require the protected debug-release environment")
	}
	for _, check := range []string{"run.event", "run.conclusion", "run.repository?.full_name", "pr.base.repo.full_name", "pr.head.sha", "debug-build-provenance.json", "workflow_run_attempt"} {
		if !strings.Contains(publishText, check) {
			stop("debug publication is missing %s validation", check)
		}
	}
	if strings.Contains(publishText, "actions/checkout") || strings.Contains(publishText, "go run") || strings.Contains(publishText, "bash release-assets") {
		stop("debug publication must not check out or execute untrusted code")
	}
	if strings.Count(publishText, "artifact-ids:") != 2 || strings.Count(publishText, "merge-multiple: true") != 2 {
		stop("debug publication must download the validated artifact by ID and merge its archive contents")
	}

	pr := documents[".github/workflows/pr.yml"]
	prJobs := asMap(pr["jobs"])
	testPermissions := permissions(asMap(prJobs["test"])["permissions"])
	if len(testPermissions) != 1 || !permissionIs(testPermissions, "contents", "read") || strings.Contains(readFile(".github/workflows/pr.yml"), "SONAR_TOKEN") {
		stop("PR execution must have contents read only and no Sonar secret")
	}
	report := asMap(prJobs["report"])
	reportPermissions := permissions(report["permissions"])
	if report["needs"] != "test" || len(reportPermissions) != 2 || !permissionIs(reportPermissions, "checks", "write") || !permissionIs(reportPermissions, "contents", "read") {
		stop("PR reporting must be isolated to the inert test artifact and checks permission")
	}

	for _, file := range []string{".github/workflows/release.yml", ".github/workflows/release_container.yml"} {
		text := readFile(file)
		if !strings.Contains(text, "github.event.workflow_run.event == 'push'") || !strings.Contains(text, "github.event.workflow_run.head_repository.full_name == github.repository") || !strings.Contains(text, "github.event.workflow_run.head_branch == 'main'") {
			stop("%s must require a successful trusted push from this repository on main", file)
		}
		if !strings.Contains(text, "ref: ${{ github.event.workflow_run.head_sha }}") || !strings.Contains(text, "Revision=${{ github.event.workflow_run.head_sha }}") {
			stop("%s must check out and build the exact validated workflow-run head SHA", file)
		}
		for name, rawJob := range asMap(documents[file]["jobs"]) {
			job := asMap(rawJob)
			if hasWrite(permissions(job["permissions"])) && name != "release" && name != "build-and-push" {
				stop("%s %s has an unallowlisted write permission", file, name)
			}
		}
	}

	scorecard := asMap(asMap(documents[".github/workflows/scorecard.yml"]["jobs"])["analysis"])
	if !strings.Contains(readFile(".github/workflows/scorecard.yml"), "persist-credentials: false") || len(permissions(scorecard["permissions"])) != 3 {
		stop("Scorecard must use pinned actions, job-scoped permissions, and no checkout credentials")
	}
	fmt.Println("workflow YAML syntax and security policy checks passed")
}

func readFile(path string) string {
	content, err := os.ReadFile(path)
	if err != nil {
		stop("read %s: %v", path, err)
	}
	return string(content)
}

func stop(format string, args ...any) {
	fmt.Fprintf(os.Stderr, "workflow security check failed: "+format+"\n", args...)
	os.Exit(1)
}
