package scheduler_test

import (
	"errors"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/Ensono/eirctl/internal/utils"
	"github.com/Ensono/eirctl/scheduler"
	"github.com/Ensono/eirctl/variables"

	"github.com/Ensono/eirctl/runner"
	"github.com/Ensono/eirctl/task"
)

type mockTaskRunner struct {
	run func(t *task.Task) error
}

func (t2 mockTaskRunner) Run(t *task.Task) error {
	return t2.run(t)
}

func (t2 mockTaskRunner) Cancel() {}

func (t2 mockTaskRunner) Finish() {}

func TestExecutionGraph_Scheduler(t *testing.T) {
	stage1 := scheduler.NewStage("stage1", func(s *scheduler.Stage) {
		s.Task = task.FromCommands("t1", "/usr/bin/true")
	})
	stage2 := scheduler.NewStage("stage2", func(s *scheduler.Stage) {
		s.Task = task.FromCommands("t2", "/usr/bin/false")
		s.DependsOn = []string{"stage1"}
	})

	stage3 := scheduler.NewStage("stage3", func(s *scheduler.Stage) {
		s.Task = task.FromCommands("t2", "/usr/bin/false")
		s.DependsOn = []string{"stage2"}
	})

	stage4 := scheduler.NewStage("stage4", func(s *scheduler.Stage) {
		s.Task = task.FromCommands("t3", "true")
		s.DependsOn = []string{"stage3"}

	})

	graph, err := scheduler.NewExecutionGraph("g1", stage1, stage2, stage3, stage4)
	if err != nil || graph.Error() != nil {
		t.Fatal(err)
	}

	taskRunner := mockTaskRunner{
		run: func(t *task.Task) error {
			if t.Commands[0] == "/usr/bin/false" {
				t.WithExitCode(1)
				t.WithError(fmt.Errorf("error"))
				return errors.New("task failed")
			}
			return nil
		},
	}

	schdlr := scheduler.NewScheduler(taskRunner)
	// Should error on stage3
	err = schdlr.Schedule(graph)
	if err == nil {
		t.Fatalf("error not captured, got %q, wanted an error", err)
	}

	if graph.Duration() <= 0 {
		t.Fatal()
	}
	// Should cancel after stage3
	if stage3.ReadStatus() != scheduler.StatusCanceled || stage4.ReadStatus() != scheduler.StatusCanceled {
		t.Fatal("stage3 was not cancelled")
	}
}

func TestExecutionGraph_Scheduler_AllowFailure(t *testing.T) {
	stage1 := scheduler.NewStage("stage1", func(s *scheduler.Stage) {
		s.Task = task.FromCommands("t1", "/usr/bin/true")
	})
	stage2 := scheduler.NewStage("stage2", func(s *scheduler.Stage) {
		s.Task = task.FromCommands("t2", "/usr/bin/false")
		s.AllowFailure = true
		s.DependsOn = []string{"stage1"}
	})
	// TODO: It doesn't matter really but the command is never replaced with
	// the variable value here..?
	stage3 := scheduler.NewStage("stage3", func(s *scheduler.Stage) {
		s.Task = task.FromCommands("t3", "{{.command}}")
		s.DependsOn = []string{"stage2"}
		s.WithVariables(variables.FromMap(map[string]string{"command": "true"}))
	})

	graph, err := scheduler.NewExecutionGraph("t1", stage1, stage2, stage3)
	if err != nil {
		t.Fatal(err)
	}

	taskRunner := mockTaskRunner{
		run: func(t *task.Task) error {
			// The test can return with a time of zero if the processor is powerful
			// add a small delay to ensure a duration is always recorded
			time.Sleep(50 * time.Nanosecond)
			if t.Commands[0] == "/usr/bin/false" {
				t.WithExitCode(1)
				t.WithError(fmt.Errorf("error"))
				return errors.New("task failed")
			}
			return nil
		},
	}

	schdlr := scheduler.NewScheduler(taskRunner)
	defer schdlr.Finish()

	err = schdlr.Schedule(graph)
	if err == nil {
		t.Fatal("expected error")
	}

	if stage2.Task.ExitCode() != 1 {
		t.Fatal("stage 2 should exit with an error code")
	}

	if stage2.ReadStatus() != scheduler.StatusDone {
		t.Fatal("stage 2 wasn't marked as done. It should allow its failure")
	}

	if stage3.ReadStatus() == scheduler.StatusCanceled {
		t.Fatal("stage3 was cancelled")
	}

	if stage3.Duration() <= 0 {
		t.Error()
	}
}

