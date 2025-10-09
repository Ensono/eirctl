package cmd_test

import (
	"os"
	"testing"
)

func Test_showCommand(t *testing.T) {
	t.Run("errors on args", func(t *testing.T) {
		cmdRunTestHelper(t, &cmdRunTestInput{
			args:    []string{"-c", "testdata/graph.yaml", "show"},
			errored: true,
		})
	})
	t.Run("errors on incorrect task name", func(t *testing.T) {
		cmdRunTestHelper(t, &cmdRunTestInput{
			args:    []string{"-c", "testdata/graph.yaml", "show", "task:unknown"},
			errored: true,
		})
	})
	t.Run("task show succeeds", func(t *testing.T) {
		os.Setenv("EIRCTL_CONFIG_FILE", "testdata/graph.yaml")
		defer os.Unsetenv("EIRCTL_CONFIG_FILE")
		cmdRunTestHelper(t, &cmdRunTestInput{
			args:   []string{"-c", "testdata/graph.yaml", "show", "graph:task1"},
			output: []string{"Task: graph:task1", "echo 'hello, world!'"},
		})
	})
	t.Run("pipeline show succeeds args", func(t *testing.T) {
		os.Setenv("EIRCTL_CONFIG_FILE", "testdata/graph.yaml")
		defer os.Unsetenv("EIRCTL_CONFIG_FILE")
		cmdRunTestHelper(t, &cmdRunTestInput{
			args:   []string{"-c", "testdata/graph.yaml", "show", "graph:pipeline1"},
			output: []string{"Pipeline: graph:pipeline1"},
		})
	})
	t.Run("context show succeeds args", func(t *testing.T) {
		os.Setenv("EIRCTL_CONFIG_FILE", "testdata/graph.yaml")
		defer os.Unsetenv("EIRCTL_CONFIG_FILE")
		cmdRunTestHelper(t, &cmdRunTestInput{
			args:   []string{"-c", "testdata/graph.yaml", "show", "foo"},
			output: []string{"Context: foo", "Image: golang:1.24.3-bookworm"},
		})
	})
	t.Run("imports show correctly with sources in task", func(t *testing.T) {

		cmdRunTestHelper(t, &cmdRunTestInput{
			args:   []string{"-c", "testdata/imports.yaml", "show", "task:task1"},
			output: []string{"- echo 'This is {{index .ArgsList 0}} argument'"},
		})
	})
	t.Run("imports show correctly with sources in context", func(t *testing.T) {
		cmdRunTestHelper(t, &cmdRunTestInput{
			args:   []string{"-c", "testdata/imports.yaml", "show", "sonar"},
			output: []string{"DefinedIn: https://raw.githubusercontent.com/Ensono/eirctl/refs/tags/0.9.1/shared/security/eirctl.yaml"},
		})
	})
}
