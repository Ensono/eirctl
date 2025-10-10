package runner

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"regexp"
	"slices"
	"strings"
	"sync"

	"github.com/Ensono/eirctl/internal/utils"
	"github.com/Ensono/eirctl/variables"
	"github.com/docker/go-connections/nat"
	"github.com/sirupsen/logrus"
	"github.com/spf13/pflag"
)

var (
	// define a list of environment variable names that are not permitted
	invalidEnvVarKeys = []string{
		"",                              // skip any empty key names
		`!::`, `=::`, `::=::`, `::=::\`, // this is found in a cygwin environment
	}
	// ipPortProtoFmt f-string for `IP:Port:Port/protocol` mapping to use with the moby nat package
	ipPortProtoFmt string = "0.0.0.0:%s/tcp"
)

type ContainerContext struct {
	Image      string
	Entrypoint []string
	ShellArgs  []string
	// BindMount uses --mount instead of -v
	//
	// when running on Windows mount is default as volume mapping does not work.
	BindMount bool
	volumes   map[string]struct{}
	// isPrivileged bool
	envOverride map[string]string
	// user is a container arg specified via --user/-u
	// can be both user or user:group
	user string
	// userns specifies the NS mode in the container - e.g. private, host, container:id
	// defaults to "" i.e. no remapping occurs
	userns string
	// Ports map from host to container
	ports []string
}

// NewContainerContext accepts name of the image
func NewContainerContext(name string) *ContainerContext {
	return &ContainerContext{
		Image:       name,
		volumes:     make(map[string]struct{}),
		envOverride: make(map[string]string),
	}
}

func (c *ContainerContext) WithVolumes(vols ...string) *ContainerContext {
	for _, v := range vols {
		c.volumes[v] = struct{}{}
	}
	return c
}

type containerArgs struct {
	args    []string
	flagSet *pflag.FlagSet
}

type singleUseFlagString struct {
	val   string
	count int
}

func (s *singleUseFlagString) String() string {
	return s.val
}

func (s *singleUseFlagString) Set(v string) error {
	s.count++
	if s.count > 1 {
		return fmt.Errorf("error in container_args, user flag (-u/--user) already specified (%v). found: %s", s.val, v)
	}
	s.val = v
	return nil
}

func (s *singleUseFlagString) Type() string {
	return "string"
}

func newContainerArgs(cargs []string) *containerArgs {
	// Create a new FlagSet to parse this single flag
	// let pflag do the work
	// Add additional flags we want to handle here
	userVarFlag := &singleUseFlagString{}
	flagSet := pflag.NewFlagSet("containerArgsTempFlags", pflag.ContinueOnError)
	_ = flagSet.StringArrayP("volume", "v", []string{}, "")
	_ = flagSet.StringArrayP("port", "p", []string{}, "")
	flagSet.VarP(userVarFlag, "user", "u", "")
	_ = flagSet.StringP("userns", "", "", "")
	// TODO: refactor this to use first class properties
	// on the container object in config/container_definition

	return &containerArgs{cargs, flagSet}
}

func (c *ContainerContext) ParseContainerArgs(cargs []string) (*ContainerContext, error) {

	if err := newContainerArgs(cargs).parseArgs(c); err != nil {
		return nil, err
	}

	return c, nil
}

func (ca *containerArgs) parseArgs(cc *ContainerContext) error {
	osArgs := []string{}
	for _, v := range ca.args {
		// expand env on the whole slice item in case
		// both the key and value are both coming from an env variable
		osArgs = append(osArgs, strings.Fields(os.ExpandEnv(v))...)
	}

	if err := ca.flagSet.Parse(osArgs); err != nil {
		return err
	}

	user, err := ca.flagSet.GetString("user")
	if err != nil {
		return err
	}
	cc.user = os.ExpandEnv(strings.TrimSpace(user))

	ports, err := ca.flagSet.GetStringArray("port")
	if err != nil {
		return err
	}
	for _, v := range ports {
		// Create a string in this format to levarage the native moby parser
		// ip:public:private/proto
		// Hardcoding ip as 0.0.0.0 and proto as tcp
		//
		// This will handle 99% of use cases - and would work well on the
		// WSL2 backed docker on windows, some edge cases might present
		// themselves on WSL1 (and HyperV) with Windows as the networking
		// was handled via a VM which does its own port mapping
		cc.ports = append(cc.ports, fmt.Sprintf(ipPortProtoFmt, v))
	}

	userns, err := ca.flagSet.GetString("userns")
	if err != nil {
		return err
	}
	cc.userns = os.ExpandEnv(strings.TrimSpace(userns))

	if err := ca.parseVolumes(cc); err != nil {
		return err
	}
	// add more parsers here if needed
	return nil
}

func (ca *containerArgs) parseVolumes(cc *ContainerContext) error {
	vols := []string{}
	volArgs, err := ca.flagSet.GetStringArray("volume")
	if err != nil {
		return err
	}
	for _, v := range volArgs {
		vols = append(vols, expandVolumeString(strings.TrimSpace(v)))
	}
	cc.WithVolumes(vols...)
	return nil
}

// expandVolumeString accepts a string in the form of:
//
//	`-v /path/on/host:/path/in/container`
//
// converts any env into full string, for example:
//
//	`-v ${HOME}/foo:/app/foo` => `/Users/me/foo:/app/foo`
//
// Special consideration will be put on `~` and replaced by HOME variable
func expandVolumeString(vol string) string {
	return os.ExpandEnv(utils.NormalizeHome(vol))
}

func (c *ContainerContext) Volumes() map[string]struct{} {
	return c.volumes
}

func (c *ContainerContext) User() string {
	return c.user
}

func (c *ContainerContext) Ports() (map[nat.Port]struct{}, map[nat.Port][]nat.PortBinding) {
	containerPorts, hostPorts, err := nat.ParsePortSpecs(c.ports)
	if err != nil {
		return map[nat.Port]struct{}{}, map[nat.Port][]nat.PortBinding{}
	}
	return containerPorts, hostPorts
}

// BindVolume formatted for bindmount
type BindVolume struct {
	// SourcePath is the path on the host
	SourcePath string
	// TargetPath is the path in the container
	TargetPath string
}

// BindMounts returns the volumes in a bind mount format
func (c *ContainerContext) BindMounts() []BindVolume {
	bv := []BindVolume{}
	for vol := range c.volumes {
		// NOTE: to avoid potential windows issues with `C:\`
		// we split on the `:/`
		// The target mount path MUST always be an absolute path i.e. `/path/in/container`
		splitVol := strings.Split(vol, ":/")
		// adjusted for edge scenarios with windows and mac folder names
		bv = append(bv, BindVolume{SourcePath: strings.Join(splitVol[0:len(splitVol)-1], ":/"), TargetPath: "/" + splitVol[len(splitVol)-1]})
	}
	return bv
}

func (c *ContainerContext) WithEnvOverride(env map[string]string) *ContainerContext {
	for k, v := range env {
		c.envOverride[k] = v
	}
	return c
}

// ExecutionContext allow you to set up execution environment, variables, binary which will run your task, up/down commands etc.
type ExecutionContext struct {
	SourceFile string
	Executable *utils.Binary
	container  *ContainerContext
	Dir        string
	Env        *variables.Variables
	Envfile    *utils.Envfile
	Variables  *variables.Variables
	// Quote character to use around a command
	// when passed to another executable, e.g. docker
	Quote string

	up           []string
	down         []string
	before       []string
	after        []string
	startupError error
	onceUp       sync.Once
	onceDown     sync.Once
	mu           *sync.Mutex
}

// ExecutionContextOption is a functional option to configure ExecutionContext
type ExecutionContextOption func(c *ExecutionContext)

// NewExecutionContext creates new ExecutionContext instance
func NewExecutionContext(executable *utils.Binary, dir string,
	env *variables.Variables, envfile *utils.Envfile, up, down, before, after []string,
	options ...ExecutionContextOption) *ExecutionContext {
	c := &ExecutionContext{
		// mu is a pointer to a mutex
		// so that it's shared across all
		// the instances that are using the given ExecutionContext
		mu:        &sync.Mutex{},
		Variables: variables.NewVariables(),
	}

	for _, o := range options {
		o(c)
	}

	c.Executable = executable
	c.Env = env
	c.Envfile = envfile
	c.Dir = dir
	c.up = up
	c.down = down
	c.before = before
	c.after = after

	return c
}

func WithContainerOpts(containerOpts *ContainerContext) ExecutionContextOption {
	return func(c *ExecutionContext) {
		c.container = containerOpts
		// add additional closed properties
	}
}

func (c *ExecutionContext) Container() *ContainerContext {
	return c.container
}

type ExecutorType string

const (
	DefaultExecutorTyp   ExecutorType = "default"
	ContainerExecutorTyp ExecutorType = "container"
)

func (c *ExecutionContext) GetExecutorType() ExecutorType {
	if c.container != nil {
		return ContainerExecutorTyp
	}
	return DefaultExecutorTyp
}

// StartUpError reports whether an error exists on startUp
func (c *ExecutionContext) StartupError() error {
	return c.startupError
}

// Up executes tasks defined to run once before first usage of the context
func (c *ExecutionContext) Up() error {
	c.onceUp.Do(func() {
		for _, command := range c.up {
			err := c.runServiceCommand(command)
			if err != nil {
				c.mu.Lock()
				c.startupError = err
				c.mu.Unlock()
				logrus.Errorf("context startup error: %s", err)
			}
		}
	})

	return c.startupError
}

// Down executes tasks defined to run once after last usage of the context
func (c *ExecutionContext) Down() {
	c.onceDown.Do(func() {
		for _, command := range c.down {
			err := c.runServiceCommand(command)
			if err != nil {
				logrus.Errorf("context cleanup error: %s", err)
			}
		}
	})
}

// Before executes tasks defined to run before every usage of the context
func (c *ExecutionContext) Before() error {
	for _, command := range c.before {
		err := c.runServiceCommand(command)
		if err != nil {
			return err
		}
	}

	return nil
}

// After executes tasks defined to run after every usage of the context
func (c *ExecutionContext) After() error {
	for _, command := range c.after {
		err := c.runServiceCommand(command)
		if err != nil {
			return err
		}
	}

	return nil
}

var ErrMutuallyExclusiveVarSet = errors.New("mutually exclusive vars have been set")

// ProcessEnvfile processes env and other supplied variables into a single context environment
func (c *ExecutionContext) ProcessEnvfile(env *variables.Variables) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	// return an error if the include and exclude have both been specified
	if len(c.Envfile.Exclude) > 0 && len(c.Envfile.Include) > 0 {
		return fmt.Errorf("include and exclude lists are mutually exclusive, %w", ErrMutuallyExclusiveVarSet)
	}

	// create a string builder object to hold all of the lines that need to be written out to
	// the resultant file
	builder := []string{}
	envfileEnv := variables.NewVariables()
	// iterate through all of the environment variables and add the selected ones to the builder
	// env container at this point should already include all the merged variables by precedence
	// if envfile path was provided it is merged with Env and inject as a whole into the container
	if readers, found := utils.ReaderFromPath(c.Envfile); readers != nil && found {
		// the last one wins
		for _, reader := range readers {
			if envFile, err := utils.ReadEnvFile(reader); envFile != nil && err == nil {
				envfileEnv = envfileEnv.Merge(variables.FromMap(envFile))
			}
		}
	}

	// env from OS > env property with the file
	// i.e. envfile keys do not overwrite OS or directly set env properties
	env = envfileEnv.Merge(env)
	for varName, varValue := range env.Map() {
		// check to see if the env matches an invalid variable, if it does
		// move onto the next item in the  loop
		if slices.Contains(invalidEnvVarKeys, varName) {
			logrus.Tracef("Skipping invalid env var: %s=%v\n'%s' is not a valid key", varName, varValue, varName)
			continue
		}

		varName = c.modifyName(varName)
		// determine if the variable should be included or excluded
		if c.includeExcludeSkip(varName) {
			continue
		}

		// sanitize variable values from newline and space characters
		// replace any newline characters with a space, this is to prevent multiline variables being passed in
		// quote the value if it has spaces in it
		// Add the name and the value to the string builder
		envstr := fmt.Sprintf("%s=%s", varName, varValue)
		builder = append(builder, envstr)
	}
	c.Env = variables.FromMap(utils.ConvertFromEnv(builder))
	return nil
}

func (c *ExecutionContext) includeExcludeSkip(varName string) bool {
	// set var name to lower to ensure case-insensitive comparison
	varName = strings.ToLower(varName)
	// ShouldExclude will be true if any varName
	shouldExclude := slices.ContainsFunc(c.Envfile.Exclude, func(v string) bool {
		return strings.HasPrefix(varName, strings.ToLower(v))
	})

	shouldInclude := true
	if len(c.Envfile.Include) > 0 {
		shouldInclude = slices.ContainsFunc(c.Envfile.Include, func(v string) bool {
			return strings.HasPrefix(varName, strings.ToLower(v))
		})
	}

	// if the variable should excluded or not explicitly included then move onto the next variable
	return shouldExclude || !shouldInclude
}

func (c *ExecutionContext) modifyName(varName string) string {
	// iterate around the modify options to see if the name needs to be
	// modified at all
	for _, modify := range c.Envfile.Modify {

		// use the pattern to determine if the string has been identified
		// this assumes 1 capture group so this will be used as the name to transform
		re := regexp.MustCompile(modify.Pattern)
		match := re.FindStringSubmatch(varName)
		if len(match) > 0 {

			keyword := match[re.SubexpIndex("keyword")]
			matchedVarName := match[re.SubexpIndex("varname")]

			// perform the operation on the varname
			switch modify.Operation {
			case "lower":
				matchedVarName = strings.ToLower(matchedVarName)
			case "upper":
				matchedVarName = strings.ToUpper(matchedVarName)
			}
			// Build up the name
			return fmt.Sprintf("%s%s", keyword, matchedVarName)
		}
	}
	return varName
}

// runServiceCommand runs all the up,down,before,after commands
// currently this is run outside of the context and always in the mvdn shell
//
// TODO: run serviceCommand in the same context as the command slice
func (c *ExecutionContext) runServiceCommand(command string) (err error) {
	logrus.Tracef("running context service command: %s", command)
	ex, err := newDefaultExecutor(nil, io.Discard, io.Discard)
	if err != nil {
		return err
	}

	out, err := ex.Execute(context.Background(), &Job{
		Command: command,
		Dir:     c.Dir,
		Env:     c.Env,
		Vars:    c.Variables,
	})
	if err != nil {
		if out != nil {
			logrus.Warning(string(out))
		}

		return err
	}
	return nil
}

// DefaultContext creates default ExecutionContext instance
func DefaultContext() *ExecutionContext {
	// the default context still needs access to global env variables
	return NewExecutionContext(nil, "",
		variables.FromMap(utils.ConvertFromEnv(os.Environ())),
		&utils.Envfile{},
		[]string{},
		[]string{},
		[]string{},
		[]string{},
	)
}

// WithQuote is functional option to set Quote for ExecutionContext
func WithQuote(quote string) ExecutionContextOption {
	return func(c *ExecutionContext) {
		c.Quote = "'"
		if quote != "" {
			c.Quote = quote
		}
	}
}
