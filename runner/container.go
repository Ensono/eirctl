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
	"github.com/docker/docker/api/types/mount"
	"github.com/docker/docker/api/types/network"
	"github.com/docker/docker/client"
	"github.com/docker/docker/pkg/stdcopy"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/sirupsen/logrus"
)

var (
	ErrImagePull                        = errors.New("failed to pull container image")
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

	containerConfig := &container.Config{
		Image:       containerContext.Image,
		Entrypoint:  containerContext.Entrypoint,
		Env:         cEnv,
		Cmd:         cmd,
		Tty:         tty, // TODO: TTY along with StdIn will require switching off stream multiplexer
		AttachStdin: attachStdin,
		// OpenStdin: ,
		// WorkingDir in a container will always be /eirctl
		// will append any job specified paths to the default working
		WorkingDir: wd,
	}
	if err := e.PullImage(ctx, containerContext.Image, job.Stdout); err != nil {
		return nil, err
	}

	logrus.Debugf("%+v", containerConfig)

	hostConfig := &container.HostConfig{Mounts: []mount.Mount{}}
	if containerContext.MountVolume {
		containerConfig.Volumes = containerContext.Volumes()
	} else {
		for _, volume := range containerContext.BindMounts() {
			hostConfig.Mounts = append(hostConfig.Mounts, mount.Mount{
				Type:   mount.TypeBind,
				Source: volume.SourcePath,
				Target: volume.TargetPath,
				BindOptions: &mount.BindOptions{
					Propagation: mount.PropagationShared,
				},
			})
		}
	}

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

const REGISTRY_AUTH_FILE string = `REGISTRY_AUTH_FILE`

type AuthFile struct {
	//
	Auths map[string]struct {
		Auth string `json:"auth"`
	} `json:"auths"`
}

func AuthLookupFunc(name string) func(ctx context.Context) (string, error) {
	return func(ctx context.Context) (string, error) {
		// extract auth from registry from `REGISTRY_AUTH_FILE`
		regContainer := strings.Split(name, "/")
		if authFile, found := os.LookupEnv(REGISTRY_AUTH_FILE); found {
			b, err := os.ReadFile(authFile)
			if err != nil {
				return "", err
			}
			af := &AuthFile{}
			if err := json.Unmarshal(b, af); err != nil {
				return "", err
			}
			for registry, auth := range af.Auths {
				if registry == regContainer[0] {
					decodedToken, err := base64.StdEncoding.DecodeString(auth.Auth)
					if err != nil {
						return "", err
					}
					authToken := strings.Split(string(decodedToken), ":")
					if len(authToken) != 2 {
						return "", fmt.Errorf("the registry token is not valid")
					}
					return base64.StdEncoding.EncodeToString(fmt.Appendf(nil, `{"username":"%s","password":"%s"}`, authToken[0], authToken[1])), nil
				}
			}
		}
		return "", nil
	}
}

// Container pull images - all contexts that have a container property
func (e *ContainerExecutor) PullImage(ctx context.Context, name string, dstOutput io.Writer) error {
	logrus.Debug(name)
	reader, err := e.cc.ImagePull(ctx, name, image.PullOptions{
		PrivilegeFunc: AuthLookupFunc(name),
	})

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

		// Wrap `out` in a buffered reader to prevent partial reads
		reader := bufio.NewReader(out)
		// Loop to continuously read from the log stream
		for {
			// Read one chunk of data
			buf := make([]byte, 4096) // Read in chunks of 4KB
			n, err := reader.Read(buf)
			if n > 0 {
				if _, err := stdcopy.StdCopy(job.Stdout, job.Stderr, bytes.NewReader(buf[:n])); err != nil {
					errCh <- fmt.Errorf("%w: %v", ErrContainerMultiplexedStdoutStream, err)
					return
				}
			}

			// Handle EOF (when logs stop)
			if err == io.EOF {
				// Stop reading once EOF is reached
				// will go to check the stderr stream
				break
			}
			if err != nil {
				errCh <- fmt.Errorf("error reading logs: %v", err)
				return
			}
		}
	}()

	return nil
}
func (e *ContainerExecutor) checkExitStatus(ctx context.Context, containerId string) error {
	resp, err := e.cc.ContainerInspect(ctx, containerId)
	if err != nil {
		return fmt.Errorf("%w, %v", ErrContainerLogs, err)
	}
	if resp.State.ExitCode != 0 {
		return fmt.Errorf("container image (%s) command %v failed with non-zero exit code, %w", resp.Image, resp.Config.Cmd, ErrContainerExecCmd)
	}
	return nil
}

// container attach stdin - via task or context
