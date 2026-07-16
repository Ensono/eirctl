package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestPrivilegedPRExecutionFixtures(t *testing.T) {
	cases := []struct {
		name string
		want bool
	}{
		{name: "broker", want: false},
		{name: "builder", want: false},
		{name: "publisher", want: false},
		{name: "privileged-checkout-execution", want: true},
		{name: "privileged-cache-poisoning", want: true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			workflow, err := os.ReadFile(filepath.Join("testdata", tc.name+".yml"))
			if err != nil {
				t.Fatal(err)
			}
			if got := hasPrivilegedPRExecution(string(workflow)); got != tc.want {
				t.Fatalf("hasPrivilegedPRExecution() = %v, want %v (trigger=%v checkout=%v execution=%v)", got, tc.want, privilegedTrigger.Match(workflow), prControlledCheckout.Match(workflow), executableStep.Match(workflow))
			}
		})
	}
}
