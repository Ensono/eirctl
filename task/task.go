package task

import (
	"bytes"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"dario.cat/mergo"
	"github.com/Ensono/eirctl/internal/utils"
	"github.com/Ensono/eirctl/variables"
	"github.com/sirupsen/logrus"
)

type ArtifactType string

const (
	FileArtifactType   ArtifactType = "file"
	DotEnvArtifactType ArtifactType = "dotenv"
	// RuntimeEnvArtifactType captures any exported/set variables inside a task execution
	// Stores them in task output which can later be used inside a pipeline
	// [Experimental]
	RuntimeEnvArtifactType ArtifactType = "env"
)

// Artifact holds the information about the artifact to produce
// for the specific task.
//
// NB: it is run at the end of the task so any after commands
// that mutate the output files/dotenv file will essentially
// overwrite anything set/outputted as part of the main command
type Artifact struct {
	// Name is the key under which the artifacts will be stored
	//
	// Currently this is unused
	Name string `mapstructure:"name" yaml:"name,omitempty" json:"name,omitempty"`
	// Path is the glob like pattern to the
	// source of the file(s) to store as an output
	Path string `mapstructure:"path" yaml:"path,omitempty" json:"path,omitempty"`
	// Type is the artifact type
	// valid values are `file`|`dotenv`
	Type ArtifactType `mapstructure:"type" yaml:"type" json:"type" jsonschema:"enum=dotenv,enum=file,enum=env,default=dotenv"`
}

type RequiredInput struct {
	// Vars is a list of required variables by the task
	// It is case sensitive
	// It checks both the default vars, supplied vars, and Environment variables
	Vars []string `yaml:"vars,omitempty" json:"vars,omitempty"`
	// Env will identify any missing environment variables
	// It checks complete env vars - merged from global > context > pipeline > task
	Env []string `yaml:"env,omitempty" json:"env,omitempty"`
	// Args checks any args supplied after `--`
	Args []string `yaml:"args,omitempty" json:"args,omitempty"`
}

func (ri *RequiredInput) HasRequired() bool {
	return (len(ri.Args) + len(ri.Vars) + len(ri.Env)) > 0
}

var ErrRequiredInputMissing = errors.New("missing required input")

// CheckRequired ensures all required environment are specified/present and not empty
//
// This is a runtime checkinput
func (ri *RequiredInput) Check(env *variables.Variables, vars *variables.Variables) error {
	notFound := []string{}
	for _, v := range ri.Env {
		if !env.Has(v) {
			notFound = append(notFound, v)
		}
	}
	if len(notFound) > 0 {
		return fmt.Errorf("%w, %v is missing from the required env variables (%v)", ErrRequiredInputMissing, notFound, ri.Env)
	}

	for _, v := range ri.Vars {
		if !vars.Has(v) {
			notFound = append(notFound, v)
		}
	}
	if len(notFound) > 0 {
		return fmt.Errorf("%w, %v is missing from the required variables (%v)", ErrRequiredInputMissing, notFound, ri.Vars)
	}
	return nil
}

// // CheckRequiredVarsArgs ensures all required vars and args are present
// //
// // This is a compile time check
//
//	func (ri *RequiredInput) CheckVars(vars *variables.Variables) error {
//		notFound := []string{}
//		for _, v := range ri.Vars {
//			if !vars.Has(v) {
//				notFound = append(notFound, v)
//			}
//		}
//		if len(notFound) > 0 {
//			return fmt.Errorf("%w, %v is missing from the required variables (%v)", ErrRequiredInputMissing, notFound, ri.Vars)
//		}
//		return nil
//	}

type outputCapture struct {
	mu     *sync.Mutex
	output map[string]string
}

