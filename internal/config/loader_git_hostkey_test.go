package config

import (
	"bytes"
	"crypto/ed25519"
	"crypto/rand"
	"errors"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/kevinburke/ssh_config"
	"github.com/sirupsen/logrus"
	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/knownhosts"
)

func testHostKey(t *testing.T) ssh.PublicKey {
	t.Helper()
	_, privateKey, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatalf("generate host key: %v", err)
	}
	signer, err := ssh.NewSignerFromKey(privateKey)
	if err != nil {
		t.Fatalf("create host signer: %v", err)
	}
	return signer.PublicKey()
}

func writeKnownHosts(t *testing.T, host string, key ssh.PublicKey) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "known_hosts")
	if err := os.WriteFile(path, []byte(fmt.Sprintf("%s %s\n", host, strings.TrimSpace(string(ssh.MarshalAuthorizedKey(key))))), 0600); err != nil {
		t.Fatalf("write known_hosts: %v", err)
	}
	return path
}

func TestHostKeyCallback(t *testing.T) {
	trustedKey := testHostKey(t)
	otherKey := testHostKey(t)
	knownHosts := writeKnownHosts(t, "[git.example.test]:2222", trustedKey)

	callback, err := hostKeyCallback(&SSHConfigAuth{Hostname: "git.example.test", Port: "2222", UserKnownHostsFile: knownHosts})
	if err != nil {
		t.Fatalf("hostKeyCallback: %v", err)
	}

	address := &net.TCPAddr{IP: net.ParseIP("192.0.2.10"), Port: 2222}
	if err := callback("git.example.test:2222", address, trustedKey); err != nil {
		t.Fatalf("trusted host key rejected: %v", err)
	}
	unknownErr := callback("unknown.example.test:2222", address, trustedKey)
	if unknownErr == nil {
		t.Fatal("unknown host key accepted")
	}
	if !strings.Contains(unknownErr.Error(), "SSH host-key verification failed for git.example.test:2222") || !strings.Contains(unknownErr.Error(), "known-hosts file") {
		t.Fatalf("unknown-host error lacks actionable effective endpoint context: %v", unknownErr)
	}
	var unknownKeyErr *knownhosts.KeyError
	if !errors.As(unknownErr, &unknownKeyErr) {
		t.Fatalf("unknown-host error did not retain the underlying knownhosts error: %v", unknownErr)
	}
	changedErr := callback("git.example.test:2222", address, otherKey)
	if changedErr == nil {
		t.Fatal("changed host key accepted")
	}
	if !strings.Contains(changedErr.Error(), "SSH host-key verification failed for git.example.test:2222") || !strings.Contains(changedErr.Error(), "known-hosts file") {
		t.Fatalf("changed-key error lacks actionable effective endpoint context: %v", changedErr)
	}
	var changedKeyErr *knownhosts.KeyError
	if !errors.As(changedErr, &changedKeyErr) {
		t.Fatalf("changed-key error did not retain the underlying knownhosts error: %v", changedErr)
	}
}

func TestHostKeyCallbackSupportsHashedKnownHost(t *testing.T) {
	trustedKey := testHostKey(t)
	knownHosts := writeKnownHosts(t, knownhosts.HashHostname("git.example.test"), trustedKey)
	callback, err := hostKeyCallback(&SSHConfigAuth{Hostname: "git.example.test", UserKnownHostsFile: knownHosts})
	if err != nil {
		t.Fatalf("hostKeyCallback: %v", err)
	}
	if err := callback("git.example.test:22", &net.TCPAddr{IP: net.ParseIP("192.0.2.10"), Port: 22}, trustedKey); err != nil {
		t.Fatalf("hashed known-host entry rejected: %v", err)
	}
}

