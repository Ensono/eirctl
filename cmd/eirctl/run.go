package cmd

import (
	"context"
	"errors"
	"fmt"
	"io"

	"github.com/Ensono/eirctl/internal/cmdutils"
	"github.com/Ensono/eirctl/internal/config"
	"github.com/Ensono/eirctl/runner"
	"github.com/Ensono/eirctl/scheduler"
	"github.com/Ensono/eirctl/task"
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

var (
	ErrSpecifiedObjectIsNotFound = errors.New("unknown")
)

type runFlags struct {
	showGraphOnly, detailedSummary bool
}

type runCmd struct {
	channelOut, channelErr io.Writer
	flags                  *runFlags
	conf                   *config.Config
	ctx                    context.Context
}

func newRunCmd(rootCmd *EirCtlCmd) {
	f := &runFlags{}
	runner := &runCmd{
		channelOut: rootCmd.ChannelOut,
		channelErr: rootCmd.ChannelErr,
		flags:      f,
		ctx:        rootCmd.ctx,
	}

	rc := &cobra.Command{
		Use:     "run",
		Aliases: []string{},
		Short:   `runs <pipeline or task>`,
		Example: `eirctl run pipeline1
		eirctl run task1`,
		Args:         cobra.MinimumNArgs(0),
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			conf, err := rootCmd.initConfig()
			if err != nil {
				return err
			}
			// display selector if nothing is supplied
			if len(args) == 0 {
				selected, err := cmdutils.DisplayTaskSelection(rootCmd.ctx, conf, false)
				if err != nil {
					return err
				}
				if selected == "" {
					logrus.Debug("no selection made, exiting...")
					return nil
				}
				args = append([]string{selected}, args[0:]...)
			}

			taskRunner, argsStringer, err := rootCmd.buildTaskRunner(args, conf)
			if err != nil {
				return err
			}

			return runner.runTarget(taskRunner, conf, argsStringer)
		},
	}

	rc.AddCommand(&cobra.Command{
		Use:          "pipeline",
		Short:        `runs pipeline <task>`,
		Example:      `eirctl run pipeline pipeline:name`,
		Args:         cobra.MinimumNArgs(1),
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			conf, err := rootCmd.initConfig()
			if err != nil {
				return err
			}
			taskRunner, argsStringer, err := rootCmd.buildTaskRunner(args, conf)
			if err != nil {
				return err
			}
			if argsStringer.pipelineName == nil {
				return fmt.Errorf("pipeline: %s is %w", args[0], ErrSpecifiedObjectIsNotFound)
			}
			return runner.runPipeline(argsStringer.pipelineName, taskRunner, conf.Summary)
		},
	})

	rc.AddCommand(&cobra.Command{
		Use:          "task",
		Aliases:      []string{},
		Short:        `runs task <task>`,
		Example:      `eirctl run task1`,
		SilenceUsage: true,
		Args:         cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			conf, err := rootCmd.initConfig()
			if err != nil {
				return err
			}
			runner.conf = conf
			taskRunner, argsStringer, err := rootCmd.buildTaskRunner(args, conf)
			if err != nil {
				return err
			}
			if argsStringer.taskName == nil {
				return fmt.Errorf("task: %s is %w", args[0], ErrSpecifiedObjectIsNotFound)
			}
			return runner.runTask(argsStringer.taskName, taskRunner)
		},
	})

	rc.PersistentFlags().StringVarP(&rootCmd.rootFlags.Output, "output", "o", "", "output format (raw, prefixed or cockpit)")
	_ = rootCmd.viperConf.BindEnv("output", "EIRCTL_OUTPUT_FORMAT")
	_ = rootCmd.viperConf.BindPFlag("output", rc.PersistentFlags().Lookup("output"))

	// Shortcut flags
	rc.PersistentFlags().BoolVarP(&rootCmd.rootFlags.Raw, "raw", "r", false, "shortcut for --output=raw")
	_ = rootCmd.viperConf.BindPFlag("raw", rc.PersistentFlags().Lookup("raw"))

	rc.PersistentFlags().BoolVarP(&rootCmd.rootFlags.Cockpit, "cockpit", "", false, "shortcut for --output=cockpit")
	_ = rootCmd.viperConf.BindPFlag("cockpit", rc.PersistentFlags().Lookup("cockpit"))

	// Key=Value pairs
	// can be supplied numerous times
	rc.PersistentFlags().StringToStringVarP(&rootCmd.rootFlags.VariableSet, "set", "", nil, "set global variable value")
	_ = rootCmd.viperConf.BindPFlag("set", rc.PersistentFlags().Lookup("set"))

	rc.PersistentFlags().BoolVarP(&f.showGraphOnly, "graph-only", "", false, "Show only the denormalized graph")
	rc.PersistentFlags().BoolVarP(&f.detailedSummary, "detailed", "", false, "Show detailed summary, otherwise will be summarised by top level stages only")

	rootCmd.Cmd.AddCommand(rc)
}

