package schema

import (
	"testing"

	"gopkg.in/yaml.v3"
)

func TestGithubWorkflowUnmarshalTriggerForms(t *testing.T) {
	tests := []struct {
		name     string
		on       string
		triggers []string
	}{
		{
			name:     "scalar",
			on:       "push",
			triggers: []string{"push"},
		},
		{
			name:     "sequence",
			on:       "[workflow_dispatch, repository_dispatch]",
			triggers: []string{"workflow_dispatch", "repository_dispatch"},
		},
		{
			name: "mapping",
			on: `
  pull_request_target:
  issue_comment:
    types: [created]
  workflow_run:
    workflows: [Lint and Test]
    types: [completed]`,
			triggers: []string{"pull_request_target", "issue_comment", "workflow_run"},
		},
		{
			name: "scalar workflow filter",
			on: `
  workflow_run:
    workflows: Lint and Test
    types: [completed]`,
			triggers: []string{"workflow_run"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			contents := "on: " + tt.on + "\njobs:\n  test:\n    runs-on: ubuntu-24.04\n"
			var workflow GithubWorkflow
			if err := yaml.Unmarshal([]byte(contents), &workflow); err != nil {
				t.Fatal(err)
			}
			assertGithubTriggers(t, workflow.On, tt.triggers)
		})
	}
}

func assertGithubTriggers(t *testing.T, events *GithubTriggerEvents, triggers []string) {
	t.Helper()
	for _, trigger := range triggers {
		if !events.Has(trigger) {
			t.Errorf("On.Has(%q) = false", trigger)
		}
	}
	if !events.Has("workflow_run") {
		return
	}
	if got := events.WorkflowRun.Workflows; len(got) != 1 || got[0] != "Lint and Test" {
		t.Fatalf("WorkflowRun.Workflows = %#v", got)
	}
}

func TestGithubJobUnmarshalPolicyFields(t *testing.T) {
	contents := []byte(`on: [push]
jobs:
  scalar:
    runs-on: ubuntu-24.04
    needs: build
    environment: production
    concurrency: deploy-main
    container: ubuntu:24.04
    permissions: {}
    services: {}
    steps:
      - run: echo scalar
  mapping:
    runs-on: [self-hosted, linux]
    needs: [build, test]
    environment:
      name: staging
    concurrency:
      group: deploy-staging
      cancel-in-progress: true
    permissions:
      contents: read
    services:
      database:
        image: postgres:17
    steps:
      - uses: actions/checkout@0123456789012345678901234567890123456789
        with:
          persist-credentials: false
`)
	var workflow GithubWorkflow
	if err := yaml.Unmarshal(contents, &workflow); err != nil {
		t.Fatal(err)
	}

	scalar := workflow.Jobs.Values["scalar"]
	if len(scalar.RunsOn) != 1 || scalar.RunsOn[0] != "ubuntu-24.04" || len(scalar.Needs) != 1 || scalar.Needs[0] != "build" || scalar.Environment != "production" ||
		scalar.Concurrency == nil || scalar.Concurrency.Group != "deploy-main" || scalar.Container == nil ||
		scalar.Container.Image != "ubuntu:24.04" || scalar.Permissions == nil || scalar.Services == nil ||
		!scalar.Has("permissions") || !scalar.Has("services") || !scalar.Has("container") {
		t.Fatalf("scalar job was not normalized: %#v", scalar)
	}

	mapping := workflow.Jobs.Values["mapping"]
	if len(mapping.RunsOn) != 2 || mapping.RunsOn[1] != "linux" || len(mapping.Needs) != 2 || mapping.Needs[1] != "test" || mapping.Environment != "staging" ||
		mapping.Concurrency == nil || mapping.Concurrency.Group != "deploy-staging" || mapping.Permissions["contents"] != "read" ||
		mapping.Services["database"].Image != "postgres:17" || len(mapping.Steps) != 1 ||
		mapping.Steps[0].With["persist-credentials"] != "false" {
		t.Fatalf("mapping job was not normalized: %#v", mapping)
	}
}
