package runner

import (
	"io"
	"time"

	"github.com/Ensono/eirctl/internal/utils"
	"github.com/Ensono/eirctl/variables"
)

// Job is a linked list of jobs to execute by Executor
// Task can have 1 or more Jobs
type Job struct {
	Command string
	Dir     string
	Env     *variables.Variables
	EnvFile *utils.Envfile
	Vars    *variables.Variables
	Timeout *time.Duration

	Stdout, Stderr io.Writer
	Stdin          io.Reader
	IsShell        bool
	Next           *Job
}

// NewJobFromCommand creates new Job instance from given command
func NewJobFromCommand(command string) *Job {
	return &Job{
		Command: command,
		Vars:    variables.NewVariables(),
		Env:     variables.NewVariables(),
	}
}