func TestKnownHostsFilesPrefersConfiguredUserFile(t *testing.T) {
	configured := filepath.Join(t.TempDir(), "custom_known_hosts")
	if err := os.WriteFile(configured, []byte("# test\n"), 0600); err != nil {
		t.Fatalf("write configured known_hosts: %v", err)
	}
	paths := knownHostsFiles(&SSHConfigAuth{UserKnownHostsFile: configured})
	if len(paths) == 0 || paths[0] != configured {
		t.Fatalf("configured known-host file was not selected first: %v", paths)
	}
}

func TestKnownHostsFilesUsesDefaultUserFile(t *testing.T) {
	home := t.TempDir()
	sshDir := filepath.Join(home, ".ssh")
	if err := os.Mkdir(sshDir, 0700); err != nil {
		t.Fatalf("create .ssh directory: %v", err)
	}
	defaultFile := filepath.Join(sshDir, "known_hosts")
	if err := os.WriteFile(defaultFile, []byte("# test\n"), 0600); err != nil {
		t.Fatalf("write default known_hosts: %v", err)
	}
	t.Setenv("HOME", home)
	paths := knownHostsFiles(&SSHConfigAuth{})
	if len(paths) == 0 || paths[0] != defaultFile {
		t.Fatalf("default known-host file was not selected first: %v", paths)
	}
}

func TestHostKeyCallbackFailsWithoutTrustSource(t *testing.T) {
	_, err := hostKeyCallback(&SSHConfigAuth{Hostname: "git.example.test", UserKnownHostsFile: filepath.Join(t.TempDir(), "missing")})
	if err == nil || !strings.Contains(err.Error(), "configured UserKnownHostsFile") {
		t.Fatalf("expected configured missing known-hosts error, got %v", err)
	}
}

func TestHostKeyCallbackAllowsExplicitInsecureOptOutAndWarns(t *testing.T) {
	var logs bytes.Buffer
	previous := logrus.StandardLogger().Out
	logrus.SetOutput(&logs)
	t.Cleanup(func() { logrus.SetOutput(previous) })

	callback, err := hostKeyCallback(&SSHConfigAuth{Hostname: "git.example.test", StrictHostKeyChecking: "no"})
	if err != nil {
		t.Fatalf("hostKeyCallback: %v", err)
	}
	if err := callback("unknown.example.test:22", &net.TCPAddr{IP: net.ParseIP("192.0.2.10"), Port: 22}, testHostKey(t)); err != nil {
		t.Fatalf("explicit insecure opt-out rejected a host: %v", err)
	}
	if !strings.Contains(logs.String(), "host-key verification is disabled") {
		t.Fatalf("expected insecure host-key warning, got %q", logs.String())
	}
}

func TestSSHTrustOptionsPropagateFromConfigAndCommand(t *testing.T) {
	configured := filepath.Join(t.TempDir(), "known_hosts")
	sshFile, err := ssh_config.Decode(strings.NewReader(fmt.Sprintf(`Host alias
  Hostname effective.example.test
  Port 2200
  UserKnownHostsFile %s
  StrictHostKeyChecking yes
`, configured)))
	if err != nil {
		t.Fatalf("decode SSH config: %v", err)
	}
	t.Setenv(GitSshCommandVar, fmt.Sprintf("ssh -o UserKnownHostsFile=%s -o StrictHostKeyChecking=no -o Port=2222", configured))
	command := parseGitSshCommandEnv()
	if err := processSSHConfig(sshFile, command, "alias"); err != nil {
		t.Fatalf("process SSH config: %v", err)
	}
	if command.Hostname != "effective.example.test" || command.Port != "2222" || command.UserKnownHostsFile != configured || command.StrictHostKeyChecking != "no" {
		t.Fatalf("unexpected effective SSH config: %+v", command)
	}
}

