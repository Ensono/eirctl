package scheduler

import (
	"sync"
	"testing"

	"github.com/Ensono/eirctl/internal/utils"
	"github.com/Ensono/eirctl/runner"
	"github.com/Ensono/eirctl/task"
)

type envFileRecordingRunner struct {
	start chan struct{}
	mu    sync.Mutex
	seen  []*utils.Envfile
}

func (r *envFileRecordingRunner) Run(t *task.Task) error {
	<-r.start
	r.mu.Lock()
	r.seen = append(r.seen, t.EnvFile)
	r.mu.Unlock()
	t.EnvFile.PathValue = append(t.EnvFile.PathValue, "runner-mutated.env")
	return nil
}

func (r *envFileRecordingRunner) Cancel() {}
func (r *envFileRecordingRunner) Finish() {}

var _ runner.Runner = (*envFileRecordingRunner)(nil)

func TestRunStageClonesEnvFileForConcurrentStages(t *testing.T) {
	shared := utils.NewEnvFile(func(env *utils.Envfile) {
		env.PathValue = []string{"shared.env"}
	})
	first := NewStage("first", func(stage *Stage) {
		stage.Task = task.FromCommands("first", "true")
		stage.Task.EnvFile = shared
		stage.WithEnvFile(utils.NewEnvFile(func(env *utils.Envfile) {
			env.PathValue = []string{"first.env"}
		}))
	})
	second := NewStage("second", func(stage *Stage) {
		stage.Task = task.FromCommands("second", "true")
		stage.Task.EnvFile = shared
		stage.WithEnvFile(utils.NewEnvFile(func(env *utils.Envfile) {
			env.PathValue = []string{"second.env"}
		}))
	})

	runner := &envFileRecordingRunner{start: make(chan struct{})}
	scheduler := NewScheduler(runner)
	var wg sync.WaitGroup
	for _, stage := range []*Stage{first, second} {
		wg.Add(1)
		go func(stage *Stage) {
			defer wg.Done()
			if err := scheduler.runStage(stage); err != nil {
				t.Errorf("runStage(%s): %v", stage.Name, err)
			}
		}(stage)
	}
	close(runner.start)
	wg.Wait()

	if got := shared.PathValue; len(got) != 1 || got[0] != "shared.env" {
		t.Fatalf("shared EnvFile was mutated: %v", got)
	}
	runner.mu.Lock()
	defer runner.mu.Unlock()
	if len(runner.seen) != 2 {
		t.Fatalf("runner saw %d tasks, want 2", len(runner.seen))
	}
	if runner.seen[0] == runner.seen[1] || runner.seen[0] == shared || runner.seen[1] == shared {
		t.Fatal("concurrent stages shared an EnvFile")
	}
	for _, env := range runner.seen {
		if len(env.PathValue) != 3 || env.PathValue[0] != "shared.env" || env.PathValue[2] != "runner-mutated.env" {
			t.Fatalf("unexpected isolated EnvFile paths: %v", env.PathValue)
		}
	}
}
