package scheduler_test

import (
	"errors"
	"slices"
	"sort"
	"strings"
	"testing"

	"github.com/Ensono/eirctl/runner"
	"github.com/Ensono/eirctl/scheduler"
	"github.com/Ensono/eirctl/task"
)

func TestExecutionGraph_AddStage(t *testing.T) {
	t.Parallel()
	g, err := scheduler.NewExecutionGraph("test")
	if err != nil {
		t.Fatal(err)
	}

	err = g.AddStage(scheduler.NewStage("stage1", func(s *scheduler.Stage) {
		s.DependsOn = []string{"stage2"}
	}))
	if err != nil {
		t.Fatal()
	}
	err = g.AddStage(scheduler.NewStage("stage2", func(s *scheduler.Stage) {
		s.DependsOn = []string{"stage1"}
	}))
	if err == nil {
		t.Fatal("add stage cycle detection failed\n")
	}
	if err != nil && !errors.Is(err, scheduler.ErrCycleDetected) {
		t.Fatalf("incorrect error (%q), wanted: %q\n", err, scheduler.ErrCycleDetected)
	}
}

func TestExecutionGraph_AddStagesAtOnce(t *testing.T) {
	t.Parallel()

	s1 := scheduler.NewStage("stage1", func(s *scheduler.Stage) {
		s.DependsOn = []string{"stage2"}
	})

	s2 := scheduler.NewStage("stage2", func(s *scheduler.Stage) {
		s.DependsOn = []string{"stage1"}
	})

	_, err := scheduler.NewExecutionGraph("test", s1, s2)

	if err == nil {
		t.Fatal("add stage cycle detection failed\n")
	}
	if err != nil && !errors.Is(err, scheduler.ErrCycleDetected) {
		t.Fatalf("incorrect error (%q), wanted: %q\n", err, scheduler.ErrCycleDetected)
	}
}

func TestExecutionGraph_Nodes(t *testing.T) {
	t.Run("all nodes exist", func(t *testing.T) {
		graph, stages := newNodeTestGraph(t, "all-nodes")

		assertGraphNodesFound(t, graph, "stage1", "stage2", "stage3", "stage4")
		assertRootChildren(t, graph, stages)
	})

	t.Run("node not found error", func(t *testing.T) {
		graph, stages := newNodeTestGraph(t, "missing-node")

		_, err := graph.Node("stage7")
		if !errors.Is(err, scheduler.ErrNodeNotFound) {
			t.Fatalf("Node(stage7) error = %v, want %v", err, scheduler.ErrNodeNotFound)
		}
		assertRootChildren(t, graph, stages)
	})
}

func newNodeTestGraph(t *testing.T, name string) (*scheduler.ExecutionGraph, []*scheduler.Stage) {
	t.Helper()

	stages := []*scheduler.Stage{
		scheduler.NewStage("stage1"),
		scheduler.NewStage("stage2"),
		scheduler.NewStage("stage3"),
		scheduler.NewStage("stage4"),
	}
	graph, err := scheduler.NewExecutionGraph(name, stages...)
	if err != nil {
		t.Fatal(err)
	}
	return graph, stages
}

func assertGraphNodesFound(t *testing.T, graph *scheduler.ExecutionGraph, names ...string) {
	t.Helper()

	for _, name := range names {
		if _, err := graph.Node(name); err != nil {
			t.Fatalf("Node(%q) error = %v", name, err)
		}
	}
}

func assertRootChildren(t *testing.T, graph *scheduler.ExecutionGraph, stages []*scheduler.Stage) {
	t.Helper()

	children := graph.Children(scheduler.RootNodeName)
	if len(children) != len(stages) {
		t.Fatalf("root has %d children, want %d", len(children), len(stages))
	}
	for _, stage := range stages {
		if child, ok := children[stage.Name]; !ok || child != stage {
			t.Errorf("root children do not contain stage %q", stage.Name)
		}
	}
}