func TestGitSSHCommandPreservesRepeatedQuotedKnownHostsOptions(t *testing.T) {
	t.Setenv(GitSshCommandVar, `ssh -o UserKnownHostsFile="/tmp/first known_hosts" -o UserKnownHostsFile="/tmp/second known_hosts"`)
	config := parseGitSshCommandEnv()
	want := []string{"/tmp/first known_hosts", "/tmp/second known_hosts"}
	if !reflect.DeepEqual(config.UserKnownHostsFiles, want) {
		t.Fatalf("UserKnownHostsFiles = %q, want %q", config.UserKnownHostsFiles, want)
	}
}

func TestGitSSHCommandPreservesGlobalKnownHostsOptions(t *testing.T) {
	t.Setenv(GitSshCommandVar, `ssh -o GlobalKnownHostsFile="/tmp/first global" -oGlobalKnownHostsFile="/tmp/second global"`)
	config := parseGitSshCommandEnv()
	want := []string{"/tmp/first global", "/tmp/second global"}
	if !reflect.DeepEqual(config.SystemKnownHostsFiles, want) {
		t.Fatalf("SystemKnownHostsFiles = %q, want %q", config.SystemKnownHostsFiles, want)
	}
	if config.SystemKnownHostsFile != want[0] {
		t.Fatalf("SystemKnownHostsFile = %q, want %q", config.SystemKnownHostsFile, want[0])
	}

	fileConfig, err := ssh_config.Decode(strings.NewReader("Host alias\n  GlobalKnownHostsFile /file/global\n"))
	if err != nil {
		t.Fatalf("decode SSH config: %v", err)
	}
	if err := processSSHConfig(fileConfig, config, "alias"); err != nil {
		t.Fatalf("process SSH config: %v", err)
	}
	if !reflect.DeepEqual(config.SystemKnownHostsFiles, want) || config.SystemKnownHostsFile != want[0] {
		t.Fatalf("file config overrode command global known-hosts values: %+v", config)
	}
}

func TestKnownHostsFilesPreservesQuotedAndMultiplePaths(t *testing.T) {
	directory := t.TempDir()
	first := filepath.Join(directory, "first known hosts")
	second := filepath.Join(directory, "second known hosts")
	for _, path := range []string{first, second} {
		if err := os.WriteFile(path, []byte("# test\n"), 0600); err != nil {
			t.Fatal(err)
		}
	}
	paths := knownHostsFiles(&SSHConfigAuth{UserKnownHostsFile: fmt.Sprintf("%q %q", first, second)})
	if len(paths) < 2 || paths[0] != first || paths[1] != second {
		t.Fatalf("quoted known-host paths were not preserved: %v", paths)
	}
}

func TestKnownHostsFilesUsesInjectablePlatformDefaults(t *testing.T) {
	path := filepath.Join(t.TempDir(), "ssh_known_hosts")
	if err := os.WriteFile(path, []byte("# test\n"), 0600); err != nil {
		t.Fatal(err)
	}
	previous := platformDefaultKnownHostsFiles
	platformDefaultKnownHostsFiles = func() []string { return []string{path} }
	t.Cleanup(func() { platformDefaultKnownHostsFiles = previous })
	paths := knownHostsFiles(&SSHConfigAuth{})
	if len(paths) == 0 || paths[len(paths)-1] != path {
		t.Fatalf("injected platform known-host default was not selected: %v", paths)
	}
}

func TestConfiguredKnownHostsFailureDoesNotFallBackToDefaults(t *testing.T) {
	missing := filepath.Join(t.TempDir(), "missing-known-hosts")
	fallback := filepath.Join(t.TempDir(), "fallback-known-hosts")
	if err := os.WriteFile(fallback, []byte("# fallback\n"), 0600); err != nil {
		t.Fatalf("write fallback known_hosts: %v", err)
	}
	_, err := hostKeyCallback(&SSHConfigAuth{Hostname: "git.example.test", UserKnownHostsFile: missing, SystemKnownHostsFile: fallback})
	if err == nil || !strings.Contains(err.Error(), missing) {
		t.Fatalf("expected explicit trust-source failure, got %v", err)
	}
}

