package cmd

import (
	"fmt"
	"text/template"

	"github.com/Ensono/eirctl/internal/utils"
	"github.com/Ensono/eirctl/runner"
	"github.com/coryb/templatecolor"
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
	DefinedIn string
}

var showContextTmpl = `Context: {{ .Name | bold -}}
{{- if .Container }}
Image: {{ .Container.Image }}
{{- if .Volumes }}
Volumes: 
{{- range .Volumes }}
  - {{ . | bg.Blue }}
{{- end }}
{{- end }}
{{- end }}
Quote: {{ .Quote }}
{{- /* 
This is only the current env plus added to the context

DOES NOT INCLUDE computed envFile include/excludes and paths

*/}}
DefinedIn: {{ .DefinedIn | fg.Magenta }}
Env: 
{{- range $key, $val := .Env }}
  {{ $key | bg.Yellow }} => '{{ $val}}'
{{- end }}
Variables: 
{{- range $key, $val := .Vars }}
{{ $key }} => '{{ $val}}'
{{- end }}
Envfile: {{ .Envfile }}
`

type pipelineShow struct {
	Name      string
	Generator map[string]any
	Env       map[string]string
	Envfile   *utils.Envfile
}

var showPipelineTmpl = `Pipeline: {{ .Name | bold }}
Env:
{{- range $key, $val := .Env }}
{{ $key }} => '{{ $val}}'
{{- end }}
CI_Metadata:
{{- range $key, $val := .Generator }}
{{ $key }} => '{{ $val}}'
{{- end }}
{{- if .Envfile }}
Envfile: {{ .Envfile }}
{{- end }}
---
NOTE: to see the nodes of this pipeline run: 
---
eirctl graph {{ .Name }}
`

var showTaskTmpl = `Task: {{ .Name | bold }}
Description: {{ .Description -}}
{{- if .Context }}
Context: {{ .Context }}
{{- end }}
Commands: 
{{- range .Commands }}
- {{ . | bg.Green }}
{{- end -}}
{{ if .Dir }}
Dir: {{ .Dir }}
{{- end }}
{{- if .Timeout }}
Timeout: {{ .Timeout }}
{{- end}}
AllowFailure: {{ .AllowFailure }}
{{- if .Required }}
{{- if .Required.Env }}
Required:
  Env:
  {{- range .Required.Env }}
  - {{ . | bg.Red }}
  {{- end }}
{{- end }}
{{- end }}
DefinedIn: {{ .SourceFile | fg.Magenta }}
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
				tmpl := template.Must(template.New("show-task").Funcs(templatecolor.FuncMap()).Parse(showTaskTmpl))
				return tmpl.Execute(rootCmd.ChannelOut, taskDef)
			}
			if pipelineDef != nil {
				sp := pipelineShow{Name: pipelineDef.Name(),
					Env:       pipelineDef.Env,
					Envfile:   pipelineDef.EnvFile,
					Generator: pipelineDef.Generator,
				}
				tmpl := template.Must(template.New("show-pipeline").Funcs(templatecolor.FuncMap()).Parse(showPipelineTmpl))
				return tmpl.Execute(rootCmd.ChannelOut, sp)
			}
			if contextDef != nil {
				sc := showContext{Name: args[0], Volumes: []string{},
					Quote: contextDef.Quote, Env: contextDef.Env.Map(),
					Vars:      contextDef.Variables.Map(),
					DefinedIn: contextDef.SourceFile,
				}
				cc := contextDef.Container()
				if cc != nil {
					sc.Container = cc
					for vol := range cc.Volumes() {
						sc.Volumes = append(sc.Volumes, vol)
					}
				}
				tmpl := template.Must(template.New("show-context").Funcs(templatecolor.FuncMap()).Parse(showContextTmpl))
				return tmpl.Execute(rootCmd.ChannelOut, sc)
			}
			return fmt.Errorf("%s. %w", args[0], ErrIncorrectPipelineTaskArg)
		},
	}
	rootCmd.Cmd.AddCommand(showCmd)
}
