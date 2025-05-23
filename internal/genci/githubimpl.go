package genci

import (
	"bytes"
	"errors"
	"fmt"
	"regexp"
	"sort"

	"github.com/Ensono/eirctl/internal/config"
	"github.com/Ensono/eirctl/internal/schema"
	"github.com/Ensono/eirctl/internal/utils"
	"github.com/Ensono/eirctl/scheduler"
	"github.com/Ensono/eirctl/variables"
	"gopkg.in/yaml.v3"
)

var ErrInvalidCiMeta = errors.New("CI meta is invalid")

// githubCiImpl is the implementation of GHA pipeline generation from EirCtl ExecutionGraph
// The graph has to be denormalized to ensure that all env variables are correctly cascaded to the tasks
type githubCiImpl struct {
	eirctlVersion string
	conf          *config.Config
	pipeline      *scheduler.ExecutionGraph
}

func newGithubCiImpl(conf *config.Config) (*githubCiImpl, error) {
	impl := &githubCiImpl{
		eirctlVersion: "v2.0.0",
		conf:          conf,
	}
	if conf.Generate != nil && conf.Generate.Version != "" {
		impl.eirctlVersion = conf.Generate.Version
	}
	return impl, nil
}

func (impl *githubCiImpl) Convert(pipeline *scheduler.ExecutionGraph) ([]byte, error) {
	// use the denormalized pipeline to ensure unique variables are injected into the
	// tasks
	dp, err := pipeline.Denormalize()

	if err != nil {
		return nil, err
	}

	impl.pipeline = dp
	ghaWorkflow := &schema.GithubWorkflow{
		Name: impl.pipeline.Name(), // this can be the raw name as it's a string value not the key
		Jobs: schema.NewOrderedMap(),
	}

	// top level On is required for a valid GHA pipeline
	// if this is missing we need to exit
	if impl.conf.Generate == nil {
		return nil, fmt.Errorf("cannot generate a GHA pipeline (%s) without required "+
			"info in ci_meta, at least the ci_meta.targetOpts.on property must be set, %w",
			pipeline.Name(), ErrInvalidCiMeta)
	}

	gh, err := extractGeneratorMetadata[schema.GithubWorkflow](GitHubCITarget, impl.conf.Generate.TargetOptions)
	if err != nil {
		return nil, fmt.Errorf("failed to parse ci_meta")
	}
	if gh.On == nil {
		return nil, fmt.Errorf("on is a required key for GHA pipeline (%s) it must be included in "+
			"the top level inside the ci_meta.targetOpts: property, %w",
			pipeline.Name(), ErrInvalidCiMeta)
	}
	ghaWorkflow.On = gh.On
	if gh.Env != nil {
		ghaWorkflow.Env = gh.Env
	}

	if err := jobBuilder(ghaWorkflow, impl.pipeline); err != nil {
		return nil, err
	}
	b := &bytes.Buffer{}
	enc := yaml.NewEncoder(b)
	enc.SetIndent(2)
	defer enc.Close()
	if err := enc.Encode(ghaWorkflow); err != nil {
		return nil, err
	}
	return b.Bytes(), nil
}

// addDefaultStepsToJob should be included at the top of each stage
// it injects the required steps for the runner to successfully execute the job.
//
// Checkout step and install eirctl step which will run all subsequent steps.
func addDefaultStepsToJob(job *schema.GithubJob) {
	// toggle if checkout or not
	_ = job.AddStep(&schema.GithubStep{
		Uses: "actions/checkout@v4",
	})
	_ = job.AddStep(&schema.GithubStep{
		Name: "Install eirctl",
		ID:   "install-eirctl",
		Run: `rm -rf /tmp/eirctl-linux-amd64-v1.8.0-alpha-aaaabbbb1234
wget https://github.com/Ensono/eirctl/releases/download/v1.8.0-alpha-aaaabbbb1234/eirctl-linux-amd64 -O /tmp/eirctl-linux-amd64-v1.8.0-alpha-aaaabbbb1234
cp /tmp/eirctl-linux-amd64-v1.8.0-alpha-aaaabbbb1234 /usr/local/bin/eirctl
chmod u+x /usr/local/bin/eirctl`,
		Shell: "bash",
	})
}

