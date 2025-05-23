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
			output: []string{"Name: graph:task1", "echo &#39;hello, world!&#39"},
		})
	})
	t.Run("pipeline show succeeds args", func(t *testing.T) {
		os.Setenv("EIRCTL_CONFIG_FILE", "testdata/graph.yaml")
		defer os.Unsetenv("EIRCTL_CONFIG_FILE")
		cmdRunTestHelper(t, &cmdRunTestInput{
			args:   []string{"-c", "testdata/graph.yaml", "show", "graph:pipeline1"},
			output: []string{"Name: graph:pipeline1"},
		})
	})
	t.Run("context show succeeds args", func(t *testing.T) {
		os.Setenv("EIRCTL_CONFIG_FILE", "testdata/graph.yaml")
		defer os.Unsetenv("EIRCTL_CONFIG_FILE")
		cmdRunTestHelper(t, &cmdRunTestInput{
			args:   []string{"-c", "testdata/graph.yaml", "show", "foo"},
			output: []string{"Name: foo", "Image: golang:1.24.3-bookworm"},
		})
	})
}
