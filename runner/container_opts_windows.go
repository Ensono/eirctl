//go:build windows
// +build windows

package runner

import (
	"context"
	"os"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/image"
	"github.com/docker/docker/api/types/mount"
)

func platformPullOptions(ctx context.Context, containerConf *container.Config) (image.PullOptions, error) {
	pullOpts := image.PullOptions{}

	ra, err := AuthLookupFunc(containerConf)(ctx)
	if err != nil {
		return image.PullOptions{}, err
	}
	pullOpts.RegistryAuth = ra

	return pullOpts, nil
}

func platformContainerConfig(containerContext *ContainerContext, cEnv []string, cmd []string, wd string, tty, attachStdin bool) (*container.Config, *container.HostConfig) {
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
		User:       containerContext.User(),
	}

	hostConfig := &container.HostConfig{Mounts: []mount.Mount{}}
	// only mount of type bind  can be used on windows
	for _, volume := range containerContext.BindMounts() {
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
	}
	return containerConfig, hostConfig
}

func mutateShellContainerConfig(containerConfig *container.Config) {
	containerConfig.Tty = true
	containerConfig.OpenStdin = true
	containerConfig.AttachStdin = true
	containerConfig.AttachStdout = true
	containerConfig.AttachStderr = true
	containerConfig.Cmd = []string{containerConfig.Cmd[0]}
	containerConfig.Env = append(containerConfig.Env, []string{"COLUMNS=120", "LINES=40"}...)
}

func resizeSignal() chan os.Signal {
	// effectively a No-Op
	return make(chan os.Signal, 1)
}
