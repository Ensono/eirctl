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
