package schema

type Bitbucket struct {
	Clone *CloneRepositorySettings `yaml:"clone,omitempty"`
	// The definitions of caches and services used in the declared pipelines.
	Definitions *GlobalDefinitions `yaml:"definitions,omitempty"`
	// Allows other Bitbucket repositories to import pipeline definitions from this file. A
	// shared pipeline definition can't contain another `import` property.
	Export *bool  `yaml:"export,omitempty"`
	Image  *Image `yaml:"image,omitempty"`
	// Additional key value data supplied in the configuration YAML.
	Labels map[string]any `yaml:"labels,omitempty"`
	// Global options allow to override the default values applied to all steps in all declared
	// pipelines.
	Options   *GlobalOptions `yaml:"options,omitempty"`
	Pipelines *Pipelines     `yaml:"pipelines,omitempty"`
}

// Settings for cloning a repository into a container.
type CloneRepositorySettings struct {
	// The depth argument of Git clone operation. It can be either number or "full" value
	Depth *GitCloneDepth `yaml:"depth,omitempty"`
	// Enables cloning of the repository.
	Enabled *bool `yaml:"enabled,omitempty"`
	// Enables the download of files from LFS storage when cloning.
	LFS *bool `yaml:"lfs,omitempty"`
	// Disables SSL verification during Git clone operation, allowing the use of self-signed
	// certificates.
	SkipSSLVerify *bool `yaml:"skip-ssl-verify,omitempty"`
}

// The definitions of caches and services used in the declared pipelines.
type GlobalDefinitions struct {
	Caches map[string]*CachValue `yaml:"caches,omitempty"`
	// Definitions of the pipelines which can be used in other repositories of the same
	// Bitbucket workspace.
	Pipelines map[string]*PipelineValue    `yaml:"pipelines,omitempty"`
	Services  map[string]ServiceDefinition `yaml:"services,omitempty"`
}

type Cache struct {
	Key  *CacheKey `yaml:"key,omitempty"`
	Path string    `yaml:"path"`
}

type CacheKey struct {
	// Checksum of these file paths will be used to generate the cache key.
	Files []string `yaml:"files"`
}

// List of variables, steps, stages and parallel groups of the custom pipeline.
//
// List of steps in the parallel group to run concurrently.
type CustomPipelineItem struct {
	// List of variables for the custom pipeline.
	Variables []CustomPipelineVariable `yaml:"variables,omitempty"`
	Step      *ParallelGroupStepStep   `yaml:"step,omitempty"`
	Parallel  *Parallel                `yaml:"parallel,omitempty"`
	Stage     *Stage                   `yaml:"stage,omitempty"`
}

// List of steps in the parallel group to run concurrently.
type ParallelGroupStep struct {
	Step *ParallelGroupStepStep `yaml:"step,omitempty"`
}

type ParallelGroupStepStep struct {
	// List of commands to execute after the step succeeds or fails.
	AfterScript []AfterScriptElement `yaml:"after-script,omitempty"`
	Artifacts   *Artifacts           `yaml:"artifacts,omitempty"`
	// Caches enabled for the step.
	Caches []string                 `yaml:"caches,omitempty"`
	Clone  *CloneRepositorySettings `yaml:"clone,omitempty"`
	// The deployment environment for the step.
	Deployment *string `yaml:"deployment,omitempty"`
	// Stop the parent parallel group in case this step fails.
	FailFast *bool  `yaml:"fail-fast,omitempty"`
	Image    *Image `yaml:"image,omitempty"`
	MaxTime  *int64 `yaml:"max-time,omitempty"`
	// The name of the step.
	Name *string `yaml:"name,omitempty"`
	// Enables the use of OpenID Connect to connect a pipeline step to a resource server.
	Oidc    *bool        `yaml:"oidc,omitempty"`
	RunsOn  *RunsOn      `yaml:"runs-on,omitempty"`
	Runtime *StepRuntime `yaml:"runtime,omitempty"`
	// List of commands that are executed in sequence.
	Script []AfterScriptElement `yaml:"script"`
	// Services enabled for the step.
	Services []string  `yaml:"services,omitempty"`
	Size     *StepSize `yaml:"size,omitempty"`
	// The trigger used for the pipeline step.
	Trigger *Trigger `yaml:"trigger,omitempty"`
	// The condition to execute the step.
	Condition *Condition `yaml:"condition,omitempty"`
}

