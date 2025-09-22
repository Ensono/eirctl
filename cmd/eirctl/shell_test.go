package cmd_test

import (
	"os"
	"testing"
)

func Test_shellCommand(t *testing.T) {
	t.Parallel()
	t.Run("errors on args", func(t *testing.T) {
		cmdRunTestHelper(t, &cmdRunTestInput{
			args:    []string{"-c", "testdata/graph.yaml", "shell"},
			errored: true,
		})
	})
	t.Run("errors on incorrect task name", func(t *testing.T) {
		cmdRunTestHelper(t, &cmdRunTestInput{
			args:    []string{"-c", "testdata/graph.yaml", "shell", "context:unknown"},
			errored: true,
		})
	})

	t.Run("fails on incorrect context type", func(t *testing.T) {
		os.Setenv("EIRCTL_CONFIG_FILE", "testdata/task.yaml")
		defer os.Unsetenv("EIRCTL_CONFIG_FILE")
		cmdRunTestHelper(t, &cmdRunTestInput{
			args:    []string{"shell", "context:v1"},
			errored: true,
		})
	})

	// Cannot run an integration test from the command level
	// term has too many side effects
	t.Run("fails in on stdin fd", func(t *testing.T) {
		os.Setenv("EIRCTL_CONFIG_FILE", "testdata/task.yaml")
		defer os.Unsetenv("EIRCTL_CONFIG_FILE")
		// os.Stdin.Write([]byte("echo ${FOO}\nexit\n"))
		cmdRunTestHelper(t, &cmdRunTestInput{
			args:    []string{"shell", "context:v2"},
			errored: true,
		})
	})
}
