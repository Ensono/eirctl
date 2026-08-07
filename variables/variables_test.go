package variables_test

import (
	"testing"

	"github.com/Ensono/eirctl/variables"
)

func TestNewVariables(t *testing.T) {
	vars1 := variables.FromMap(map[string]string{"a": "1", "b": "2"})

	if vars1.Get("a") != "1" {
		t.Fatal("get test failed")
	}

	vars2 := variables.NewVariables()
	vars2.Set("c", "3")
	vars2.Set("d", "4")

	vars3 := vars2.With("e", "5")
	if vars3.Get("e") != "5" {
		t.Fatal("with test failed")
	}

	if vars2.Get("d") != "4" || vars3.Get("d") != "4" {
		t.Fatal("with test failed")
	}

	vars1 = vars1.Merge(vars2)
	if vars1.Get("a") != "1" || vars1.Get("c") != "3" {
		t.Fatal("merge test failed")
	}

	if vars2.Get("c") != "3" {
		t.Fatal("merge test failed")
	}

	if !vars2.Has("d") {
		t.Fatal()
	}

	// test overwrite
	vars2.Set("a", "overwritten")
	varsMergedOverwrite := vars1.Merge(vars2)
	if varsMergedOverwrite.Get("a") != "overwritten" {
		t.Fatalf("merge test overwrite failed, got %v, want: 'overwritten'", varsMergedOverwrite.Get("a"))
	}

}

func TestVariables_MergeV2(t *testing.T) {
	tests := map[string]struct {
		currentVar    *variables.Variables
		overwriteVars *variables.Variables
		expected      map[string]string
	}{
		"value is overwritten": {
			currentVar:    variables.FromMap(map[string]string{"original": "ignore", "untouched": "foo"}),
			overwriteVars: variables.FromMap(map[string]string{"original": "new"}),
			expected:      map[string]string{"original": "new", "untouched": "foo"},
		},
		"value is left": {
			currentVar:    variables.FromMap(map[string]string{"original": "ignore", "untouched": "foo"}),
			overwriteVars: variables.FromMap(map[string]string{"foo": "bar"}),
			expected:      map[string]string{"original": "ignore", "untouched": "foo", "foo": "bar"},
		},
		"value is merged with nothing to overwrite": {
			currentVar:    variables.FromMap(map[string]string{"original": "ignore", "untouched": "foo"}),
			overwriteVars: variables.FromMap(map[string]string{"foo": "bar"}),
			expected:      map[string]string{"original": "ignore", "untouched": "foo", "foo": "bar"},
		},
	}
	for name, testCase := range tests {
		t.Run(name, func(t *testing.T) {
			assertVariablesEqual(t, testCase.currentVar.Merge(testCase.overwriteVars), testCase.expected)
		})
	}

	t.Run("check chaining by precedence", func(t *testing.T) {
		mainVar := variables.FromMap(map[string]string{}).
			Merge(variables.FromMap(map[string]string{"foo": "bar", "some": "123"})).
			Merge(variables.FromMap(map[string]string{"baz": "qux", "some": "456"}))

		assertVariablesEqual(t, mainVar, map[string]string{"foo": "bar", "some": "456", "baz": "qux"})
	})
}

func assertVariablesEqual(t *testing.T, got *variables.Variables, expected map[string]string) {
	t.Helper()
	actual := got.Map()
	if len(actual) != len(expected) {
		t.Fatalf("got %d keys, wanted %d: got %#v", len(actual), len(expected), actual)
	}
	for key, expectedValue := range expected {
		if actualValue, found := actual[key]; !found || actualValue != expectedValue {
			t.Errorf("got %q for key %q, wanted %q", actualValue, key, expectedValue)
		}
	}
}
