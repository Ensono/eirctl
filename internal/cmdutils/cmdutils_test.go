package cmdutils_test

import (
	"bytes"
	"context"
	"testing"
	"time"

	"github.com/Ensono/eirctl/internal/cmdutils"
	"github.com/Ensono/eirctl/internal/config"
	"github.com/Ensono/eirctl/scheduler"
	"github.com/Ensono/eirctl/task"
)

func Test_PrintSummary(t *testing.T) {
	t.Run("no stages run", func(t *testing.T) {
		out := bytes.Buffer{}
		cmdutils.PrintSummary(&scheduler.ExecutionGraph{}, &out, true)
		if len(out.Bytes()) == 0 {
			t.Fatal("got 0, wanted bytes written")
		}
	})

	t.Run("one stage run", func(t *testing.T) {
		out := bytes.Buffer{}
		graph, _ := scheduler.NewExecutionGraph("t1")
		stage := scheduler.NewStage("foo", func(s *scheduler.Stage) {
		})

		stage.UpdateStatus(scheduler.StatusDone)
		graph.AddStage(stage)
		cmdutils.PrintSummary(graph, &out, false)
		if len(out.Bytes()) == 0 {
			t.Fatal("got 0, wanted bytes written")
		}
	})
}

func Test_DisplayTaskSelection_cancelled(t *testing.T) {

	sut := config.NewConfig()
	graph, _ := scheduler.NewExecutionGraph("t1")
	stage := scheduler.NewStage("foo", func(s *scheduler.Stage) {
	})

	stage.UpdateStatus(scheduler.StatusDone)
	graph.AddStage(stage)

	sut.Pipelines["foo"] = graph
	sut.Tasks["bar"] = task.NewTask("qux")

	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		time.Sleep(1 * time.Millisecond)
		cancel()
	}()

	// the error needs to be unable to attach/open a TTY
	_, err := cmdutils.DisplayTaskSelection(ctx, sut, false)
	t.Log(err)
}
