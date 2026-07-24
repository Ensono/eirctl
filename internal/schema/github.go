package schema

import (
	"errors"
	"fmt"
	"sync"

	"gopkg.in/yaml.v3"
)

// Workflow represents the root of a GitHub workflow file.
type GithubWorkflow struct {
	Name        string               `json:"name,omitempty" yaml:"name,omitempty"`
	On          *GithubTriggerEvents `json:"on" yaml:"on"`
	Jobs        OrderedMap           `json:"jobs" yaml:"jobs"`
	Defaults    *GithubDefaults      `json:"defaults,omitempty" yaml:"defaults,omitempty"`
	Env         map[string]any       `json:"env,omitempty" yaml:"env,omitempty"`
	Permissions map[string]string    `json:"permissions,omitempty" yaml:"permissions,omitempty"`
}

// TriggerEvents represents the trigger events for the GitHub workflow.
type GithubTriggerEvents struct {
	Push               GithubPushEvent               `json:"push" yaml:"push,omitempty"`
	PullRequest        GithubPullRequestEvent        `json:"pull_request" yaml:"pull_request,omitempty"`
	PullRequestTarget  GithubPullRequestEvent        `json:"pull_request_target" yaml:"pull_request_target,omitempty"`
	IssueComment       GithubIssueCommentEvent       `json:"issue_comment" yaml:"issue_comment,omitempty"`
	RepositoryDispatch GithubRepositoryDispatchEvent `json:"repository_dispatch" yaml:"repository_dispatch,omitempty"`
	Schedule           []GithubScheduleEvent         `json:"schedule,omitempty" yaml:"schedule,omitempty"`
	WorkflowDispatch   GithubWorkflowDispatchEvent   `json:"workflow_dispatch" yaml:"workflow_dispatch,omitempty"`
	WorkflowRun        GithubWorkflowRunEvent        `json:"workflow_run" yaml:"workflow_run,omitempty"`

	configured map[string]struct{}
}

// Has reports whether an event was explicitly configured in the workflow. It
// distinguishes an absent event from an event with an empty or null mapping.
func (events *GithubTriggerEvents) Has(name string) bool {
	if events == nil {
		return false
	}
	_, ok := events.configured[name]
	return ok
}

// UnmarshalYAML accepts every trigger form supported by GitHub Actions: a
// scalar event, a sequence of events, or a mapping containing event options.
func (events *GithubTriggerEvents) UnmarshalYAML(node *yaml.Node) error {
	*events = GithubTriggerEvents{configured: map[string]struct{}{}}
	switch node.Kind {
	case yaml.ScalarNode:
		return events.configure(node.Value, nil)
	case yaml.SequenceNode:
		for _, value := range node.Content {
			if value.Kind != yaml.ScalarNode {
				return fmt.Errorf("github trigger: expected an event name but got %v", value.Kind)
			}
			if err := events.configure(value.Value, nil); err != nil {
				return err
			}
		}
		return nil
	case yaml.MappingNode:
		for i := 0; i+1 < len(node.Content); i += 2 {
			if err := events.configure(node.Content[i].Value, node.Content[i+1]); err != nil {
				return fmt.Errorf("github trigger %s: %w", node.Content[i].Value, err)
			}
		}
		return nil
	default:
		return fmt.Errorf("github trigger: expected a scalar, sequence, or mapping but got %v", node.Kind)
	}
}

func (events *GithubTriggerEvents) configure(name string, value *yaml.Node) error {
	events.configured[name] = struct{}{}
	if value == nil || value.Tag == "!!null" {
		return nil
	}
	switch name {
	case "push":
		return value.Decode(&events.Push)
	case "pull_request":
		return value.Decode(&events.PullRequest)
	case "pull_request_target":
		return value.Decode(&events.PullRequestTarget)
	case "issue_comment":
		return value.Decode(&events.IssueComment)
	case "repository_dispatch":
		return value.Decode(&events.RepositoryDispatch)
	case "schedule":
		return value.Decode(&events.Schedule)
	case "workflow_dispatch":
		return value.Decode(&events.WorkflowDispatch)
	case "workflow_run":
		return value.Decode(&events.WorkflowRun)
	default:
		return nil
	}
}