// The pipe to execute.
type Pipe struct {
	// The full pipe identifier.
	Pipe string `yaml:"pipe"`
	// Environment variables passed to the pipe container.
	Variables map[string]*PipeVariable `yaml:"variables,omitempty"`
}

type ArtifactsClass struct {
	// Enables downloading of all available artifacts at the start of a step.
	Download *bool    `yaml:"download,omitempty"`
	Paths    []string `yaml:"paths,omitempty"`
}

// The condition to execute the step.
//
// The condition to execute the stage.
type Condition struct {
	// Condition on the changesets involved in the pipeline.
	Changesets ChangesetCondition `yaml:"changesets"`
}

// Condition on the changesets involved in the pipeline.
type ChangesetCondition struct {
	// Condition which holds only if all of the modified files match any of the specified
	// patterns.
	ExcludePaths []string `yaml:"excludePaths,omitempty"`
	// Condition which holds only if any of the modified files match any of the specified
	// patterns.
	IncludePaths []string `yaml:"includePaths,omitempty"`
}

// The parameters of the Docker image to use when running a step.
type ImageClass struct {
	Name string `yaml:"name"`
	// The UID of a user in the docker image to run as. Overrides image's default user,
	// specified user UID must be an existing user in the image with a valid home directory.
	RunAsUser *int64 `yaml:"run-as-user,omitempty"`
	Aws       any    `yaml:"aws,omitempty"`
	// The password to use when fetching the Docker image.
	Password any `yaml:"password,omitempty"`
	// The username to use when fetching the Docker image.
	Username any `yaml:"username,omitempty"`
}

// Custom step runtime
type StepRuntime struct {
	Cloud *CloudStepRuntime `yaml:"cloud,omitempty"`
}

// Custom cloud step runtime
type CloudStepRuntime struct {
	// Architecture type used to run the step.
	Arch *Arch `yaml:"arch,omitempty"`
	// Whether it uses Atlassian ip ranges.
	AtlassianIPRanges *bool `yaml:"atlassian-ip-ranges,omitempty"`
	// Cloud Runtime version.
	Version *string `yaml:"version,omitempty"`
}

type ParallelClass struct {
	// Stop the whole parallel group in case one of its steps fails.
	FailFast *bool               `yaml:"fail-fast,omitempty"`
	Steps    []ParallelGroupStep `yaml:"steps"`
}

type Stage struct {
	// The condition to execute the stage.
	Condition *Condition `yaml:"condition,omitempty"`
	// The deployment environment for the stage.
	Deployment *string `yaml:"deployment,omitempty"`
	// The name of the stage.
	Name *string `yaml:"name,omitempty"`
	// List of steps in the stage.
	Steps []StageStep `yaml:"steps"`
	// The trigger used for the pipeline stage.
	Trigger *Trigger `yaml:"trigger,omitempty"`
}

type StageStep struct {
	Step *StepStep `yaml:"step,omitempty"`
}

type StepStep struct {
	// List of commands to execute after the step succeeds or fails.
	AfterScript []AfterScriptElement `yaml:"after-script,omitempty"`
	Artifacts   *Artifacts           `yaml:"artifacts,omitempty"`
	// Caches enabled for the step.
	Caches []string                 `yaml:"caches,omitempty"`
	Clone  *CloneRepositorySettings `yaml:"clone,omitempty"`
	// The deployment environment for the step.
	Deployment *string `yaml:"deployment,omitempty"`
	// Stop the parent parallel group in case this step fails.
	FailFast *bool  `yaml:"fail-fast,omitempty"`
	Image    *Image `yaml:"image,omitempty"`
	MaxTime  *int64 `yaml:"max-time,omitempty"`
	// The name of the step.
	Name *string `yaml:"name,omitempty"`
	// Enables the use of OpenID Connect to connect a pipeline step to a resource server.
	Oidc    *bool        `yaml:"oidc,omitempty"`
	RunsOn  *RunsOn      `yaml:"runs-on,omitempty"`
	Runtime *StepRuntime `yaml:"runtime,omitempty"`
	// List of commands that are executed in sequence.
	Script []AfterScriptElement `yaml:"script"`
	// Services enabled for the step.
	Services []string  `yaml:"services,omitempty"`
	Size     *StepSize `yaml:"size,omitempty"`
	// The trigger used for the pipeline step.
	Trigger   *Trigger `yaml:"trigger,omitempty"`
	Condition any      `yaml:"condition,omitempty"`
}

// Settings for the custom variable.
type CustomPipelineVariable struct {
	// A list of values that are allowed for the variable.
	AllowedValues []string `yaml:"allowed-values,omitempty"`
	Default       *string  `yaml:"default,omitempty"`
	Description   *string  `yaml:"description,omitempty"`
	Name          string   `yaml:"name"`
}

