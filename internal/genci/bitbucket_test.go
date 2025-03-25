package genci_test

import (
	"testing"

	"github.com/Ensono/eirctl/internal/config"
	"github.com/Ensono/eirctl/internal/genci"
	"github.com/Ensono/eirctl/scheduler"
	"github.com/Ensono/eirctl/task"
)

func Test_GenCi_Bitbucket(t *testing.T) {
	sp, _ := scheduler.NewExecutionGraph("foo",
		scheduler.NewStage("stage1", func(s *scheduler.Stage) {
			s.Pipeline, _ = scheduler.NewExecutionGraph("dev",
				scheduler.NewStage("sub-one", func(s *scheduler.Stage) {
					s.Task = task.NewTask("t2")
					s.Generator = map[string]any{"bitbucket": map[string]any{}}
				}),
				scheduler.NewStage("sub-two", func(s *scheduler.Stage) {
					s.Task = task.NewTask("t4")
					s.DependsOn = []string{"t2"}
				}),
				scheduler.NewStage("sub-three", func(s *scheduler.Stage) {
					s.Task = task.NewTask("t5")
					s.DependsOn = []string{"t2", "t4"}
				}),
			)
			s.Generator = map[string]any{"bitbucket": map[string]any{}}
		}),
		scheduler.NewStage("stage2", func(s *scheduler.Stage) {
			ts1 := task.NewTask("task:dostuff")
			ts1.Generator = map[string]any{"bitbucket": map[string]any{}}
			s.Task = ts1
			s.DependsOn = []string{"stage1"}
		}))

	_, err := genci.New(genci.BitbucketCITarget, &config.Config{
		Pipelines: map[string]*scheduler.ExecutionGraph{"foo": sp},
		Generate: &config.Generator{
			TargetOptions: map[string]any{},
		},
	})
	if err != nil {
		t.Errorf("failed to generate bitbucket, %v\n", err)
	}
}