// PushEvent represents a push event trigger configuration.
type GithubPushEvent struct {
	Branches       []string `json:"branches,omitempty" yaml:"branches,omitempty"`
	Tags           []string `json:"tags,omitempty" yaml:"tags,omitempty"`
	Paths          []string `json:"paths,omitempty" yaml:"paths,omitempty"`
	PathsIgnore    []string `json:"paths-ignore,omitempty" yaml:"paths_ignore,omitempty"`
	BranchesIgnore []string `json:"branches-ignore,omitempty" yaml:"branches_ignore,omitempty"`
	TagsIgnore     []string `json:"tags-ignore,omitempty" yaml:"tags_ignore,omitempty"`
}

// PullRequestEvent represents a pull request event trigger configuration.
type GithubPullRequestEvent struct {
	Branches       []string `json:"branches,omitempty" yaml:"branches,omitempty"`
	Paths          []string `json:"paths,omitempty" yaml:"paths,omitempty"`
	BranchesIgnore []string `json:"branches-ignore,omitempty" yaml:"branches_ignore,omitempty"`
	PathsIgnore    []string `json:"paths-ignore,omitempty" yaml:"paths_ignore,omitempty"`
	Types          []string `json:"types,omitempty" yaml:"types,omitempty"`
}

// ScheduleEvent represents a cron schedule event trigger configuration.
type GithubScheduleEvent struct {
	Cron string `json:"cron,omitempty" yaml:"cron,omitempty"`
}

// WorkflowDispatchEvent represents a manually triggered workflow dispatch event.
type GithubWorkflowDispatchEvent struct {
	Inputs map[string]GithubInput `json:"inputs,omitempty" yaml:"inputs,omitempty"`
}

// IssueCommentEvent represents issue-comment trigger filtering.
type GithubIssueCommentEvent struct {
	Types []string `json:"types,omitempty" yaml:"types,omitempty"`
}

// RepositoryDispatchEvent represents repository-dispatch trigger filtering.
type GithubRepositoryDispatchEvent struct {
	Types []string `json:"types,omitempty" yaml:"types,omitempty"`
}

// WorkflowRunEvent represents workflow-run trigger filtering.
type GithubWorkflowRunEvent struct {
	Workflows GithubStringList `json:"workflows,omitempty" yaml:"workflows,omitempty"`
	Types     []string         `json:"types,omitempty" yaml:"types,omitempty"`
	Branches  []string         `json:"branches,omitempty" yaml:"branches,omitempty"`
}

// GithubStringList normalizes GitHub Actions fields that accept either a
// scalar string or a sequence of strings.
type GithubStringList []string

// UnmarshalYAML accepts scalar and sequence forms.
func (values *GithubStringList) UnmarshalYAML(node *yaml.Node) error {
	switch node.Kind {
	case yaml.ScalarNode:
		*values = GithubStringList{node.Value}
		return nil
	case yaml.SequenceNode:
		return node.Decode((*[]string)(values))
	default:
		return fmt.Errorf("expected a string or sequence but got %v", node.Kind)
	}
}

// MarshalYAML preserves the concise scalar form for a single value.
func (values GithubStringList) MarshalYAML() (any, error) {
	if len(values) == 1 {
		return values[0], nil
	}
	return []string(values), nil
}

// Input represents an input for a workflow dispatch event.
type GithubInput struct {
	Description string `json:"description,omitempty" yaml:"description,omitempty"`
	Required    bool   `json:"required,omitempty" yaml:"required,omitempty"`
	Default     string `json:"default,omitempty" yaml:"default,omitempty"`
}

