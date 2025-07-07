//go:build !windows
// +build !windows

package runner

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/image"
	"github.com/docker/docker/api/types/mount"
	"github.com/sirupsen/logrus"
)

func platformPullOptions(ctx context.Context, containerConf *container.Config) (image.PullOptions, error) {
	afn := AuthLookupFunc(containerConf)
	po := image.PullOptions{
		PrivilegeFunc: afn,
	}
	ra, err := afn(ctx)
	if err != nil {
		return image.PullOptions{}, err
	}
	po.RegistryAuth = ra
	return po, nil
}

func platformContainerConfig(containerContext *ContainerContext, cEnv []string, cmd []string, wd string, tty, attachStdin bool) (*container.Config, *container.HostConfig) {
	containerPorts, hostPorts := containerContext.Ports()

	containerConfig := &container.Config{
		Image:      containerContext.Image,
		Entrypoint: containerContext.Entrypoint,
		Env:        cEnv,
		// These are reserved for named volumes if they don't exist they are created as anonymous volumes
		// TODO: reserve this for future volume management
		Volumes:     map[string]struct{}{},
		Cmd:         cmd,
		Tty:         tty, // TODO: TTY along with StdIn will require switching off stream multiplexer
		AttachStdin: attachStdin,
		// OpenStdin: ,
		// WorkingDir in a container will always be /eirctl
		// will append any job specified paths to the default working
		WorkingDir:   wd,
		User:         containerContext.User(),
		ExposedPorts: containerPorts,
	}

	hostConfig := &container.HostConfig{
		Mounts:       []mount.Mount{},
		Binds:        []string{},
		UsernsMode:   container.UsernsMode(containerContext.userns),
		PortBindings: hostPorts,
		AutoRemove:   true,
	}
	for _, volume := range containerContext.BindMounts() {
		if containerContext.BindMount {
			// use the new mounts
			hostConfig.Mounts = append(hostConfig.Mounts, mount.Mount{
				// TODO: enable additional mount types
				// e.g. `image` for built container volume inspection
				Type:   mount.TypeBind, // current default is bind
				Source: volume.SourcePath,
				Target: volume.TargetPath,
				// FIXME: allow a more comprehensive list of options.
				//
				// Perhaps struct embedding from the docker/api/types package in the context definition.
				// BindOptions:   &mount.BindOptions{},
				// VolumeOptions: &mount.VolumeOptions{},
				// Consistency:   mount.ConsistencyDefault,
				// TmpfsOptions:  &mount.TmpfsOptions{},
			})
			continue
		}
		hostConfig.Binds = append(hostConfig.Binds, fmt.Sprintf("%s:%s:rw", volume.SourcePath, volume.TargetPath))
	}
	// debug config
	logrus.Debugf("ContainerConfig: %+v", containerConfig)
	logrus.Debugf("HostConfig: %+v", hostConfig)
	return containerConfig, hostConfig
}

func mutateShellContainerConfig(containerConfig *container.Config) {
	containerConfig.Tty = true
	containerConfig.OpenStdin = true
	containerConfig.AttachStdin = true
	containerConfig.AttachStdout = true
	containerConfig.AttachStderr = true
	containerConfig.Cmd = []string{containerConfig.Cmd[0]}
	// debug config
	logrus.Debugf("Shell Mutated Unix ContainerConfig: %+v", containerConfig)
}

func resizeSignal() chan os.Signal {
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGWINCH)
	return sigCh
}
