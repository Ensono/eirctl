package output_test

import (
	"bytes"
	"strings"
	"testing"

	"github.com/Ensono/eirctl/output"
	"github.com/Ensono/eirctl/task"
)

func TestOutput_prefixedOutputDecorator(t *testing.T) {
	ttests := map[string]struct {
		input  []byte
		expect string
	}{
		"new line added": {
			input:  []byte("lorem ipsum"),
			expect: "\x1b[36mtask1\x1b[0m: lorem ipsum\r\n",
		},
		"contains new lines": {
			input: []byte(`lorem ipsum

multiline stuff`),
			expect: "\x1b[36mtask1\x1b[0m: lorem ipsum\r\n\x1b[36mtask1\x1b[0m: \r\n\x1b[36mtask1\x1b[0m: multiline stuff\r\n",
		},
		"contains new lines with trailing newline": {
			input: []byte(`lorem ipsum
multiline stuff
`),
			expect: "\x1b[36mtask1\x1b[0m: lorem ipsum\r\n\x1b[36mtask1\x1b[0m: multiline stuff\r\n",
		},
	}
	for name, tt := range ttests {
		t.Run(name, func(t *testing.T) {
			runPrefixedOutputTest(t, tt.input, tt.expect)
		})
	}
}

func runPrefixedOutputTest(t *testing.T, input []byte, expectedOutput string) {
	t.Helper()
	buffer := &bytes.Buffer{}
	decorator := output.NewPrefixedOutputWriter(task.NewTask("task1"), buffer)

	if err := decorator.WriteHeader(); err != nil {
		t.Fatal(err)
	}
	assertOutputContains(t, buffer.String(), "Running task task1...")

	if written, err := decorator.Write(input); err != nil && written == 0 {
		t.Fatal(err)
	}
	assertOutputContains(t, buffer.String(), expectedOutput)

	if err := decorator.WriteFooter(); err != nil {
		t.Fatal(err)
	}
	assertOutputContains(t, buffer.String(), "task1 finished")
}

func assertOutputContains(t *testing.T, got string, expected string) {
	t.Helper()
	if !strings.Contains(got, expected) {
		t.Fatalf("got: %s\nwanted: %s\n", got, expected)
	}
}
