package config

import (
	"time"

	"github.com/Ensono/eirctl/internal/schema"
	"github.com/Ensono/eirctl/internal/utils"
	"github.com/Ensono/eirctl/task"
	"github.com/invopop/jsonschema"
)

//go:generate go run ../../tools/schemagenerator/main.go -dir ../../

// ConfigDefinition holds the top most config definition
// this can be parsed from a yaml, json, toml files/mediaTypes
//
// Note: works the same way for local/remote config resources
// correct content-type or file extension must be specified
// to successfully decode/unmarshal into type
type ConfigDefinition struct {
	// Import is a list of additional resources to bring into the main config
	// these can be remote or local resources
	Import []string `mapstructure:"import" yaml:"import" json:"import,omitempty" jsonschema:"anyOf_required=import"`
	// Contexts is a map of contexts to use
	// for specific tasks
	Contexts map[string]*ContextDefinition `mapstructure:"contexts" yaml:"contexts" json:"contexts,omitempty" jsonschema:"anyOf_required=contexts"`
	// Pipelines are a set of tasks wrapped in additional run conditions
	// e.g. depends on or allow failure
	Pipelines map[string][]*PipelineDefinition `mapstructure:"pipelines" yaml:"pipelines" json:"pipelines,omitempty" jsonschema:"anyOf_required=pipelines"`
	// Tasks are the most basic building blocks of eirctl
	Tasks    map[string]*TaskDefinition    `mapstructure:"tasks" yaml:"tasks" json:"tasks,omitempty" jsonschema:"anyOf_required=tasks"`
	Watchers map[string]*WatcherDefinition `mapstructure:"watchers" yaml:"watchers" json:"watchers,omitempty"`
	Debug    bool                          `json:"debug,omitempty"`
	DryRun   bool                          `json:"dry_run,omitempty"`
	Summary  bool                          `json:"summary,omitempty"`
	Verbose  bool                          `json:"verbose,omitempty"`
	// Output sets globally the output type for all tasks and pipelines
	//
	Output string `mapstructure:"output" yaml:"output" json:"output,omitempty" jsonschema:"enum=raw,enum=cockpit,enum=prefixed"`
	// Variables are the top most variables and will be merged and overwritten with lower level
	// specifications.
	// e.g. variable of Version=123
	// will be overwritten by a variables specified in this order, lowest takes the highest precedence
	// - context
	// - pipeline
	// - task
	// - commandline.
	// Variables can be used inside templating using the text/template go package
	Variables EnvVarMapType `mapstructure:"variables" yaml:"variables" json:"variables,omitempty"`
	// Generator defines the options for the desired CI yaml generation
	// Currently these are just map[string]any so that the user can specify the desired behaviour
	// NOTE: will provide no build time safety
	Generator *Generator `mapstructure:"ci_meta,omitempty" yaml:"ci_meta,omitempty" json:"ci_meta,omitempty"`
}

type ContextDefinition struct {
	Dir    string   `mapstructure:"dir" yaml:"dir" json:"dir,omitempty"`
	Up     []string `mapstructure:"up" yaml:"up" json:"up,omitempty"`
	Down   []string `mapstructure:"down" yaml:"down" json:"down,omitempty"`
	Before []string `mapstructure:"before" yaml:"before" json:"before,omitempty"`
	After  []string `mapstructure:"after" yaml:"after" json:"after,omitempty"`
	// Env is supplied from config file definition and is merged with the
	// current process environment variables list
	//
	// User supplied env map will overwrite any keys inside the process env
	// TODO: check this is desired behaviour
	Env EnvVarMapType `mapstructure:"env" yaml:"env" json:"env,omitempty"` // jsonschema:"anyof_type=string;integer;number;bool"
	// Envfile is a special block for use in executables that support file mapping
	// e.g. podman or docker
	//
	// Note: Envfile in the container context will ignore the generate flag
	// it will however respect all the other directives of include/exclude and modify operations
	Envfile *utils.Envfile `mapstructure:"envfile" yaml:"envfile,omitempty" json:"envfile,omitempty"`
	// Variables
	Variables EnvVarMapType `mapstructure:"variables" yaml:"variables" json:"variables,omitempty"`
	// Executable block holds the exec info
	Executable *utils.Binary `mapstructure:"executable" yaml:"executable" json:"executable,omitempty"`
	// Quote is the quote char to use when parsing commands into executables like docker
	Quote string `mapstructure:"quote" yaml:"quote" json:"quote,omitempty"`
	// Container is the specific context for containers
	// only available to docker API compliant implementations
	//
	// e.g. docker and podman
	//
	// The aim is to remove some of the boilerplate away from the existing more
	// generic context and introduce a specific context for tasks run in containers.
	//
	// Example:
	//
	// ```yaml
	// container:
	//   image:
	//     name: alpine
	//     # entrypoint: ""
	//     # shell: sh # default sh
	//     # args: nil
	// ```
	Container *utils.Container `mapstructure:"container" yaml:"container,omitempty" json:"container,omitempty"`
}

