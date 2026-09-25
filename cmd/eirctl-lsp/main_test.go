package main

import (
	"bytes"
	"testing"
)

func Test_runMain(t *testing.T) {

	t.Run("run tcp", func(t *testing.T) {
		// Here you would set up the necessary input, output, and error streams
		in, out, errOut := &bytes.Buffer{}, &bytes.Buffer{}, &bytes.Buffer{}
		got := runMain(in, out, errOut)
		
		if got != 0 {
			t.Errorf("runMain() = %v, want %v", got, 0)
		}
	})
}