func TestSkippedStage(t *testing.T) {
	stage1 := scheduler.NewStage("stage1", func(s *scheduler.Stage) {
		s.Task = task.FromCommands("t1", "true")
		s.Condition = "true"

	})
	stage2 := scheduler.NewStage("stage2", func(s *scheduler.Stage) {
		s.Task = task.FromCommands("t2", "false")
		s.AllowFailure = true
		s.DependsOn = []string{"stage1"}
		s.Condition = "false"
	})

	graph, err := scheduler.NewExecutionGraph("t1", stage1, stage2)
	if err != nil {
		t.Fatal(err)
	}

	taskRunner := mockTaskRunner{
		run: func(t *task.Task) error {
			if t.Commands[0] == "/usr/bin/false" {
				t.WithExitCode(1)
				t.WithError(fmt.Errorf("error"))
				return errors.New("task failed")
			}
			return nil
		},
	}

	schdlr := scheduler.NewScheduler(taskRunner)
	err = schdlr.Schedule(graph)
	if err != nil {
		t.Fatal(err)
	}

	if stage1.ReadStatus() != scheduler.StatusDone || stage2.ReadStatus() != scheduler.StatusSkipped {
		t.Error()
	}
}

func TestScheduler_Cancel(t *testing.T) {
	stage1 := scheduler.NewStage("stage1", func(s *scheduler.Stage) {
		s.Task = task.FromCommands("t1", "sleep 60")
	})

	graph, err := scheduler.NewExecutionGraph("t1", stage1)
	if err != nil {
		t.Fatal(err)
	}

	taskRunner := mockTaskRunner{
		run: func(t *task.Task) error {
			if t.Commands[0] == "/usr/bin/false" {
				t.WithExitCode(1)
				t.WithError(fmt.Errorf("error"))
				return errors.New("task failed")
			}
			return nil
		},
	}

	schdlr := scheduler.NewScheduler(taskRunner)
	go func() {
		schdlr.Cancel()
	}()

	err = schdlr.Schedule(graph)
	if err != nil {
		t.Fatal(err)
	}

	if schdlr.Cancelled() != 1 {
		t.Error()
	}
}

func Test_Scheduler_ConditionErroredStage(t *testing.T) {
	stage1 := scheduler.NewStage("stage1", func(s *scheduler.Stage) {
		s.Task = task.FromCommands("t1", "true")
		s.Condition = "true"
	})

	stage2 := scheduler.NewStage("stage2", func(s *scheduler.Stage) {
		s.Task = task.FromCommands("t2", "false")
		s.AllowFailure = true
		s.DependsOn = []string{"stage1"}
		s.Condition = "wrong"
	})

	graph, err := scheduler.NewExecutionGraph("t1", stage1, stage2)
	if err != nil {
		t.Fatal(err)
	}

	taskRunner := mockTaskRunner{
		run: func(t *task.Task) error {
			if t.Commands[0] == "/usr/bin/false" {
				t.WithExitCode(1)
				t.WithError(fmt.Errorf("error"))
				return errors.New("task failed")
			}
			return nil
		},
	}

	schdlr := scheduler.NewScheduler(taskRunner)
	err = schdlr.Schedule(graph)
	if err != nil {
		t.Fatal(err)
	}

	if stage1.ReadStatus() != scheduler.StatusDone {
		t.Errorf("stage 1 incorrectly finished, got %v wanted Done", stage1.ReadStatus())
	}
	// This is now kind of pointless
	if stage2.ReadStatus() != scheduler.StatusSkipped {
		t.Errorf("stage 2 incorrectly finished, got %v wanted Done", stage2.ReadStatus())
	}
}

func Test_Scheduler_Error_Required(t *testing.T) {
	stage1 := scheduler.NewStage("stage1", func(s *scheduler.Stage) {
		s.Task = task.FromCommands("t1", "true")
		s.Condition = "true"
	})

	stage2 := scheduler.NewStage("stage2", func(s *scheduler.Stage) {
		s.Task = task.FromCommands("t2", "false")
		s.AllowFailure = true
		s.DependsOn = []string{"stage1"}
		s.Condition = "wrong"
	})

	graph, err := scheduler.NewExecutionGraph("t1", stage1, stage2)
	if err != nil {
		t.Fatal(err)
	}

	taskRunner := mockTaskRunner{
		run: func(t *task.Task) error {
			if t.Commands[0] == "/usr/bin/false" {
				t.WithExitCode(1)
				t.WithError(fmt.Errorf("error"))
				return errors.New("task failed")
			}
			return nil
		},
	}

	schdlr := scheduler.NewScheduler(taskRunner)
	err = schdlr.Schedule(graph)
	if err != nil {
		t.Fatal(err)
	}

	if stage1.ReadStatus() != scheduler.StatusDone {
		t.Errorf("stage 1 incorrectly finished, got %v wanted Done", stage1.ReadStatus())
	}
	// This is now kind of pointless
	if stage2.ReadStatus() != scheduler.StatusSkipped {
		t.Errorf("stage 2 incorrectly finished, got %v wanted Done", stage2.ReadStatus())
	}
}