func (r *runCmd) runTarget(taskRunner *runner.TaskRunner, conf *config.Config, argsStringer *argsToStringsMapper) (err error) {

	if argsStringer.pipelineName != nil {
		return r.runPipeline(argsStringer.pipelineName, taskRunner, conf.Summary)
	}

	if argsStringer.taskName != nil {
		if err := r.runTask(argsStringer.taskName, taskRunner); err != nil {
			return fmt.Errorf("task `%s` failed: %w", argsStringer.taskOrPipelineName, err)
		}
	}

	return nil
}

func (r *runCmd) runPipeline(g *scheduler.ExecutionGraph, taskRunner *runner.TaskRunner, summary bool) error {
	sd := scheduler.NewScheduler(taskRunner)
	defer sd.Finish()

	// rebuild the tree with deduped nested graphs
	// when running embedded pipelines in pipelines referencing
	// creating a new graph ensures no race occurs as
	// go routine stages all point to a different address space
	ng, err := g.Denormalize()
	if err != nil {
		return err
	}

	if r.flags.showGraphOnly {
		return graphCmdRun(ng, r.channelOut, &graphFlags{})
	}

	if err := sd.Schedule(ng); err != nil {
		logrus.Debugf("scheduler error: %v", err)
	}

	fmt.Fprint(r.channelOut, "\r\n")

	if summary {
		cmdutils.PrintSummary(ng, r.channelOut, r.flags.detailedSummary)
	}
	return ng.Error()
}

func (r *runCmd) runTask(t *task.Task, taskRunner *runner.TaskRunner) error {
	err := taskRunner.Run(t)
	if err != nil {
		return err
	}

	taskRunner.Finish()

	return nil
}

var ErrIncorrectPipelineTaskArg = errors.New("supplied argument does not match any pipelines or tasks")

type argsToStringsMapper struct {
	taskOrPipelineName string
	pipelineName       *scheduler.ExecutionGraph
	taskName           *task.Task
	argsList           []string
}

// argsValidator assigns the task or pipeline name to run
// Will have errored already if the args length is 0
//
// the first arg should be the name of the task or pipeline
func argsValidator(args []string, conf *config.Config) (*argsToStringsMapper, error) {
	argsStringer := &argsToStringsMapper{}

	if conf.Pipelines[args[0]] != nil {
		argsStringer.pipelineName = conf.Pipelines[args[0]]
	}
	if conf.Tasks[args[0]] != nil {
		argsStringer.taskName = conf.Tasks[args[0]]
	}

	if argsStringer.pipelineName == nil && argsStringer.taskName == nil && conf.Watchers[args[0]] == nil {
		return argsStringer, fmt.Errorf("%s does not exist, ensure your first argument is the name of the pipeline or task. %w", args[0], ErrIncorrectPipelineTaskArg)
	}

	argsStringer.argsList = args[1:]
	argsStringer.taskOrPipelineName = args[0]
	return argsStringer, nil
}
