package cmd

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/Ensono/eirctl/internal/config"
	outputpkg "github.com/Ensono/eirctl/output"
	"github.com/Ensono/eirctl/runner"
	"github.com/Ensono/eirctl/variables"
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var (
	Version  = "0.0.0"
	Revision = "aaaa1234"
)

type rootCmdFlags struct {
	// all vars here
	Debug       bool
	Verbose     bool
	CfgFilePath string
	Output      string
	Raw         bool
	Cockpit     bool
	Quiet       bool
	VariableSet map[string]string
	DryRun      bool
	// NoSummary report at the end of the pipeline or task run
	NoSummary bool
}

type OsFSOpsIface interface {
	Rename(oldpath string, newpath string) error
	WriteFile(name string, data []byte, perm os.FileMode) error
	Create(name string) (io.Writer, error)
}

type EirCtlCmd struct {
	ctx        context.Context
	Cmd        *cobra.Command
	ChannelOut io.Writer
	ChannelErr io.Writer
	OsFsOps    OsFSOpsIface
	viperConf  *viper.Viper
	rootFlags  *rootCmdFlags
}

type osFsOps struct {
}

func (o osFsOps) Rename(oldpath string, newpath string) error {
	return os.Rename(oldpath, newpath)
}

func (o osFsOps) WriteFile(name string, data []byte, perm os.FileMode) error {
	return os.WriteFile(name, data, perm)
}

func (o osFsOps) Create(name string) (io.Writer, error) {
	return os.Create(name)
}

func NewEirCtlCmd(ctx context.Context, channelOut, channelErr io.Writer) *EirCtlCmd {
	tc := &EirCtlCmd{
		ctx:        ctx,
		ChannelOut: channelOut,
		ChannelErr: channelErr,
		OsFsOps:    osFsOps{},
		Cmd: &cobra.Command{
			Use:                        "eirctl",
			Version:                    fmt.Sprintf("%s-%s", Version, Revision),
			Args:                       cobra.ExactArgs(0),
			Short:                      "Modern task runner with native support for containerised tasks",
			Long:                       `eirctl allows for task composition and running them in eirctl pipelines (graph)`,
			SuggestionsMinimumDistance: 1,
			SilenceErrors:              true, // handle errors in the main
		},
	}

	tc.rootFlags = &rootCmdFlags{}
	tc.viperConf = viper.NewWithOptions()
	tc.viperConf.SetEnvPrefix("EIRCTL")
	tc.viperConf.SetConfigName("eirctl_conf")

	tc.Cmd.PersistentFlags().StringVarP(&tc.rootFlags.CfgFilePath, "config", "c", "eirctl.yaml", "config file to use - `eirctl.yaml`. For backwards compatibility it also accepts taskctl.yaml and tasks.yaml")
	_ = tc.viperConf.BindEnv("config", "EIRCTL_CONFIG_FILE")
	_ = tc.viperConf.BindPFlag("config", tc.Cmd.PersistentFlags().Lookup("config"))

	// flag toggles
	tc.Cmd.PersistentFlags().BoolVarP(&tc.rootFlags.Debug, "debug", "d", false, "enable debug level logging")
	_ = tc.viperConf.BindPFlag("debug", tc.Cmd.PersistentFlags().Lookup("debug")) // EIRCTL_DEBUG

	tc.Cmd.PersistentFlags().BoolVarP(&tc.rootFlags.Verbose, "verbose", "", false, "enable trace level logging")
	_ = tc.viperConf.BindPFlag("verbose", tc.Cmd.PersistentFlags().Lookup("verbose")) // EIRCTL_VERBOSE

	tc.Cmd.PersistentFlags().BoolVarP(&tc.rootFlags.DryRun, "dry-run", "", false, "dry run")
	_ = tc.viperConf.BindPFlag("dry-run", tc.Cmd.PersistentFlags().Lookup("dry-run"))

	tc.Cmd.PersistentFlags().BoolVarP(&tc.rootFlags.NoSummary, "no-summary", "", false, "show summary")
	_ = tc.viperConf.BindPFlag("no-summary", tc.Cmd.PersistentFlags().Lookup("no-summary"))

	tc.Cmd.PersistentFlags().BoolVarP(&tc.rootFlags.Quiet, "quiet", "q", false, "quite mode")
	_ = tc.viperConf.BindPFlag("quiet", tc.Cmd.PersistentFlags().Lookup("quiet"))

	return tc
}

// WithSubCommands returns a manually maintained list of commands
func WithSubCommands() []func(rootCmd *EirCtlCmd) {
	// add all sub commands
	return []func(rootCmd *EirCtlCmd){
		newRunCmd,
		newGraphCmd,
		newShowCmd,
		newListCmd,
		newInitCmd,
		newValidateCmd,
		newWatchCmd,
		newGenerateCmd,
		newShellCmd,
		newUpdateCommand,
	}
}

// InitCommand ensures each subcommand is added to the root using an IoC injection pattern
func (tc *EirCtlCmd) InitCommand(iocFuncs ...func(rootCmd *EirCtlCmd)) error {
	for _, fn := range iocFuncs {
		fn(tc)
	}
	return nil
}

