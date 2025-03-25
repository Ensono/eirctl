package runner

import (
	"bufio"
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path"
	"strings"

	"github.com/Ensono/eirctl/internal/utils"
	"github.com/Ensono/eirctl/variables"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/image"
	"github.com/docker/docker/api/types/network"
	"github.com/docker/docker/client"
	"github.com/docker/docker/pkg/stdcopy"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/sirupsen/logrus"
	"mvdan.cc/sh/v3/interp"
)

var (
	ErrImagePull                        = errors.New("failed to pull container image")
	ErrRegistryAuth                     = errors.New("failed to auth to registry")
	ErrContainerCreate                  = errors.New("failed to create container")
	ErrContainerStart                   = errors.New("failed to start container")
	ErrContainerWait                    = errors.New("failed to wait for container")
	ErrContainerLogs                    = errors.New("failed to get container logs")
	ErrContainerExecCmd                 = errors.New("failed to run cmd in container")
	ErrContainerMultiplexedStdoutStream = errors.New("failed to de-muiltiplex the stream")
)

// ContainerExecutorIface interface used by this implementation
type ContainerExecutorIface interface {
	Close() error
	ImagePull(ctx context.Context, refStr string, options image.PullOptions) (io.ReadCloser, error)
	ContainerCreate(ctx context.Context, config *container.Config, hostConfig *container.HostConfig, networkingConfig *network.NetworkingConfig, platform *ocispec.Platform, containerName string) (container.CreateResponse, error)
	ContainerStart(ctx context.Context, containerID string, options container.StartOptions) error
	ContainerWait(ctx context.Context, containerID string, condition container.WaitCondition) (<-chan container.WaitResponse, <-chan error)
	ContainerLogs(ctx context.Context, containerID string, options container.LogsOptions) (io.ReadCloser, error)
	ContainerInspect(ctx context.Context, containerID string) (container.InspectResponse, error)
}

type ContainerExecutor struct {
	// containerClient
	cc          ContainerExecutorIface
	execContext *ExecutionContext
}

type ContainerOpts func(*ContainerExecutor)

// NewContainerExecutor initialises an OCI compliant client
//
// It implicitely creates it from `env` any missing vars required to initialise it,
// will be flagged in the error response.
func NewContainerExecutor(execContext *ExecutionContext, opts ...ContainerOpts) (*ContainerExecutor, error) {
	// NOTE: potentially check env vars are set here
	// also cover it in tests to ensure errors are handled correctly
	// os.Setenv("DOCKER_HOST", "unix:///var/run/docker.sock")
	c, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		return nil, err
	}

	ce := &ContainerExecutor{
		cc:          c,
		execContext: execContext,
	}

	for _, opt := range opts {
		opt(ce)
	}

	return ce, nil
}

func WithContainerClient(client ContainerExecutorIface) ContainerOpts {
	return func(ce *ContainerExecutor) {
		ce.cc = client
	}
}

func (e *ContainerExecutor) WithReset(doReset bool) {}

// Execute executes given job with provided context
// Returns job output
func (e *ContainerExecutor) Execute(ctx context.Context, job *Job) ([]byte, error) {
	defer e.cc.Close()
	containerContext := e.execContext.Container()
	cmd := containerContext.ShellArgs
	cmd = append(cmd, job.Command)
	tty, attachStdin := false, false
	// if job.Stdin != nil {
	// 	tty = true
	// 	attachStdin = true
	// }
	remoteDir := ""
	if e.execContext.Dir != job.Dir {
		remoteDir = job.Dir
	}

	// everything in the container is relative to the `/eirctl` directory
	wd := path.Join("/eirctl", remoteDir)
	// adding the opiniated PWD into the Container Env as per the wd variable
	cEnv := utils.ConvertEnv(utils.ConvertToMapOfStrings(
		job.Env.Merge(variables.FromMap(map[string]string{"PWD": wd})).
			Merge(variables.FromMap(containerContext.envOverride)).
			Map()))

	containerConfig, hostConfig := platformContainerConfig(containerContext, cEnv, cmd, wd, tty, attachStdin)

	if err := e.PullImage(ctx, containerContext.Image, job.Stdout); err != nil {
		return nil, err
	}

	logrus.Debugf("%+v", containerConfig)

	resp, err := e.cc.ContainerCreate(ctx, containerConfig, hostConfig, nil, nil, "")
	if err != nil {
		return nil, fmt.Errorf("%v\n%w", err, ErrContainerCreate)
	}

	if err := e.cc.ContainerStart(ctx, resp.ID, container.StartOptions{}); err != nil {
		return nil, fmt.Errorf("%v\n%w", err, ErrContainerStart)
	}

	// streamLogs
	errExecCh := make(chan error)
	if err := e.streamLogs(ctx, resp.ID, errExecCh, job); err != nil {
		return nil, err
	}

	statusWaitCh, errWaitCh := e.cc.ContainerWait(ctx, resp.ID, container.WaitConditionNotRunning)
	select {
	case err := <-errWaitCh:
		if err != nil {
			return nil, fmt.Errorf("%v\n%w", err, ErrContainerWait)
		}
	case err := <-errExecCh:
		if err != nil {
			return nil, err
		}
	case <-statusWaitCh:
	}
	return []byte{}, e.checkExitStatus(ctx, resp.ID)
}

