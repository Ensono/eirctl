package config

import (
	"runtime"

	"dario.cat/mergo"
	"github.com/Ensono/eirctl/internal/utils"
	"github.com/Ensono/eirctl/variables"

	"github.com/Ensono/eirctl/task"
)

func buildTask(def *TaskDefinition, lc *loaderContext) (*task.Task, error) {

	t := task.NewTask(def.Name)

	t.Description = def.Description
	t.Condition = def.Condition
	t.Commands = def.Command
	t.Variations = def.Variations
	t.Timeout = def.Timeout
	t.AllowFailure = def.AllowFailure
	t.After = def.After
	t.Before = def.Before
	t.Artifacts = def.Artifacts
	t.Context = def.Context
	t.Interactive = def.Interactive
	t.ResetContext = def.ResetContext
	t.Required = def.Required

	t.Env = variables.FromMap(def.Env).Merge(t.Env)
	ef := utils.NewEnvFile()

	if def.Envfile != nil {
		_ = mergo.Merge(ef, def.Envfile)
	}
	t.EnvFile = ef

	t.Variables = variables.FromVarsMap(def.Variables).Merge(t.Variables)

	t.Dir = def.Dir
	if def.Dir == "" {
		t.Dir = lc.Dir
	}

	t.SourceFile = def.SourceFile

	// Generator CI YAML
	t.Generator = def.Generator

	setDefaultVariables(t)

	return t, nil
}

func setDefaultVariables(t *task.Task) {
	defaultVarsMap := map[string]any{
		"Context": map[string]any{
			"Name": t.Context,
		},
		"Task": map[string]any{
			"Name": t.Name,
		},
		"Current": map[string]any{
			"OS":   runtime.GOOS,
			"Arch": runtime.GOARCH,
		},
	}
	t.Variables = t.Variables.Merge(variables.FromVarsMap(defaultVarsMap))
}