func (tc *EirCtlCmd) Execute() error {
	// NOTE: do we need logrus ???
	// latest Go has structured logging...
	logrus.SetFormatter(&logrus.TextFormatter{
		DisableColors:   false,
		TimestampFormat: "2006-01-02 15:04:05",
		FullTimestamp:   false,
	})
	logrus.SetOutput(tc.ChannelErr)

	return tc.Cmd.ExecuteContext(tc.ctx)
}

var (
	ErrIncompleteConfig = errors.New("config key is missing")
)

// initConfig constructs the config on execute
func (tc *EirCtlCmd) initConfig() (*config.Config, error) {
	// consume env and build options via Viper
	tc.viperConf.AutomaticEnv()
	configFilePath, err := configFileFinder(tc)
	if err != nil {
		return nil, err
	}

	cl := config.NewConfigLoader(config.NewConfig())
	conf, err := cl.Load(configFilePath)
	if err != nil {
		return nil, err
	}

	// if cmd line flags were passed in, they override anything
	// parsed from theconfig file
	if tc.viperConf.GetBool("debug") {
		conf.Debug = tc.viperConf.GetBool("debug") // this is bound to viper env flag
	}

	if conf.Debug {
		logrus.SetLevel(logrus.DebugLevel)
	}

	conf.Verbose = tc.viperConf.GetBool("verbose")

	if conf.Verbose {
		logrus.SetLevel(logrus.TraceLevel)
	}

	// if default config keys were set to false
	// we check the overrides
	if tc.rootFlags.Quiet {
		conf.Quiet = tc.rootFlags.Quiet
	}
	if tc.rootFlags.DryRun {
		conf.DryRun = tc.rootFlags.DryRun
	}

	// default set up of summary to true
	conf.Summary = true

	if tc.rootFlags.NoSummary {
		// This is to maintain the old behaviour of exposing a flag with a default state in `true`
		conf.Summary = !tc.rootFlags.NoSummary
	}

	// if output not specified in yaml/CLI/EnvVar
	// conf.Output comes from the yaml - if not set in yaml
	if tc.viperConf.GetString("output") != "" {
		// then compute the cmd line --output flag
		// can be set via env or as a flag
		conf.Output = outputpkg.OutputEnum(tc.viperConf.GetString("output"))
	}

	// if cmdline flags for output shorthand has been provided we
	// overwrite the output in config
	if tc.viperConf.GetBool("raw") {
		conf.Output = outputpkg.RawOutput
	}

	if tc.viperConf.GetBool("cockpit") {
		conf.Output = outputpkg.CockpitOutput
	}

	// if no value was set - we set to default
	if conf.Output == "" {
		conf.Output = outputpkg.RawOutput
	}

	// these are CLI args only
	conf.Options.GraphOrientationLeftRight = tc.viperConf.GetBool("lr")
	conf.Options.InitDir = tc.viperConf.GetString("dir")
	conf.Options.InitNoPrompt = tc.viperConf.GetBool("no-prompt")
	return conf, nil
}

// buildTaskRunner initiates the task runner struct
//
// assigns to the global var to the package
// args are the stdin inputs of strings following the `--` parameter
func (tc *EirCtlCmd) buildTaskRunner(args []string, conf *config.Config) (*runner.TaskRunner, *argsToStringsMapper, error) {
	argsStringer, err := argsValidator(args, conf)
	if err != nil {
		return nil, nil, err
	}

	vars := variables.FromMap(tc.viperConf.GetStringMapString("set"))
	// These are stdin args passed over `-- arg1 arg2`
	vars.Set("ArgsList", argsStringer.argsList)
	vars.Set("Args", strings.Join(argsStringer.argsList, " "))
	tr, err := tc.initTaskRunner(conf, vars)
	if err != nil {
		return nil, nil, err
	}
	tr.OutputFormat = string(conf.Output)
	tr.DryRun = conf.DryRun

	if conf.Quiet {
		tr.Stdout = io.Discard
		tr.Stderr = io.Discard
	}

	go func() {
		<-tc.ctx.Done()
		tr.Cancel()
	}()

	return tr, argsStringer, nil
}

func (tc *EirCtlCmd) initTaskRunner(conf *config.Config, vars *variables.Variables) (*runner.TaskRunner, error) {
	return runner.NewTaskRunner(runner.WithContexts(conf.Contexts),
		runner.WithVariables(vars),
		func(runner *runner.TaskRunner) {
			runner.Stdout = tc.ChannelOut
			runner.Stderr = tc.ChannelErr
			runner.Stdin = tc.Cmd.InOrStdin()
		},
		runner.WithGracefulCtx(tc.ctx))
}

// configFileFinder loops through the possible options
func configFileFinder(tc *EirCtlCmd) (string, error) {
	configFilePath := tc.viperConf.GetString("config")
	// creates a slice of all possible options for the file name
	// taskctl|tasks.yaml are a backwards compatible options
	configOpts := []string{configFilePath, "tasks.yaml", "taskctl.yaml"}
	found, confFile := false, ""
	for _, co := range configOpts {
		if _, err := os.Stat(co); err != nil {
			logrus.Debugf("%s config file not present", co)
			continue
		}
		confFile = co
		found = true
		// exit loop on first occurence of file found
		logrus.Debugf("config file found: %s", co)
		break
	}
	if found && confFile != "" {
		return confFile, nil
	}
	return "", fmt.Errorf("%w\nincorrect config file (%s) does not exist", ErrIncompleteConfig, configFilePath)
}