// Job represents a job in the GitHub workflow.
type GithubJob struct {
	Name           string                     `json:"name,omitempty" yaml:"name,omitempty"`
	Needs          []string                   `json:"needs,omitempty" yaml:"needs,omitempty"`
	RunsOn         GithubStringList           `json:"runs-on,omitempty" yaml:"runs-on,omitempty"`
	Container      *GithubContainer           `json:"container,omitempty" yaml:"container,omitempty"`
	Services       map[string]GithubContainer `json:"services,omitempty" yaml:"services,omitempty"`
	Steps          []*GithubStep              `json:"steps,omitempty" yaml:"steps,omitempty"`
	If             string                     `json:"if,omitempty" yaml:"if,omitempty"`
	Env            map[string]any             `json:"env,omitempty" yaml:"env,omitempty"`
	Environment    string                     `json:"environment,omitempty" yaml:"environment,omitempty"`
	Permissions    map[string]string          `json:"permissions,omitempty" yaml:"permissions,omitempty"`
	Concurrency    *GithubConcurrency         `json:"concurrency,omitempty" yaml:"concurrency,omitempty"`
	TimeoutMinutes int                        `json:"timeout-minutes,omitempty" yaml:"timeout-minutes,omitempty"`
	Strategy       *GithubStrategy            `json:"strategy,omitempty" yaml:"strategy,omitempty"`

	configured map[string]struct{}
}

// Has reports whether a job field was explicitly configured, including an
// empty or null value.
func (job GithubJob) Has(name string) bool {
	_, ok := job.configured[name]
	return ok
}

// GithubConcurrency represents job-level concurrency configuration. Group is
// populated for both the scalar and mapping YAML forms.
type GithubConcurrency struct {
	Group            string `json:"group,omitempty" yaml:"group,omitempty"`
	CancelInProgress any    `json:"cancel-in-progress,omitempty" yaml:"cancel-in-progress,omitempty"`
}

// UnmarshalYAML normalizes the scalar concurrency shorthand.
func (concurrency *GithubConcurrency) UnmarshalYAML(node *yaml.Node) error {
	if node.Kind == yaml.ScalarNode {
		concurrency.Group = node.Value
		return nil
	}
	type plain GithubConcurrency
	return node.Decode((*plain)(concurrency))
}

// UnmarshalYAML normalizes polymorphic GitHub Actions job fields while keeping
// the generator-facing Go types stable.
func (job *GithubJob) UnmarshalYAML(node *yaml.Node) error {
	if node.Kind != yaml.MappingNode {
		return fmt.Errorf("github job: expected a mapping node but got %v", node.Kind)
	}
	values := yamlMapping(node)
	*job = GithubJob{configured: make(map[string]struct{}, len(values))}
	for name := range values {
		job.configured[name] = struct{}{}
	}
	if err := decodeGithubJobFields(values, job); err != nil {
		return err
	}
	if err := decodeGithubJobNeeds(values["needs"], job); err != nil {
		return err
	}
	if err := decodeGithubJobEnvironment(values["environment"], job); err != nil {
		return err
	}
	if err := decodeGithubJobContainer(values["container"], job); err != nil {
		return err
	}
	return validateGithubJobSteps(job.Steps)
}

func decodeGithubJobFields(values map[string]*yaml.Node, job *GithubJob) error {
	for name, target := range map[string]any{
		"name":            &job.Name,
		"runs-on":         &job.RunsOn,
		"steps":           &job.Steps,
		"if":              &job.If,
		"env":             &job.Env,
		"permissions":     &job.Permissions,
		"strategy":        &job.Strategy,
		"timeout-minutes": &job.TimeoutMinutes,
		"services":        &job.Services,
		"concurrency":     &job.Concurrency,
	} {
		value := values[name]
		if value == nil {
			continue
		}
		if err := value.Decode(target); err != nil {
			return fmt.Errorf("github job field %s: %w", name, err)
		}
	}
	return nil
}