type PipelineDefinition struct {
	// Name is the friendly name to give to pipeline
	Name string `mapstructure:"name" yaml:"name" json:"name,omitempty"`
	// Condition evaluates whether to run this task or not within a given Schedule (Pipeline).
	// This always runs in the default Executor and does not inherit any pipeline or task variables and environment.
	//
	// The condition must be a valid shell syntax and returns a non-zero (0) exit code for the task to be skipped inside a pipeline.
	Condition string `mapstructure:"condition" yaml:"condition" json:"condition,omitempty"`
	// Task is the pointer to the task to run
	// it has to match the key in tasks map
	Task string `mapstructure:"task" yaml:"task,omitempty" json:"task,omitempty" jsonschema:"oneof_required=task"`
	// Pipeline is the name of the pipeline to run
	// Task and Pipeline are mutually exclusive
	// if both are specified task will win
	Pipeline string `mapstructure:"pipeline" yaml:"pipeline,omitempty" json:"pipeline,omitempty" jsonschema:"oneof_required=pipeline"`
	// DependsOn
	DependsOn schema.StringSlice `mapstructure:"depends_on" yaml:"depends_on,omitempty" json:"depends_on,omitempty" jsonschema:"oneof_type=string;array"`
	// AllowFailure
	AllowFailure bool `mapstructure:"allow_failure" yaml:"allow_failure,omitempty" json:"allow_failure,omitempty"`
	// Dir is the place where to run the task(s) in.
	// If empty - currentDir is used
	Dir string `mapstructure:"dir" yaml:"dir,omitempty" json:"dir,omitempty"`
	// Env is the Key: Value map of env vars to inject into the tasks within this pipeline
	Env EnvVarMapType `mapstructure:"env" yaml:"env,omitempty" json:"env,omitempty"`
	// Variables is the Key: Value map of vars vars to inject into the tasks
	Variables EnvVarMapType `mapstructure:"variables" yaml:"variables,omitempty" json:"variables,omitempty"`
	// Generator PipelineLevel
	Generator map[string]any `mapstructure:"ci_meta,omitempty" yaml:"ci_meta,omitempty" json:"ci_meta,omitempty"`
}

type Generator struct {
	// Version sets the version of eirctl to use for generation of default tasks, defaults to the current version of the binary
	Version       string         `mapstructure:"version" yaml:"version,omitempty" json:"version,omitempty"`
	TargetOptions map[string]any `mapstructure:"targetOpts" yaml:"targetOpts,omitempty" json:"targetOpts,omitempty"`
}

