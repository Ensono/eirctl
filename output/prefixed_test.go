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
			b := &bytes.Buffer{}

			dec := output.NewPrefixedOutputWriter(task.NewTask("task1"), b)
			err := dec.WriteHeader()
			if err != nil {
				t.Fatal(err)
			}

			if !strings.Contains(b.String(), "Running task task1...") {
				t.Fatal()
			}

			n, err := dec.Write(tt.input)
			if err != nil && n == 0 {
				t.Fatal()
			}
			if !strings.Contains(b.String(), tt.expect) {
				t.Fatalf("got: %s\nwanted: %s\n", b.String(), tt.expect)
			}

			err = dec.WriteFooter()
			if err != nil {
				t.Fatal(err)
			}

			if !strings.Contains(b.String(), "task1 finished") {
				t.Fatal()
			}
		})
	}
}
