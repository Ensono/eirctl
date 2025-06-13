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

func Test_HandleOutputCapture(t *testing.T) {
	ttests := map[string]struct {
		b       []byte
		wantKey string
		wantVal string
	}{
		"clean input": {[]byte(`export TASK_OUTPUT_FOO=bar`), "FOO", "bar"},
		"multiline input": {[]byte(`export BAR=Bassdd
export TASK_OUTPUT_FOO="bar"
`), "FOO", "bar"},
		"single quotes": {[]byte(`export TASK_OUTPUT_FOO='bar'`), "FOO", "bar"},
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