const (
	// REGISTRY_AUTH_FILE is the environment variable name
	// for the file to use with container registry authentication
	REGISTRY_AUTH_FILE string = `REGISTRY_AUTH_FILE`
	// REGISTRY_AUTH_USER is the environment variable name for the user
	// to use for authenticating to a registry
	// when it is using a v2 style token, i.e. when the decoded token
	// does *NOT* include a `user:password` style text, it can be something like `00000000-0000-0000-0000-000000000000` for ACR as an example, or another value
	//
	// Defaults to `AWS`.
	REGISTRY_AUTH_USER      string = `REGISTRY_AUTH_USER`
	container_registry_auth string = `{"username":"%s","password":"%s"}`
)

var (
	DOCKER_CONFIG_FILE string = ".docker/config.json"
	PODMAN_CONFIG_FILE string = ".config/containers/auth.json"
)

type AuthFile struct {
	//
	Auths map[string]struct {
		Auth string `json:"auth"`
	} `json:"auths"`
}

func registryAuthFile() (*AuthFile, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, err
	}

	defaultPaths := []string{path.Join(home, DOCKER_CONFIG_FILE), path.Join(home, PODMAN_CONFIG_FILE)}

	if authFile, found := os.LookupEnv(REGISTRY_AUTH_FILE); found {
		// If REGISTRY_AUTH_FILE has been supplied - check it first
		defaultPaths = append([]string{authFile}, defaultPaths...)
	}

	af := &AuthFile{}

	for _, authFile := range defaultPaths {
		if _, err := os.Stat(authFile); err == nil {
			logrus.Debugf("auth file: %s", authFile)
			b, err := os.ReadFile(authFile)
			if err != nil {
				return nil, fmt.Errorf("%w, auth file read: %v", ErrRegistryAuth, err)
			}
			if err := json.Unmarshal(b, af); err != nil {
				return nil, fmt.Errorf("%w, auth file unmarshal: %v", ErrRegistryAuth, err)
			}
			return af, nil
		}
	}
	return nil, fmt.Errorf("%w, no auth file found", ErrRegistryAuth)
}

func AuthLookupFunc(name string) func(ctx context.Context) (string, error) {
	return func(ctx context.Context) (string, error) {
		// extract auth from registry from `REGISTRY_AUTH_FILE`
		logrus.Debug("looking for REGISTRY_AUTH_FILE")

		rc := strings.Split(name, "/")
		registryName := rc[0]
		af, err := registryAuthFile()
		if err != nil {
			return "", err
		}
		for registry, auth := range af.Auths {
			logrus.Debug(registry)
			if registry == registryName {
				decodedToken, err := base64.StdEncoding.DecodeString(auth.Auth)
				if err != nil {
					return "", err
				}

				authToken := strings.Split(string(decodedToken), ":")

				// The decoded token will include `UserName:Password`
				if len(authToken) == 2 {
					logrus.Debug("auth func - basic auth")
					return base64.StdEncoding.EncodeToString(fmt.Appendf(nil, container_registry_auth, authToken[0], authToken[1])), nil
				}
				// The decoded token will include a JSON like string `{"key":""}`
				// in which case it will still use username/password but the whole token is the password
				if strings.Contains(string(decodedToken), "payload") {
					logrus.Debug("auth func - uses a v2 style token")
					user, found := os.LookupEnv(REGISTRY_AUTH_USER)
					if !found {
						user = "AWS"
					}
					return base64.StdEncoding.EncodeToString(fmt.Appendf(nil, container_registry_auth, user, auth.Auth)), nil
				}
				return "", fmt.Errorf("the registry token is not valid")
			}
		}
		return "", nil
	}
}

// Container pull images - all contexts that have a container property
func (e *ContainerExecutor) PullImage(ctx context.Context, name string, dstOutput io.Writer) error {
	logrus.Debugf("pulling image: %s", name)

	pullOpts, err := platformPullOptions(ctx, name)
	if err != nil {
		return err
	}

	reader, err := e.cc.ImagePull(ctx, name, pullOpts)
	if err != nil {
		return fmt.Errorf("%v\n%w", err, ErrImagePull)
	}

	defer reader.Close()
	// container.ImagePull is asynchronous.
	// The reader needs to be read completely for the pull operation to complete.
	// If stdout is not required, consider using io.Discard instead of os.Stdout.
	// Debug log pull image output
	b := &bytes.Buffer{}
	if _, err := io.Copy(b, reader); err != nil {
		return err
	}
	logrus.Debug(b.String())
	return nil
}

func (e *ContainerExecutor) streamLogs(ctx context.Context, containerId string, errCh chan error, job *Job) error {
	out, err := e.cc.ContainerLogs(ctx, containerId, container.LogsOptions{
		ShowStdout: true,
		ShowStderr: true,
		Timestamps: false,
		Follow:     true,
	})
	if err != nil {
		return fmt.Errorf("%v\n%w", err, ErrContainerLogs)
	}

	go func() {
		defer out.Close()
		reader := bufio.NewReader(out)
		for {
			if _, err := stdcopy.StdCopy(job.Stdout, job.Stderr, reader); err != nil {
				// Handle EOF (when logs stop)
				if errors.Is(err, io.EOF) {
					break
				}
				errCh <- fmt.Errorf("%w: %v", ErrContainerMultiplexedStdoutStream, err)
				return
			}
		}
	}()

	return nil
}

func (e *ContainerExecutor) checkExitStatus(ctx context.Context, containerId string) error {
	resp, err := e.cc.ContainerInspect(ctx, containerId)
	if err != nil {
		logrus.Debugf("%v: %v", ErrContainerLogs, err)
		return interp.NewExitStatus(125)

	}
	if resp.State.ExitCode != 0 {
		logrus.Debugf("container image (%s) command %v failed with non-zero exit code", resp.Image, resp.Config.Cmd)
		return interp.NewExitStatus(uint8(resp.State.ExitCode))
	}
	return nil
}

// container attach stdin - via task or context
