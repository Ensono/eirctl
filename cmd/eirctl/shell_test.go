package cmd_test

import (
	"testing"
)

func Test_shellCommand(t *testing.T) {
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
	// Cannot run an integration test from the command level
	// term has too many side effects
	// t.Run("succeeds on args", func(t *testing.T) {
	// 	os.Setenv("EIRCTL_CONFIG_FILE", "testdata/task.yaml")
	// 	defer os.Unsetenv("EIRCTL_CONFIG_FILE")
	// 	os.Stdin.Write([]byte("echo ${FOO}\nexit\n"))
	// 	cmdRunTestHelper(t, &cmdRunTestInput{
	// 		args:   []string{"shell", "context"},
	// 		output: []string{"bar"},
	// 	})
	// })
}
