package cmd_test

import (
	"context"
	"os"
	"testing"
	"time"
)

func Test_watchCommand(t *testing.T) {
	t.Run("cancelled by user", func(t *testing.T) {
		defer os.Unsetenv("EIRCTL_CONFIG_FILE")
		ctx, cancel := context.WithCancel(context.Background())
		go func() {
			time.Sleep(100 * time.Millisecond)
			cancel()
		}()
		cmdRunTestHelper(t, &cmdRunTestInput{
			args:    []string{"-c", "testdata/watch.yaml", "watch", "watch:watcher1"},
			errored: false,
			ctx:     ctx})
	})
}