func decodeGithubJobNeeds(needs *yaml.Node, job *GithubJob) error {
	if needs == nil {
		return nil
	}
	switch needs.Kind {
	case yaml.ScalarNode:
		job.Needs = []string{needs.Value}
		return nil
	case yaml.SequenceNode:
		if err := needs.Decode(&job.Needs); err != nil {
			return fmt.Errorf("github job field needs: %w", err)
		}
		return nil
	default:
		return fmt.Errorf("github job field needs: expected a scalar or sequence but got %v", needs.Kind)
	}
}

func decodeGithubJobEnvironment(environment *yaml.Node, job *GithubJob) error {
	if environment == nil {
		return nil
	}
	if environment.Kind == yaml.ScalarNode {
		job.Environment = environment.Value
		return nil
	}
	var configured struct {
		Name string `yaml:"name"`
	}
	if err := environment.Decode(&configured); err != nil {
		return fmt.Errorf("github job field environment: %w", err)
	}
	job.Environment = configured.Name
	return nil
}

func decodeGithubJobContainer(container *yaml.Node, job *GithubJob) error {
	if container == nil || container.Tag == "!!null" {
		return nil
	}
	job.Container = &GithubContainer{}
	if container.Kind == yaml.ScalarNode {
		job.Container.Image = container.Value
		return nil
	}
	if err := container.Decode(job.Container); err != nil {
		return fmt.Errorf("github job field container: %w", err)
	}
	return nil
}

func validateGithubJobSteps(steps []*GithubStep) error {
	for index, step := range steps {
		if step == nil {
			return fmt.Errorf("github job step %d must be a mapping", index)
		}
	}
	return nil
}

func yamlMapping(node *yaml.Node) map[string]*yaml.Node {
	values := map[string]*yaml.Node{}
	for i := 0; node != nil && node.Kind == yaml.MappingNode && i+1 < len(node.Content); i += 2 {
		values[node.Content[i].Value] = node.Content[i+1]
	}
	return values
}

var ErrMustIncludeSubComponents = errors.New("must include at least one")

func (job *GithubJob) AddStep(step *GithubStep) error {
	if step != nil {
		job.Steps = append(job.Steps, step)
		return nil
	}
	return fmt.Errorf("github job, %w", ErrMustIncludeSubComponents)
}

// Container represents a container configuration for a GitHub Action job.
type GithubContainer struct {
	Image       string                      `json:"image" yaml:"image"`                                 // The Docker image to use for the container.
	Credentials *GithubContainerCredentials `json:"credentials,omitempty" yaml:"credentials,omitempty"` // Credentials for the container registry, if required.
	Env         map[string]string           `json:"env,omitempty" yaml:"env,omitempty"`                 // Environment variables for the container.
	Ports       []interface{}               `json:"ports,omitempty" yaml:"ports,omitempty"`             // Array of ports to expose, can be numbers or strings.
	Volumes     []string                    `json:"volumes,omitempty" yaml:"volumes,omitempty"`         // Array of volumes to use in the container.
	Options     string                      `json:"options,omitempty" yaml:"options,omitempty"`         // Additional Docker container options.
}

// ContainerCredentials represents credentials for the container registry.
type GithubContainerCredentials struct {
	Username string `json:"username,omitempty" yaml:"username,omitempty"` // Username for the container registry.
	Password string `json:"password,omitempty" yaml:"password,omitempty"` // Password for the container registry.
}

