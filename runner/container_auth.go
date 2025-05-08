package runner

import (
	"bytes"
	"context"
	"encoding/base64"
	"fmt"
	"os"
	"path"
	"strings"

	"github.com/Ensono/eirctl/internal/utils"
	"github.com/docker/cli/cli/config"
	"github.com/docker/cli/cli/config/configfile"
	"github.com/docker/docker/api/types/container"
	"github.com/sirupsen/logrus"
)

const (
	// TODO: potentially re-create the old docker behaviour
	// of a lookup in a directory
	DOCKER_CONFIG string = `DOCKER_CONFIG`

	// REGISTRY_AUTH_FILE is the environment variable name
	// for the file to use with container registry authentication
	REGISTRY_AUTH_FILE string = `REGISTRY_AUTH_FILE`
	// REGISTRY_AUTH_USER is the environment variable name for the user
	// to use for authenticating to a registry
	// when it is using a v2 style token, i.e. when the decoded token
	// does *NOT* include a `user:password` style text, it can be something like `00000000-0000-0000-0000-000000000000` for ACR as an example, or another value

	container_registry_auth_basic string = `{"username":"%s","password":"%s"}`
	container_registry_auth_oidc  string = `{"identitytoken":"%s"}`
)

var (
	DOCKER_CONFIG_FILE    string = ".docker/config.json"
	CONTAINER_CONFIG_FILE string = ".config/containers/auth.json"
)

func registryAuthFile(contextEnv []string) (*configfile.ConfigFile, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, err
	}

	authFiles := []string{path.Join(home, DOCKER_CONFIG_FILE), path.Join(home, CONTAINER_CONFIG_FILE)}

	containerEnv := utils.ConvertFromEnv(contextEnv)

	// break if found REGISTRY_AUTH_FILE variable
	// if this is registered first it will always have preference over DOCKER_CONFIG
	if regFile, found := containerEnv[REGISTRY_AUTH_FILE]; found {
		authFiles = append([]string{regFile}, authFiles...)
	} else {
		// if registry is not specified
		// check docker to maintain the old docker config directory behaviour
		// it must contain a file called `config.json` in this directory
		if regFile, found := containerEnv[DOCKER_CONFIG]; found {
			authFiles = append([]string{path.Join(regFile, "config.json")}, authFiles...)
		}
	}

	for _, authFile := range authFiles {
		logrus.Debugf("trying file: %s", authFile)
		if _, err := os.Stat(authFile); err == nil {
			logrus.Debugf("auth file: %s", authFile)
			b, err := os.ReadFile(authFile)
			if err != nil {
				return nil, fmt.Errorf("%w, auth file read: %v", ErrRegistryAuth, err)
			}
			af, err := config.LoadFromReader(bytes.NewReader(b))
			if err != nil {
				return nil, fmt.Errorf("%w, unable to load config: %v", ErrRegistryAuth, err)
			}
			return af, nil
		}
	}
	logrus.Debug("using unauthenticated registry client")
	return &configfile.ConfigFile{}, nil
}

func AuthLookupFunc(containerConf *container.Config) func(ctx context.Context) (string, error) {
	return func(ctx context.Context) (string, error) {
		logrus.Debugf("beginning AuthFunc")
		rc := strings.Split(containerConf.Image, "/")
		registryName := rc[0]
		af, err := registryAuthFile(containerConf.Env)
		if err != nil {
			return "", err
		}

		authConf, err := af.GetAuthConfig(registryName)
		if err != nil {
			return "", err
		}
		// Majority of registries should be using BasicAuth
		if authConf.Username != "" && authConf.Password != "" {
			return base64.StdEncoding.EncodeToString(fmt.Appendf(nil, container_registry_auth_basic, authConf.Username, authConf.Password)), nil
		}
		if authConf.IdentityToken != "" {
			return base64.StdEncoding.EncodeToString(fmt.Appendf(nil, container_registry_auth_oidc, authConf.IdentityToken)), nil
		}
		return "", nil
	}
}
