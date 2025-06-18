// package Cmdutils provides testable helpers to commands only
package cmdutils

import (
	"fmt"
	"io"
	"slices"
	"strings"

	"github.com/Ensono/eirctl/internal/config"
	"github.com/Ensono/eirctl/internal/utils"
	"github.com/Ensono/eirctl/scheduler"
	"github.com/charmbracelet/bubbles/list"
)

const (
	MAGENTA_TERMINAL string = "\x1b[35m%s\x1b[0m"
	GREEN_TERMINAL   string = "\x1b[32m%s\x1b[0m"
	CYAN_TERMINAL    string = "\x1b[36m%s\x1b[0m"
	RED_TERMINAL     string = "\x1b[31m%s\x1b[0m"
	GREY_TERMINAL    string = "\x1b[18m%s\x1b[0m"
	BOLD_TERMINAL    string = "\x1b[1m%s"
)

func DisplayTaskSelection(conf *config.Config, showPipelineOnly bool) (string, error) {
	initItems := []list.Item{}

	pipelines := utils.MapKeys(conf.Pipelines)
	slices.Sort(pipelines)

	for _, pipeline := range pipelines {
		p := conf.Pipelines[pipeline]

		stages := []string{}
		for _, v := range p.BFSNodesFlattened(scheduler.RootNodeName) {
			stages = append(stages, v.Name)
		}

		initItems = append(initItems, item{title: pipeline, description: fmt.Sprintf("Stages: %s", strings.Join(stages, ","))})
	}

	if !showPipelineOnly {
		tasks := utils.MapKeys(conf.Tasks)
		slices.Sort(tasks)
		for _, tsk := range tasks {
			task := conf.Tasks[tsk]
			desc := task.Description
			if desc == "" {
				desc = "No description"
			}
			initItems = append(initItems, item{title: task.Name, description: "Task: " + desc})

		}
	}

	return TuiRun(newModel(initItems))
}

// printSummary is a TUI helper
func PrintSummary(g *scheduler.ExecutionGraph, chanOut io.Writer, detailedSummary bool) {
	stages := g.BFSNodesFlattened(scheduler.RootNodeName)

	fmt.Fprintf(chanOut, BOLD_TERMINAL, "Summary: \n")

	slices.SortFunc(stages, func(i, j *scheduler.Stage) int {
		if i.Start().After(j.Start()) {
			return 1
		}
		if j.Start().After(i.Start()) {
			return -1
		}
		return 0
	})

	for _, stage := range stages {
		stage.Name = stageNameHelper(g.Name(), stage.Name)
		switch stage.ReadStatus() {
		case scheduler.StatusDone:
			fmt.Fprintf(chanOut, GREEN_TERMINAL, fmt.Sprintf("- Stage %s was completed in %s\n", stage.Name, stage.Duration()))
		case scheduler.StatusSkipped:
			fmt.Fprintf(chanOut, CYAN_TERMINAL, fmt.Sprintf("- Stage %s was skipped\n", stage.Name))
		case scheduler.StatusError:
			log := ""
			if stage.Task != nil {
				log = strings.TrimSpace(stage.Task.ErrorMessage())
			}
			if stage.Pipeline != nil && stage.Pipeline.Error() != nil && log == "" {
				log = stage.Pipeline.Error().Error()
			}
			fmt.Fprintf(chanOut, RED_TERMINAL, fmt.Sprintf("- Stage %s failed in %s\n", stage.Name, stage.Duration()))
			if log != "" {
				fmt.Fprintf(chanOut, RED_TERMINAL, fmt.Sprintf("  > %s\n", log))
			}
		case scheduler.StatusCanceled:
			fmt.Fprintf(chanOut, GREY_TERMINAL, fmt.Sprintf("- Stage %s was cancelled\n", stage.Name))
		default:
			fmt.Fprintf(chanOut, RED_TERMINAL, fmt.Sprintf("- Unexpected status %d for stage %s\n", stage.ReadStatus(), stage.Name))
		}
	}

	fmt.Fprintf(chanOut, "%s: %s\n", fmt.Sprintf(BOLD_TERMINAL, "Total duration"), fmt.Sprintf(GREEN_TERMINAL, g.Duration()))
}

// stageNameHelper strips out the root pipeline name
func stageNameHelper(prefix, stage string) string {
	return strings.Replace(stage, prefix+"->", "", 1)
}