// Step represents a step in a GitHub job.
type GithubStep struct {
	Name            string            `json:"name,omitempty" yaml:"name,omitempty"`
	ID              string            `json:"id,omitempty" yaml:"id,omitempty"`
	Uses            string            `json:"uses,omitempty" yaml:"uses,omitempty"`
	Run             string            `json:"run,omitempty" yaml:"run,omitempty"`
	With            map[string]string `json:"with,omitempty" yaml:"with,omitempty"`
	Env             map[string]any    `json:"env,omitempty" yaml:"env,omitempty"`
	Shell           string            `json:"shell,omitempty" yaml:"shell,omitempty"`
	ContinueOnError bool              `json:"continue-on-error,omitempty" yaml:"continue_on_error,omitempty"`
	TimeoutMinutes  int               `json:"timeout-minutes,omitempty" yaml:"timeout_minutes,omitempty"`
	If              string            `json:"if,omitempty" yaml:"if,omitempty"`
}

// Defaults represents default values for jobs in the GitHub workflow.
type GithubDefaults struct {
	Run GithubDefaultRun `json:"run,omitempty" yaml:"run,omitempty"`
}

// DefaultRun represents default run configurations for the jobs in the workflow.
type GithubDefaultRun struct {
	Shell            string `json:"shell,omitempty" yaml:"shell,omitempty"`
	WorkingDirectory string `json:"working-directory,omitempty" yaml:"working_directory,omitempty"`
}

// Strategy represents a job strategy (matrix) configuration.
type GithubStrategy struct {
	Matrix      map[string]any `json:"matrix,omitempty" yaml:"matrix,omitempty"`
	MaxParallel int            `json:"max-parallel,omitempty" yaml:"max-parallel,omitempty"`
	FailFast    bool           `json:"fail-fast,omitempty" yaml:"fail-fast,omitempty"`
}

//
// YAML helpers for custom types
//

// OrderedMap preserves insertion order of keys when (un)marshaling YAML.
type OrderedMap struct {
	Keys   []string
	Values map[string]GithubJob
	mu     *sync.Mutex
}

func NewOrderedMap(initialItems ...OrderedMap) OrderedMap {
	om := OrderedMap{Keys: []string{}, Values: map[string]GithubJob{}, mu: &sync.Mutex{}}
	return om
}

func (om *OrderedMap) Add(key string, val GithubJob) {
	om.mu.Lock()
	defer om.mu.Unlock()
	om.Keys = append(om.Keys, key)
	om.Values[key] = val
}

// UnmarshalYAML implements yaml.Unmarshaler.
func (om *OrderedMap) UnmarshalYAML(node *yaml.Node) error {
	if node.Kind != yaml.MappingNode {
		return fmt.Errorf("OrderedMap: expected a mapping node but got %v", node.Kind)
	}
	om.Values = make(map[string]GithubJob, len(node.Content)/2)
	om.Keys = make([]string, 0, len(node.Content)/2)
	// node.Content is [ key1, val1, key2, val2, ... ]
	for i := 0; i < len(node.Content); i += 2 {
		keyNode := node.Content[i]
		valNode := node.Content[i+1]
		key := keyNode.Value

		var v GithubJob
		if err := valNode.Decode(&v); err != nil {
			return err
		}

		om.Keys = append(om.Keys, key)
		om.Values[key] = v
	}
	return nil
}

// MarshalYAML implements yaml.Marshaler.
func (om OrderedMap) MarshalYAML() (any, error) {
	out := &yaml.Node{
		Kind: yaml.MappingNode,
		Tag:  "!!map",
	}
	for _, key := range om.Keys {
		// key node
		out.Content = append(out.Content, &yaml.Node{
			Kind:  yaml.ScalarNode,
			Tag:   "!!str",
			Value: key,
		})
		// value node: marshal the value back into a node
		raw, err := yaml.Marshal(om.Values[key])
		if err != nil {
			return nil, err
		}
		var valDoc yaml.Node
		if err := yaml.Unmarshal(raw, &valDoc); err != nil {
			return nil, err
		}
		// valDoc.Content[0] is the real node for the value
		if len(valDoc.Content) > 0 {
			out.Content = append(out.Content, valDoc.Content[0])
		} else {
			// fallback
			out.Content = append(out.Content, &valDoc)
		}
	}
	return out, nil
}
