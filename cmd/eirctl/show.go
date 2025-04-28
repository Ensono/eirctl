package cmd

import (
	"fmt"
	"html/template"

	"github.com/Ensono/eirctl/internal/utils"
	"github.com/Ensono/eirctl/runner"
	"github.com/spf13/cobra"
)

type showContext struct {
	Name      string
	Container *runner.ContainerContext
	Volumes   []string
	Quote     string
	Env       map[string]any
	Vars      map[string]any
	Envfile   *utils.Envfile
}

var showContextTmpl = `
Name: {{ .Name -}}
{{ if .Container }}
Image: {{ .Container.Image }}
Volumes: {{ .Volumes }}
{{- end }}
Quote: {{ .Quote }}
Env: {{ .Env }}
Variables: {{ .Vars }}
{{ if .Envfile }}
Envfile: {{ .Envfile }}
{{- end }}
`

type pipelineShow struct {
	Name      string
	Generator map[string]any
	Env       map[string]string
	Envfile   *utils.Envfile
}

var showPipelineTmpl = `
Name: {{ .Name }}
Env: {{ .Env }}
CI metadata: {{ .Generator }} 
{{ if .Envfile }}
Envfile: {{ .Envfile }}
{{- end }}

---
NOTE: to see the nodes of this pipeline run: 
---
eirctl graph {{ .Name }}
`

var showTaskTmpl = `
  Name: {{ .Name -}}
{{ if .Description }}
  Description: {{ .Description }}
{{- end }}
  Context: {{ .Context }}
  Commands: 
{{- range .Commands }}
    - {{ . -}}
{{ end -}}
{{ if .Dir }}
  Dir: {{ .Dir }}
{{- end }}
{{ if .Timeout }}
  Timeout: {{ .Timeout }}
{{- end}}
  AllowFailure: {{ .AllowFailure }}
`

func newShowCmd(rootCmd *EirCtlCmd) {

	showCmd := &cobra.Command{
		Use:     "show <task|pipeline|context>",
		Aliases: []string{},
		Short:   `Shows task, pipeline or context  details, useful for imported configs via remote files/URLs.`,
		Args:    cobra.RangeArgs(1, 1),
		RunE: func(cmd *cobra.Command, args []string) error {
			conf, err := rootCmd.initConfig()
			if err != nil {
				return err
			}

			taskDef, pipelineDef, contextDef := conf.Tasks[args[0]], conf.Pipelines[args[0]], conf.Contexts[args[0]]
			if taskDef != nil {
				tmpl := template.Must(template.New("show-task").Parse(showTaskTmpl))
				return tmpl.Execute(rootCmd.ChannelOut, taskDef)
			}
			if pipelineDef != nil {
				sp := pipelineShow{Name: pipelineDef.Name(),
					Env:       pipelineDef.Env,
					Envfile:   pipelineDef.EnvFile,
					Generator: pipelineDef.Generator}
				tmpl := template.Must(template.New("show-pipeline").Parse(showPipelineTmpl))
				return tmpl.Execute(rootCmd.ChannelOut, sp)
			}
			if contextDef != nil {
				sc := showContext{Name: args[0], Volumes: []string{},
					Quote: contextDef.Quote, Env: contextDef.Env.Map(),
					Vars: contextDef.Variables.Map()}
				cc := contextDef.Container()
				if cc != nil {
					sc.Container = cc
					for vol := range cc.Volumes() {
						sc.Volumes = append(sc.Volumes, vol)
					}
				}
				tmpl := template.Must(template.New("show-context").Parse(showContextTmpl))
				return tmpl.Execute(rootCmd.ChannelOut, sc)
			}
			return fmt.Errorf("%s. %w", args[0], ErrIncorrectPipelineTaskArg)
		},
	}
	rootCmd.Cmd.AddCommand(showCmd)
}