func addStepsToTopLevelJob(job *schema.GithubJob, node *scheduler.Stage) {
	if node.Pipeline != nil {
		flattenTasksInPipeline(job, node.Pipeline)
	}
	if node.Task != nil {
		_ = job.AddStep(convertTaskToStep(node))
	}

	// These are top level jobs only
	for _, v := range node.DependsOn {
		job.Needs = append(
			job.Needs,
			// reference it the same way it was set
			ghaNameConverter(utils.TailExtract(v)),
		)
	}
}

func addMetaToJob(job *schema.GithubJob, node *scheduler.Stage) error {
	gh, err := extractGeneratorMetadata[schema.GithubJob](GitHubCITarget, node.Generator)
	if err != nil {
		return fmt.Errorf("unable to extract metadata for %s\n%v", node.Name, err)
	}
	if gh != nil {
		if gh.If != "" {
			job.If = gh.If
		}
		if gh.Environment != "" {
			job.Environment = gh.Environment
		}
		if gh.RunsOn != "" {
			job.RunsOn = gh.RunsOn
		}
		if gh.Env != nil {
			// merge top level pipeline env vars into the top level GHA Job
			job.Env = node.Env().
				Merge(variables.FromMap(utils.ConvertToMapOfStrings(gh.Env))).
				Map()
		}
	}
	return nil
}

func convertTaskToStep(node *scheduler.Stage) *schema.GithubStep {

	step := &schema.GithubStep{
		Name: ghaNameConverter(node.Name),
		ID:   ghaNameConverter(node.Name),
		Run:  fmt.Sprintf("eirctl run task %s", utils.TailExtract(node.Task.Name)),
		Env:  node.Env().Merge(node.Task.Env).Map(),
	}
	if gh, err := extractGeneratorMetadata[schema.GithubStep](GitHubCITarget, node.Generator); err == nil {
		if gh.If != "" {
			step.If = gh.If
		}
		// if env is specified on this level we want to overwrite it
		// with ci_meta.github.env keys
		if gh.Env != nil {
			step.Env = node.Env().
				Merge(node.Task.Env).
				Merge(variables.FromMap(utils.ConvertToMapOfStrings(gh.Env))).
				Map()
		}
	}
	return step
}

// flattenTasksInPipeline extracts all the tasks recursively across pipelines
func flattenTasksInPipeline(job *schema.GithubJob, graph *scheduler.ExecutionGraph) {
	nodes := graph.BFSNodesFlattened(scheduler.RootNodeName)
	// sort nodes according to depends on order
	sort.Sort(nodes)
	for _, node := range nodes {
		if node.Pipeline != nil {
			flattenTasksInPipeline(job, node.Pipeline)
		}
		if node.Task != nil {
			_ = job.AddStep(convertTaskToStep(node))
		}
	}
}

// jobBuilder accepts a list of top level jobs.
//
// Recursively walks the nodes and flattens any nested pipelines
// and adds them to the list of tasks.
//
// Respects the order of execution set in tascktl.
func jobBuilder(ciyaml *schema.GithubWorkflow, pipeline *scheduler.ExecutionGraph) error {
	nodes := pipeline.BFSNodesFlattened(scheduler.RootNodeName)
	// sort nodes according to depends on order
	// ensuring the pipeline will look the same everytime
	// alphabetically sorted top level jobs and same level children
	sort.Sort(nodes)
	for _, node := range nodes {
		jobName := ghaNameConverter(utils.TailExtract(node.Name))
		job := &schema.GithubJob{
			Name:   jobName,
			RunsOn: "ubuntu-24.04",
			Env:    node.Env().Map(),
		}
		// Add defaults
		addDefaultStepsToJob(job)

		// Adds correctly ordered steps to job
		addStepsToTopLevelJob(job, node)

		if err := addMetaToJob(job, node); err != nil {
			return err
		}
		ciyaml.Jobs.Add(jobName, *job)
	}
	return nil
}

// ghaNameConverter is a GHA specific converter of names and IDs which has to conform to the GHA rules
// It has to be alphanumeric and `-` and `_` only
func ghaNameConverter(str string) string {
	rgx, _ := regexp.Compile(`[^A-Za-z0-9\-\_]`)
	return string(rgx.ReplaceAll([]byte(str), []byte(`_`)))
}
