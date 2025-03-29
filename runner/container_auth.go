package runner

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"os"
	"path"
	"strings"

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