func TestProcessSSHConfigPreservesIncludedKnownHostsPaths(t *testing.T) {
	directory := t.TempDir()
	userPaths := []string{
		filepath.Join(directory, "quoted user"),
		filepath.Join(directory, "escaped user"),
		filepath.Join(directory, "user-three"),
		filepath.Join(directory, "user-four"),
		filepath.Join(directory, "mixed quoted user"),
		filepath.Join(directory, "user-six"),
		filepath.Join(directory, "user-seven"),
		filepath.Join(directory, "middle quoted user"),
		filepath.Join(directory, "user-nine"),
	}
	globalPaths := []string{
		filepath.Join(directory, "quoted global"),
		filepath.Join(directory, "escaped global"),
		filepath.Join(directory, "global-three"),
		filepath.Join(directory, "global-four"),
	}
	for _, path := range append(append([]string{}, userPaths...), globalPaths...) {
		if err := os.WriteFile(path, []byte("# test\n"), 0600); err != nil {
			t.Fatalf("write known-hosts fixture: %v", err)
		}
	}

	includedPath := filepath.Join(directory, "included-ssh-config")
	includedConfig := fmt.Sprintf(`Host alias
  UserKnownHostsFile %q
  UserKnownHostsFile %s
  UserKnownHostsFile %s %s
  UserKnownHostsFile %q %s
  UserKnownHostsFile %s %q %s
  GlobalKnownHostsFile %q
  GlobalKnownHostsFile %s
  GlobalKnownHostsFile %s %s
`, userPaths[0], strings.ReplaceAll(userPaths[1], " ", `\ `), userPaths[2], userPaths[3], userPaths[4], userPaths[5], userPaths[6], userPaths[7], userPaths[8], globalPaths[0], strings.ReplaceAll(globalPaths[1], " ", `\ `), globalPaths[2], globalPaths[3])
	if err := os.WriteFile(includedPath, []byte(includedConfig), 0600); err != nil {
		t.Fatalf("write included SSH config: %v", err)
	}
	fileConfig, err := ssh_config.Decode(strings.NewReader("Include " + includedPath + "\n"))
	if err != nil {
		t.Fatalf("decode SSH config: %v", err)
	}

	config := &SSHConfigAuth{}
	if err := processSSHConfig(fileConfig, config, "alias"); err != nil {
		t.Fatalf("process SSH config: %v", err)
	}
	if !reflect.DeepEqual(config.UserKnownHostsFiles, userPaths) || config.UserKnownHostsFile != userPaths[0] {
		t.Fatalf("unexpected included user known-hosts config: %+v", config)
	}
	if !reflect.DeepEqual(config.SystemKnownHostsFiles, globalPaths) || config.SystemKnownHostsFile != globalPaths[0] {
		t.Fatalf("unexpected included global known-hosts config: %+v", config)
	}
}

func TestSplitIncludedSSHConfigPathsPreservesPlatformPaths(t *testing.T) {
	tests := map[string]struct {
		value string
		want  []string
	}{
		"Windows drive path": {
			value: `C:\ProgramData\ssh\known_hosts`,
			want:  []string{`C:\ProgramData\ssh\known_hosts`},
		},
		"UNC path": {
			value: `\\server\share\known_hosts`,
			want:  []string{`\\server\share\known_hosts`},
		},
		"quoted Windows and UNC paths": {
			value: `"C:\ProgramData\ssh\known hosts" \\server\share\known_hosts`,
			want:  []string{`C:\ProgramData\ssh\known hosts`, `\\server\share\known_hosts`},
		},
	}
	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			got, err := splitIncludedSSHConfigPaths(test.value)
			if err != nil {
				t.Fatalf("split included SSH paths: %v", err)
			}
			if !reflect.DeepEqual(got, test.want) {
				t.Fatalf("splitIncludedSSHConfigPaths(%q) = %q, want %q", test.value, got, test.want)
			}
		})
	}
}

