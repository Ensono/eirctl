package runner

import (
	"bytes"
	"context"
	"encoding/base64"
	"fmt"
	"os"
	"path"
	"strings"

	"github.com/docker/cli/cli/config"
	"github.com/docker/cli/cli/config/configfile"
	"github.com/sirupsen/logrus"
)

const (
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

func registryAuthFile() (*configfile.ConfigFile, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, err
	}

	defaultPaths := []string{path.Join(home, DOCKER_CONFIG_FILE), path.Join(home, CONTAINER_CONFIG_FILE)}

	if authFile, found := os.LookupEnv(REGISTRY_AUTH_FILE); found {
		// If REGISTRY_AUTH_FILE has been supplied - check it first
		defaultPaths = append([]string{authFile}, defaultPaths...)
	}

	for _, authFile := range defaultPaths {
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
	return nil, fmt.Errorf("%w, no auth file found", ErrRegistryAuth)
}

func AuthLookupFunc(name string) func(ctx context.Context) (string, error) {
	return func(ctx context.Context) (string, error) {
		rc := strings.Split(name, "/")
		registryName := rc[0]
		af, err := registryAuthFile()
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
