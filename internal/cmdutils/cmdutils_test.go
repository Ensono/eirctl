package cmdutils_test

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/Ensono/eirctl/internal/cmdutils"
	"github.com/Ensono/eirctl/internal/config"
	"github.com/Ensono/eirctl/scheduler"
	"github.com/Ensono/eirctl/task"
)

func Test_PrintSummary(t *testing.T) {
	graph, stages := summaryTestGraph(t)
	out := bytes.Buffer{}

	cmdutils.PrintSummary(graph, &out, false)

	want := strings.Join([]string{
		fmt.Sprintf(cmdutils.BOLD_TERMINAL, "Summary: \n"),
		fmt.Sprintf(cmdutils.GREEN_TERMINAL, "- Stage done was completed in 2s\n"),
		fmt.Sprintf(cmdutils.CYAN_TERMINAL, "- Stage skipped was skipped\n"),
		fmt.Sprintf(cmdutils.RED_TERMINAL, "- Stage errored failed in 2s\n"),
		fmt.Sprintf(cmdutils.RED_TERMINAL, "  > task failed\n"),
		fmt.Sprintf(cmdutils.GREY_TERMINAL, "- Stage canceled was cancelled\n"),
		fmt.Sprintf(cmdutils.RED_TERMINAL, "- Unexpected status 99 for stage unexpected\n"),
		fmt.Sprintf("%s: %s\n", fmt.Sprintf(cmdutils.BOLD_TERMINAL, "Total duration"), fmt.Sprintf(cmdutils.GREEN_TERMINAL, graph.Duration())),
	}, "")
	if got := out.String(); got != want {
		t.Errorf("PrintSummary() output = %q, want %q", got, want)
	}

	for _, stage := range stages {
		if strings.Contains(stage.Name, "pipeline->") {
			t.Errorf("PrintSummary() did not preserve stripped name for stage %q", stage.Name)
		}
	}
}

func summaryTestGraph(t *testing.T) (*scheduler.ExecutionGraph, []*scheduler.Stage) {
	t.Helper()

	base := time.Date(2026, time.January, 1, 0, 0, 0, 0, time.UTC)
	stages := []*scheduler.Stage{
		summaryStage("pipeline->done", scheduler.StatusDone, base),
		summaryStage("pipeline->skipped", scheduler.StatusSkipped, base.Add(time.Second)),
		summaryStage("pipeline->errored", scheduler.StatusError, base.Add(2*time.Second)),
		summaryStage("pipeline->canceled", scheduler.StatusCanceled, base.Add(3*time.Second)),
		summaryStage("pipeline->unexpected", 99, base.Add(4*time.Second)),
	}
	stages[2].Task = task.NewTask("errored").WithError(errors.New(" task failed \n"))

	graph, err := scheduler.NewExecutionGraph("pipeline", stages...)
	if err != nil {
		t.Fatal(err)
	}
	return graph, stages
}

func summaryStage(name string, status int32, start time.Time) *scheduler.Stage {
	stage := scheduler.NewStage(name).WithStart(start).WithEnd(start.Add(2 * time.Second))
	stage.UpdateStatus(status)
	return stage
}

func Test_DisplayTaskSelection_BuildOptionsList(t *testing.T) {

	sut := config.NewConfig()
	graph, _ := scheduler.NewExecutionGraph("t1")
	stage := scheduler.NewStage("foo", func(s *scheduler.Stage) {
	})

	stage.UpdateStatus(scheduler.StatusDone)
	graph.AddStage(stage)

	sut.Pipelines["foo"] = graph
	sut.Tasks["bar"] = task.NewTask("qux")

	// the error needs to be unable to attach/open a TTY
	got := cmdutils.BuildOptionsList(context.TODO(), sut, false)
	if len(got) != 2 {
		t.Errorf("got %v, wanted 2 items in the list", got)
	}
}