type TaskDefinition struct {
	Name        string `mapstructure:"name" yaml:"name,omitempty" json:"name,omitempty"`
	Description string `mapstructure:"description" yaml:"description,omitempty" json:"description,omitempty"`
	Condition   string `mapstructure:"condition" yaml:"condition,omitempty" json:"condition,omitempty"`
	// Command is the actual command to run in either a specified executable or
	// in mvdn.shell
	Command schema.StringSlice `mapstructure:"command" yaml:"command" json:"command" jsonschema:"oneof_type=string;array"`
	After   []string           `mapstructure:"after" yaml:"after,omitempty" json:"after,omitempty"`
	Before  []string           `mapstructure:"before" yaml:"before,omitempty" json:"before,omitempty"`
	// Context is the pointer to the key in the context map
	// it must exist else it will fallback to default context
	Context string `mapstructure:"context" yaml:"context,omitempty" json:"context,omitempty"`
	// Variations is per execution env var mutator
	// the number of variations in the list defines the number of times the command will be run
	// if using the default executor, see `ResetContext` if you need
	Variations []map[string]string `mapstructure:"variations" yaml:"variations,omitempty" json:"variations,omitempty"`
	// Dir to run the command from
	// If empty defaults to current directory
	Dir          string         `mapstructure:"dir" yaml:"dir,omitempty" json:"dir,omitempty"`
	Timeout      *time.Duration `mapstructure:"timeout" yaml:"timeout,omitempty" json:"timeout,omitempty"`
	AllowFailure bool           `mapstructure:"allow_failure" yaml:"allow_failure,omitempty" json:"allow_failure,omitempty"`
	Interactive  bool           `mapstructure:"interactive" yaml:"interactive,omitempty" json:"interactive,omitempty"`
	Artifacts    *task.Artifact `mapstructure:"artifacts" yaml:"artifacts,omitempty" json:"artifacts,omitempty"`
	// Env key=value map that will overwrite everything set at a higher level
	Env EnvVarMapType `mapstructure:"env" yaml:"env,omitempty" json:"env,omitempty"`
	// EnvFile is an object points to a path of an env file.
	//
	// This file is read dynamically each time before the task execution.
	// Any keys specified in the `env` key=value map will overwrite those set in the env file.
	//
	// The precedence of environment setting can be summarized as below
	// Context < Pipeline < Task
	//
	// This means that whilst all env vars will be merged downwards to the task
	// i.e. key specified at a task level will overwrite those set at any level above.
	//
	// Setting a key in an envfile with the same name will be overwritten by an `env.key`.
	Envfile *utils.Envfile `mapstructure:"envfile" yaml:"envfile,omitempty" json:"envfile,omitempty"`
	// Variables merged with others if any already priovided
	// These will overwrite any previously set keys
	Variables EnvVarMapType `mapstructure:"variables" yaml:"variables,omitempty" json:"variables,omitempty"`
	// ResetContext ensures each invocation of the variation is run with a Reset on the executor.
	// Currently only applies to a default executor and when run in variations.
	ResetContext bool `mapstructure:"reset_context" yaml:"reset_context,omitempty" json:"reset_context,omitempty" jsonschema:"default=false"`
	// Generator is the CI meta properties that will only be used during a generate process
	Generator map[string]any `mapstructure:"ci_meta,omitempty" yaml:"ci_meta,omitempty" json:"ci_meta,omitempty"`
	// Required specifies any required inputs into the task
	Required *task.RequiredInput `yaml:"required,omitempty" json:"required,omitempty"`
}

type WatcherDefinition struct {
	Events    []string          `mapstructure:"events" yaml:"events" json:"events"`
	Watch     []string          `mapstructure:"watch" yaml:"watch" json:"watch"`
	Exclude   []string          `mapstructure:"exclude" yaml:"exclude,omitempty" json:"exclude,omitempty"`
	Task      string            `mapstructure:"task" yaml:"task" json:"task"`
	Variables map[string]string `mapstructure:"variables" yaml:"variables,omitempty" json:"variables,omitempty"`
}

// EnvVarMapType is the custom reflection type for schema generation
type EnvVarMapType map[string]string

func (EnvVarMapType) JSONSchema() *jsonschema.Schema {
	return &jsonschema.Schema{
		Type: "object",
		AdditionalProperties: &jsonschema.Schema{
			AnyOf: []*jsonschema.Schema{
				{Type: "string"},
				{Type: "number"},
				{Type: "boolean"},
			},
		},
	}
}
