package genci_test

import (
	"os"
	"testing"

	"github.com/Ensono/eirctl/internal/config"
	"github.com/Ensono/eirctl/internal/genci"
	"github.com/Ensono/eirctl/internal/schema"
	"github.com/Ensono/eirctl/scheduler"
	"github.com/Ensono/eirctl/task"
	"gopkg.in/yaml.v3"
)

func TestGenCi_GithubImpl(t *testing.T) {
	sp, _ := scheduler.NewExecutionGraph("foo",
		scheduler.NewStage("stage1", func(s *scheduler.Stage) {
			s.Pipeline, _ = scheduler.NewExecutionGraph("dev",
				scheduler.NewStage("sub-one", func(s *scheduler.Stage) {
					s.Task = task.NewTask("t2")
					s.Generator = map[string]any{"github": map[string]any{"env": map[string]any{"bar": "${{ secrets.VAR2}}", "foo": "${{ secrets.VAR1}}"}}}
				}),
				scheduler.NewStage("sub-two", func(s *scheduler.Stage) {
					s.Task = task.NewTask("t4")
					s.DependsOn = []string{"t2"}
				}),
				scheduler.NewStage("sub-three", func(s *scheduler.Stage) {
					s.Task = task.NewTask("t5")
					s.DependsOn = []string{"t2", "t4"}
				}),
			)
			s.Generator = map[string]any{"github": map[string]any{"if": "condition1 != false", "environment": "some-env", "runs-on": "my-own-stuff", "env": map[string]any{"bar": "${{ secrets.VAR2}}", "foo": "${{ secrets.VAR1}}"}}}
		}),
		scheduler.NewStage("stage2", func(s *scheduler.Stage) {
			ts1 := task.NewTask("task:dostuff")
			ts1.Generator = map[string]any{"github": map[string]any{"if": "condition2 != false"}}
			s.Task = ts1
			s.DependsOn = []string{"stage1"}
		}))

	gc, err := genci.New("github", &config.Config{
		Pipelines: map[string]*scheduler.ExecutionGraph{"foo": sp},
		Generate: &config.Generator{
			TargetOptions: map[string]any{"github": map[string]any{"on": map[string]any{"push": map[string][]string{"branches": {"foo", "bar"}}}}}},
	})
	if err != nil {
		t.Errorf("failed to generate github, %v\n", err)
	}
	b, err := gc.Convert(sp)
	if err != nil {
		t.Fatal(err)
	}
	if len(b) == 0 {
		t.Fatal("no bytes written")
	}
}

func TestGenCi_GithubImpl_ordering(t *testing.T) {
	workflow := generateNestedWorkflow(t)

	assertWorkflowStepOrder(t, workflow, []workflowStepExpectation{
		{job: "first", index: 2, name: "foo-_first-_one"},
		{job: "first", index: 3, name: "foo-_first-_two"},
		{job: "second", index: 2, name: "foo-_second-_task3"},
		{job: "second", index: 5, name: "foo-_second-_two"},
	})
}

type workflowStepExpectation struct {
	job   string
	index int
	name  string
}

func generateNestedWorkflow(t *testing.T) *schema.GithubWorkflow {
	t.Helper()

	config := genGraphHelper(t, eirctlTesterYaml)
	gc, err := genci.New("github", config)
	if err != nil {
		t.Fatalf("failed to generate github: %v", err)
	}

	bytes, err := gc.Convert(config.Pipelines["foo"])
	if err != nil {
		t.Fatal(err)
	}
	if len(bytes) == 0 {
		t.Fatal("no bytes written")
	}

	workflow := &schema.GithubWorkflow{}
	if err := yaml.Unmarshal(bytes, workflow); err != nil {
		t.Fatal(err)
	}
	return workflow
}

func assertWorkflowStepOrder(t *testing.T, workflow *schema.GithubWorkflow, expectations []workflowStepExpectation) {
	t.Helper()

	for _, expectation := range expectations {
		steps := workflow.Jobs.Values[expectation.job].Steps
		if expectation.index >= len(steps) {
			t.Fatalf("job %q has %d steps, want step %d named %q", expectation.job, len(steps), expectation.index, expectation.name)
		}
		if got := steps[expectation.index].Name; got != expectation.name {
			t.Errorf("job %q step %d name = %q, want %q", expectation.job, expectation.index, got, expectation.name)
		}
	}
}

var eirctlTesterYaml = []byte(`contexts:
  podman:
    container:
      name: alpine:latest
    env: 
      GLOBAL_VAR: this is it
    envfile:
      exclude:
        - HOME

ci_meta:
  targetOpts:
    github:
      "on": 
        push:
          branches:
            - gfooo

pipelines:
  p1:
    - task: one
    - task: two
      depends_on:
        - one
  p2: 
    - task: task3
    - task: task4
      depends_on:
        - task3
    - task: one 
      depends_on:
        - task4
    - task: two
      depends_on:
        - task3
        - task4
        - one

  foo: 
    - name: first 
      pipeline: p1
    - name: second 
      pipeline: p2
      depends_on:
        - first
    - task: task5
      depends_on:
        - second

tasks:
  one:
    command: |
      for i in $(seq 1 5); do
        echo "hello task 1 in env ${ENV_NAME} - iteration $i"
        sleep 0
      done
    context: podman

  two:
    command: |
      echo "hello task 2"
    context: podman

  task3:
    command: 
      - echo "hello, task3 in env ${ENV_NAME}"
    env:
      FOO: bar

  task4:
    command: | 
      echo "hello, task4 in env ${ENV_NAME}"
    context: podman
    env:
      FOO: bar

  task5:
    command:
      - |
        echo "hello, p2 ${FOO} env: ${ENV_NAME:-unknown}"
    context: podman
    env:
      FOO: task1
      GLOBAL_VAR: overwritteninTask
    envfile:
      path: ./cmd/eirctl/testdata/dev.env

  task6:
    command:
      - |
        for i in $(seq 1 5); do
          echo "hello, p2 ${FOO} - env: ${ENV_NAME:-unknown} - iteration $i"
          sleep 0
        done
    env:
      FOO: task2
`)

func genGraphHelper(t *testing.T, configYaml []byte) *config.Config {
	t.Helper()

	tf, err := os.CreateTemp("", "gha-*.yml")
	if err != nil {
		t.Fatal("failed to create a temp file")
	}
	defer os.Remove(tf.Name())
	if _, err := tf.Write(configYaml); err != nil {
		t.Fatal(err)
	}

	cl := config.NewConfigLoader(config.NewConfig())
	cfg, err := cl.Load(tf.Name())
	if err != nil {
		t.Fatal(err)
	}
	return cfg
}