// Task is a structure that describes task, its commands, environment, working directory etc.
// After task completes it provides task's execution status, exit code, stdout and stderr
type Task struct {
	Commands     []string // Commands to run
	Context      string
	Env          *variables.Variables
	EnvFile      *utils.Envfile
	Variables    *variables.Variables
	Variations   []map[string]string
	Dir          string
	Timeout      *time.Duration
	AllowFailure bool
	After        []string
	Before       []string
	Interactive  bool
	// ResetContext is useful if multiple variations are running in the same task
	ResetContext bool
	Condition    string
	Artifacts    *Artifact

	Name        string
	Description string
	Required    *RequiredInput
	// internal fields updated by a mutex
	// only used with the single instance of the task
	mu             sync.Mutex // guards the below private fields
	start          time.Time
	end            time.Time
	skipped        bool
	exitCode       int16
	errored        bool
	errorVal       error
	capturedOutput outputCapture
	Generator      map[string]any
	SourceFile     string
}

// NewTask creates new Task instance
func NewTask(name string) *Task {
	return &Task{
		Name:           name,
		Env:            variables.NewVariables(),
		EnvFile:        utils.NewEnvFile(),
		Variables:      variables.NewVariables(),
		Required:       &RequiredInput{},
		exitCode:       -1,
		errored:        false,
		mu:             sync.Mutex{},
		capturedOutput: outputCapture{mu: &sync.Mutex{}, output: map[string]string{}},
	}
}

func (t *Task) FromTask(task *Task) {
	if err := mergo.Merge(t, task); err != nil {
		logrus.Error("failed to dereference task")
	}
	// merge vars from preceeding higher contexts
	t.Env = t.Env.Merge(task.Env)
	t.EnvFile = task.EnvFile
	t.Variables = t.Variables.Merge(task.Variables)
}

func (t *Task) WithStart(start time.Time) *Task {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.start = start
	return t
}

func (t *Task) Start() time.Time {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.start
}

func (t *Task) WithEnd(end time.Time) *Task {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.end = end
	return t
}

func (t *Task) End() time.Time {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.end
}

func (t *Task) WithSkipped(val bool) *Task {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.skipped = val
	return t
}

func (t *Task) Skipped() bool {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.skipped
}

// exitCode int16
func (t *Task) WithExitCode(val int16) *Task {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.exitCode = val
	return t
}

func (t *Task) ExitCode() int16 {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.exitCode
}

// errored  bool
func (t *Task) WithError(val error) *Task {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.errored = true
	t.errorVal = val
	return t
}

func (t *Task) Errored() bool {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.errored
}

func (t *Task) Error() error {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.errorVal
}

// FromCommands creates task new Task instance with given commands
func FromCommands(name string, commands ...string) *Task {
	t := NewTask(name)
	t.Commands = commands
	return t
}

// Duration returns task's execution duration
func (t *Task) Duration() time.Duration {
	if t.End().IsZero() {
		return time.Since(t.Start())
	}

	return t.End().Sub(t.Start())
}

// ErrorMessage returns message of the error occurred during task execution
func (t *Task) ErrorMessage() string {
	if !t.Errored() {
		return ""
	}
	return t.Error().Error()
}

// WithEnv sets environment variable
func (t *Task) WithEnv(key, value string) *Task {
	t.Env = t.Env.With(key, value)
	return t
}

// GetVariations returns array of maps which are task's variations
// if no variations exist one is returned to create the default job
func (t *Task) GetVariations() []map[string]string {
	variations := make([]map[string]string, 1)
	if t.Variations != nil {
		variations = t.Variations
	}

	return variations
}

// Output returns task's stdout as a string
//
// This is left as a legacy method for now. will be removed in the stable 2.x versions
func (t *Task) Output() string {
	return ""
}

const prefix string = `EIRCTL_TASK_OUTPUT_`

// HandleOutputCapture
func (t *Task) HandleOutputCapture(b []byte) {
	segments := bytes.Fields(b)
	for _, segment := range segments {
		if bytes.HasPrefix(segment, []byte(prefix)) {
			str := string(segment)
			parts := strings.SplitN(str, "=", 2)
			if len(parts) == 2 {
				key, value := parts[0], parts[1]
				t.capturedOutput.mu.Lock()
				t.capturedOutput.output[strings.TrimPrefix(key, prefix)] = strings.Trim(value, `'"`)
				t.capturedOutput.mu.Unlock()
			}
		}
	}
}

func (t *Task) OutputCaptured() map[string]string {
	t.capturedOutput.mu.Lock()
	defer t.capturedOutput.mu.Unlock()
	return t.capturedOutput.output
}
