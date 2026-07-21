package cmd_test

import (
	"testing"
)

func Test_Update(t *testing.T) {
	t.Run("Update Command has been added to the cmd tree", func(t *testing.T) {
		cmdRunTestHelper(t, &cmdRunTestInput{
			args:    []string{"update", "--help"},
			errored: false,
		})
	})
}