func TestExecutionGraph_TreeWalk_BFS(t *testing.T) {
	t.Parallel()
	stages := []*scheduler.Stage{
		scheduler.NewStage("stage1", func(s *scheduler.Stage) {
			s.Pipeline = &scheduler.ExecutionGraph{}
		}),
		scheduler.NewStage("stage2", func(s *scheduler.Stage) {
			s.DependsOn = []string{"stage3"}
		}),
		scheduler.NewStage("stage3", func(s *scheduler.Stage) {
			s.DependsOn = []string{"stage1"}
		}),
		scheduler.NewStage("stage4"),
	}
	g, err := scheduler.NewExecutionGraph("test_bfs", stages...)
	if err != nil {
		t.Fatal(err)
	}

	bfs := g.BFSNodesFlattened(scheduler.RootNodeName)
	// - stage1
	// - stage4
	// - stage3
	// - stage2
	if bfs[2].Name != "stage3" {
		t.Errorf("penultimate node (%q) should be stage3", bfs[2].Name)
	}

	if bfs[3].Name != "stage2" {
		t.Errorf("last node (%q) should be stage2", bfs[3].Name)
	}

	// stage1 or stage4 are run in parallel and the first node can be either
	if !slices.Contains([]string{"stage1", "stage4"}, bfs[0].Name) {
		t.Errorf("first node (%q) should be either stage1 or stage4", bfs[0].Name)
	}
}

func TestExecutionGraph_BFS_Sorted(t *testing.T) {
	t.Parallel()
	stages := []*scheduler.Stage{
		scheduler.NewStage("stage1", func(s *scheduler.Stage) {
			s.Pipeline = &scheduler.ExecutionGraph{}
		}),
		scheduler.NewStage("stage2", func(s *scheduler.Stage) {
			s.DependsOn = []string{"stage3", "stage1"}
		}),
		scheduler.NewStage("stage3", func(s *scheduler.Stage) {
			s.DependsOn = []string{"stage1"}
		}),
		scheduler.NewStage("stage4"),
		scheduler.NewStage("stage5"),
	}

	g, err := scheduler.NewExecutionGraph("test_bfs", stages...)
	if err != nil {
		t.Fatal(err)
	}

	bfs := g.BFSNodesFlattened(scheduler.RootNodeName)
	sort.Sort(bfs)
	// after sorting it needs to look like this - always
	// - stage1
	// - stage4
	// - stage5
	// - stage3
	// - stage2
	for idx, v := range []string{"stage1", "stage4", "stage5", "stage3", "stage2"} {
		if bfs[idx].Name != v {
			t.Errorf("last node (%q) should be %s", bfs[idx].Name, v)
		}
	}

}

func TestExecutionGraph_Error(t *testing.T) {
	t.Parallel()
	s1, _ := scheduler.NewExecutionGraph("stage1")
	stages := []*scheduler.Stage{
		scheduler.NewStage("stage1", func(s *scheduler.Stage) {
			s1.AddStage(scheduler.NewStage("t:one", func(s *scheduler.Stage) {
				task := task.NewTask("t:one")
				task.Commands = []string{"true"}
				s.Task = task
			}))
			s.Pipeline = s1
		}),
		scheduler.NewStage("stage2", func(s *scheduler.Stage) {
			task := task.NewTask("s2:t2")
			task.Commands = []string{"false"}
			// task.AllowFailure = false
			s.Task = task
			s.DependsOn = []string{"stage3", "stage1"}
		}),
		scheduler.NewStage("stage3", func(s *scheduler.Stage) {
			task := task.NewTask("stage3")
			task.Commands = []string{"true"}
			s.Task = task
			s.DependsOn = []string{"stage1"}
		}),
		scheduler.NewStage("stage4", func(s *scheduler.Stage) {
			task := task.NewTask("s4:t1")
			task.Commands = []string{"false"}
			// task.AllowFailure = false
			s.Task = task
		}),
		scheduler.NewStage("stage5", func(s *scheduler.Stage) {
			task := task.NewTask("stage5")
			task.Commands = []string{"true"}
			s.Task = task
		}),
	}
	g, err := scheduler.NewExecutionGraph("test_bfs", stages...)
	if err != nil {
		t.Fatal(err)
	}
	tr, _ := runner.NewTaskRunner()
	s := scheduler.NewScheduler(tr)

	if err := s.Schedule(g); err == nil {
		t.Error("graph failed to error")
	}

	if !strings.Contains(g.Error().Error(), "stage2") {
		t.Errorf("incorrect error logged, got %s, wanted to include stage2\n", g.Error().Error())
	}
}