type DefaultClass struct {
	// The import needs to match the following format:
	// {repo-slug|repo-uuid}:{tag-name|branch-name}:{pipeline-name}.
	Import string `yaml:"import"`
}

// Custom service properties
type ServiceDefinition struct {
	Image *Image `yaml:"image,omitempty"`
	// Memory limit for the service container, in megabytes.
	Memory *int64 `yaml:"memory,omitempty"`
	// Specifies Docker service container (to run Docker-in-Docker).
	Type *Type `yaml:"type,omitempty"`
	// Environment variables passed to the service container.
	Variables map[string]string `yaml:"variables,omitempty"`
}

// Global options allow to override the default values applied to all steps in all declared
// pipelines.
type GlobalOptions struct {
	// Enables Docker service for every step.
	Docker  *bool        `yaml:"docker,omitempty"`
	MaxTime *int64       `yaml:"max-time,omitempty"`
	Runtime *StepRuntime `yaml:"runtime,omitempty"`
	Size    *StepSize    `yaml:"size,omitempty"`
}

type Pipelines struct {
	// Branch-specific build pipelines.
	Branches map[string]*DefaultPipeline `yaml:"branches,omitempty"`
	// Pipelines that can only be triggered manually or be scheduled.
	Custom map[string]*PipelineValue `yaml:"custom,omitempty"`
	// Default pipeline runs on every push except for tags unless a branch-specific pipeline is
	// defined.
	Default *DefaultPipeline `yaml:"default,omitempty"`
	// Pull-request-specific build pipelines.
	PullRequests map[string]*PullRequestValue `yaml:"pull-requests,omitempty"`
	// Tag-specific build pipelines.
	Tags map[string]*DefaultPipeline `yaml:"tags,omitempty"`
}

// List of steps, stages and parallel groups of the pipeline.
//
// List of steps in the parallel group to run concurrently.
type PipelineItem struct {
	Step     *ParallelGroupStepStep `yaml:"step,omitempty"`
	Parallel *Parallel              `yaml:"parallel,omitempty"`
	Stage    *Stage                 `yaml:"stage,omitempty"`
}

type PullRequestClass struct {
	Destinations map[string]*DefaultPipeline `yaml:"destinations,omitempty"`
}

type GitCloneDepthEnum string

const (
	Full GitCloneDepthEnum = "full"
)

// Architecture type used to run the step.
type Arch string

const (
	Arm Arch = "arm"
	X86 Arch = "x86"
)

// The size of the step, sets the amount of resources allocated.
type StepSize string

const (
	The16X StepSize = "16x"
	The1X  StepSize = "1x"
	The2X  StepSize = "2x"
	The32X StepSize = "32x"
	The4X  StepSize = "4x"
	The8X  StepSize = "8x"
)

// The trigger used for the pipeline step.
//
// The trigger used for the pipeline stage.
type Trigger string

const (
	Automatic Trigger = "automatic"
	Manual    Trigger = "manual"
)

// Specifies Docker service container (to run Docker-in-Docker).
type Type string

const (
	Docker Type = "docker"
)

// The depth argument of Git clone operation. It can be either number or "full" value
type GitCloneDepth struct {
	Enum    *GitCloneDepthEnum
	Integer *int64
}

type CachValue struct {
	Cache  *Cache
	String *string
}

type PipelineValue struct {
	CustomPipelineItemArray []CustomPipelineItem
	DefaultClass            *DefaultClass
}

type Parallel struct {
	ParallelClass          *ParallelClass
	ParallelGroupStepArray []ParallelGroupStep
}

// List of commands to execute after the step succeeds or fails.
type AfterScriptElement struct {
	Pipe   *Pipe
	String *string
}

// Environment variable value
type PipeVariable struct {
	String      *string
	StringArray []string
}

type Artifacts struct {
	ArtifactsClass *ArtifactsClass
	StringArray    []string
}

type Image struct {
	ImageClass *ImageClass
	String     *string
}

type RunsOn struct {
	String      *string
	StringArray []string
}

// Default pipeline runs on every push except for tags unless a branch-specific pipeline is
// defined.
type DefaultPipeline struct {
	DefaultClass      *DefaultClass
	PipelineItemArray []PipelineItem
}

type PullRequestValue struct {
	PipelineItemArray []PipelineItem
	PullRequestClass  *PullRequestClass
}