func sshMergeTestFileConfig(t *testing.T) *ssh_config.Config {
	t.Helper()
	fileConfig, err := ssh_config.Decode(strings.NewReader(`Host alias
  Hostname file.example.test
  Port 2200
  User file-user
  IdentityFile /file/identity
  UserKnownHostsFile "/file/user one" /file/user\ two
  UserKnownHostsFile /file/user-three
  GlobalKnownHostsFile "C:\ProgramData\ssh\system one" /file/system\ two
  GlobalKnownHostsFile /file/system-three
  GlobalKnownHostsFile \\server\share\known_hosts
  StrictHostKeyChecking yes
`))
	if err != nil {
		t.Fatalf("decode SSH config: %v", err)
	}
	return fileConfig
}

func TestProcessSSHConfigUsesFileValuesAndLegacyFirstEntries(t *testing.T) {
	config := &SSHConfigAuth{}
	if err := processSSHConfig(sshMergeTestFileConfig(t), config, "alias"); err != nil {
		t.Fatalf("process SSH config: %v", err)
	}
	if config.Port != "2200" || config.User != "file-user" || config.Hostname != "file.example.test" || config.IdentityFile != "/file/identity" || config.StrictHostKeyChecking != "yes" {
		t.Fatalf("unexpected file-derived scalar config: %+v", config)
	}
	if want := []string{"/file/user one", "/file/user two", "/file/user-three"}; !reflect.DeepEqual(config.UserKnownHostsFiles, want) || config.UserKnownHostsFile != want[0] {
		t.Fatalf("unexpected user known-hosts config: %+v", config)
	}
	if want := []string{`C:\ProgramData\ssh\system one`, "/file/system two", "/file/system-three", `\\server\share\known_hosts`}; !reflect.DeepEqual(config.SystemKnownHostsFiles, want) || config.SystemKnownHostsFile != want[0] {
		t.Fatalf("unexpected system known-hosts config: %+v", config)
	}
}

func TestProcessSSHConfigUsesDefaults(t *testing.T) {
	config := &SSHConfigAuth{}
	if err := processSSHConfig(&ssh_config.Config{}, config, "requested.example.test"); err != nil {
		t.Fatalf("process empty SSH config: %v", err)
	}
	if config.Port != "22" || config.User != "git" || config.Hostname != "requested.example.test" {
		t.Fatalf("unexpected default config: %+v", config)
	}
}

func TestProcessSSHConfigPreservesCommandPrecedence(t *testing.T) {
	config := &SSHConfigAuth{
		Hostname:              "command.example.test",
		Port:                  "2222",
		User:                  "command-user",
		IdentityFile:          "/command/identity",
		UserKnownHostsFile:    "/command/user",
		UserKnownHostsFiles:   []string{"/command/user", "/command/user-two"},
		SystemKnownHostsFile:  "/command/system",
		SystemKnownHostsFiles: []string{"/command/system", "/command/system-two"},
		StrictHostKeyChecking: "no",
	}
	if err := processSSHConfig(sshMergeTestFileConfig(t), config, "alias"); err != nil {
		t.Fatalf("process SSH config: %v", err)
	}
	if config.Hostname != "command.example.test" || config.Port != "2222" || config.User != "command-user" || config.IdentityFile != "/command/identity" || config.StrictHostKeyChecking != "no" {
		t.Fatalf("file config overrode command scalar values: %+v", config)
	}
	if want := []string{"/command/user", "/command/user-two"}; !reflect.DeepEqual(config.UserKnownHostsFiles, want) || config.UserKnownHostsFile != want[0] {
		t.Fatalf("file config overrode command user known-hosts values: %+v", config)
	}
	if want := []string{"/command/system", "/command/system-two"}; !reflect.DeepEqual(config.SystemKnownHostsFiles, want) || config.SystemKnownHostsFile != want[0] {
		t.Fatalf("file config overrode command system known-hosts values: %+v", config)
	}
}