func Test_Scheduler_EnvFile_path_precedence_after_denormalization(t *testing.T) {

	taskEnvFile, err := os.CreateTemp("", "task-*.env")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(taskEnvFile.Name())

	stageEnvFile, err := os.CreateTemp("", "stage-*.env")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(stageEnvFile.Name())

	taskEnvFile.Write([]byte(`FOO=should_overwrite_context_from_task
BAR=123
LUX=foobar`))

	stageEnvFile.Write([]byte(`FOO=should_overwrite_task_from_stage
BAR_STAGE=453
LUX_STAGE=baz`))

	t1 := task.FromCommands("t1", "/usr/bin/true")
	t1.EnvFile = utils.NewEnvFile(func(e *utils.Envfile) {
		e.PathValue = []string{taskEnvFile.Name()}
	})
	stage1 := scheduler.NewStage("stage1", func(s *scheduler.Stage) {
		s.Task = t1
		s.WithEnvFile(utils.NewEnvFile(func(e *utils.Envfile) {
			e.PathValue = []string{stageEnvFile.Name()}
		}))
	})

	t2 := task.FromCommands("t2", "/usr/bin/false")
	t2.EnvFile = utils.NewEnvFile(func(e *utils.Envfile) {
		e.PathValue = []string{taskEnvFile.Name()}
	})

	stage2 := scheduler.NewStage("stage2", func(s *scheduler.Stage) {
		s.Task = t2
	})
	g2, _ := scheduler.NewExecutionGraph("g2", stage2)
	stage3 := scheduler.NewStage("stage3", func(s *scheduler.Stage) {
		s.Pipeline = g2
		s.DependsOn = []string{"stage1"}
		s.WithEnvFile(utils.NewEnvFile(func(e *utils.Envfile) {
			e.PathValue = []string{stageEnvFile.Name()}
		}))
	})

	stage4 := scheduler.NewStage("stage4", func(s *scheduler.Stage) {
		s.Task = task.FromCommands("t3", "true")
		s.DependsOn = []string{"stage3"}
	})

	graph, err := scheduler.NewExecutionGraph("g1", stage1, stage2, stage3, stage4)
	if err != nil || graph.Error() != nil {
		t.Fatal(err)
	}

	taskRunner := mockTaskRunner{
		run: func(receivedTask *task.Task) error {
			if receivedTask.Name == "g1->t1" {
				if len(receivedTask.EnvFile.Path()) != 2 {
					t.Errorf("incorrectly merged env file paths, got %v, wanted 2", receivedTask.EnvFile)
				}
			}
			if receivedTask.Name == "g1->stage3->t2" {
				if len(receivedTask.EnvFile.Path()) != 2 {
					t.Errorf("incorrectly merged env file paths when called from a nested pipeline into a task\ngot %v, wanted 2", receivedTask.EnvFile.Path())
				}
				wantOrder := map[int]string{0: taskEnvFile.Name(), 1: stageEnvFile.Name()}
				// check order is correct
				for idx, got := range receivedTask.EnvFile.Path() {
					if got != wantOrder[idx] {
						t.Errorf("wrong order got: %s wanted: %s", got, wantOrder[idx])
					}
				}
			}
			return nil
		},
	}

	ng, err := graph.Denormalize()
	if err != nil {
		t.Fatal(err)
	}

	schdlr := scheduler.NewScheduler(taskRunner)

	if err := schdlr.Schedule(ng); err != nil {
		t.Fatal(err)
	}
}

func ExampleScheduler_Schedule() {
	format := task.FromCommands("t1", "go fmt ./...")
	build := task.FromCommands("t2", "go build ./..")
	r, _ := runner.NewTaskRunner()
	s := scheduler.NewScheduler(r)

	graph, err := scheduler.NewExecutionGraph("t1",
		scheduler.NewStage("format", func(s *scheduler.Stage) {
			s.Task = format
		}),
		scheduler.NewStage("build", func(s *scheduler.Stage) {
			s.Task = build
			s.DependsOn = []string{"format"}
		}),
	)
	if err != nil {
		return
	}

	err = s.Schedule(graph)
	if err != nil {
		fmt.Println(err)
	}
}
