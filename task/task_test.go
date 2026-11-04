package task_test

import (
	"fmt"
	"testing"

	"github.com/Ensono/eirctl/task"
)

func TestTask(t *testing.T) {
	task := task.FromCommands("t1", "ls /tmp")
	task.WithEnv("TEST_ENV", "TEST_VAL")

	if task.Commands[0] != "ls /tmp" {
		t.Error("task creation failed")
	}

	if task.Env.Get("TEST_ENV") != "TEST_VAL" {
		t.Error("task's env creation failed")
	}

	if task.Duration().Seconds() <= 0 {
		t.Error()
	}
}

func TestTask_ErrorMessage(t *testing.T) {
	tsk := task.NewTask("abc")
	tsk.WithError(fmt.Errorf("true"))

	if tsk.ErrorMessage() != "true" {
		t.Error()
	}

	tsk = task.NewTask("errored")
	if tsk.ErrorMessage() != "" {
		t.Error()
	}

	tsk.WithError(fmt.Errorf("true"))
	if tsk.Error().Error() != "true" {
		t.Error()
	}
}

func TestNewTask_WithVariations(t *testing.T) {
	tsk := task.FromCommands("t1", "ls /tmp")

	if len(tsk.GetVariations()) != 1 {
		t.Error()
	}

	tsk.Variations = []map[string]string{{"GOOS": "linux"}, {"GOOS": "windows"}}
	if len(tsk.GetVariations()) != 2 {
		t.Error()
	}
}

func TestRequiredInput_Merge(t *testing.T) {
	t.Run("additive and de-duplicated", func(t *testing.T) {
		base := &task.RequiredInput{
			Env:  []string{"AWS_REGION", "COMMIT_SHA"},
			Vars: []string{"Version"},
		}
		got := base.Merge(&task.StageRequiredInput{
			Env:  []string{"COMMIT_SHA", "DOMAIN"},
			Vars: []string{"Tag"},
		})

		assertStrings(t, "env", got.Env, []string{"AWS_REGION", "COMMIT_SHA", "DOMAIN"})
		assertStrings(t, "vars", got.Vars, []string{"Version", "Tag"})
	})

	t.Run("nil other is a no-op", func(t *testing.T) {
		base := &task.RequiredInput{Env: []string{"AWS_REGION"}}
		got := base.Merge(nil)
		assertStrings(t, "env", got.Env, []string{"AWS_REGION"})
	})
}

func assertStrings(t *testing.T, name string, got, want []string) {
	t.Helper()
	if len(got) != len(want) {
		t.Fatalf("%s: got %v, wanted %v", name, got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("%s[%d]: got %s, wanted %s", name, i, got[i], want[i])
		}
	}
}

func Test_HandleOutputCapture(t *testing.T) {
	ttests := map[string]struct {
		b       []byte
		wantKey string
		wantVal string
	}{
		"clean input": {[]byte(`export EIRCTL_TASK_OUTPUT_FOO=bar`), "FOO", "bar"},
		"multiline input": {[]byte(`export BAR=Bassdd
export EIRCTL_TASK_OUTPUT_FOO="bar"
`), "FOO", "bar"},
		"single quotes": {[]byte(`export EIRCTL_TASK_OUTPUT_FOO='bar'`), "FOO", "bar"},
	}
	for name, tt := range ttests {
		t.Run(name, func(t *testing.T) {
			tsk := task.NewTask("")
			tsk.HandleOutputCapture(tt.b)
			got := tsk.OutputCaptured()
			val, ok := got[tt.wantKey]
			if !ok {
				t.Fatal("got nil, wanted key to be present in capture output")
			}
			if val != tt.wantVal {
				t.Errorf("got %s, wanted %s", val, tt.wantVal)
			}
		})
	}
}
