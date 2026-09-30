package config

import (
	"bytes"
	"crypto/ecdsa"
	"crypto/ed25519"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/rsa"
	"strings"
	"testing"

	"github.com/sirupsen/logrus"
	"golang.org/x/crypto/ssh"
)

type unclassifiableSSHPublicKey struct{}

func (unclassifiableSSHPublicKey) Type() string    { return "unclassifiable-test-key" }
func (unclassifiableSSHPublicKey) Marshal() []byte { return nil }
func (unclassifiableSSHPublicKey) Verify([]byte, *ssh.Signature) error {
	return nil
}

func TestClassifySSHPublicKey(t *testing.T) {
	rsaKey, err := rsa.GenerateKey(rand.Reader, 1024)
	if err != nil {
		t.Fatalf("generate RSA key: %v", err)
	}
	ecdsaKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatalf("generate ECDSA key: %v", err)
	}
	_, ed25519Key, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatalf("generate Ed25519 key: %v", err)
	}

	tests := []struct {
		name              string
		key               ssh.PublicKey
		wantAlgorithm     string
		wantStatus        sshKeyStrengthStatus
		wantBits, wantMin int
	}{
		{
			name:          "undersized RSA",
			key:           mustSSHPublicKey(t, &rsaKey.PublicKey),
			wantAlgorithm: sshAlgorithmRSA,
			wantStatus:    sshKeyStrengthUndersized,
			wantBits:      1024,
			wantMin:       minimumRSABits,
		},
		{
			name:          "secure ECDSA",
			key:           mustSSHPublicKey(t, &ecdsaKey.PublicKey),
			wantAlgorithm: sshAlgorithmECDSA,
			wantStatus:    sshKeyStrengthVerified,
			wantBits:      256,
			wantMin:       minimumEllipticBits,
		},
		{
			name:          "secure Ed25519",
			key:           mustSSHPublicKey(t, ed25519Key.Public()),
			wantAlgorithm: sshAlgorithmEd25519,
			wantStatus:    sshKeyStrengthVerified,
			wantBits:      ed25519SecurityBits,
			wantMin:       minimumEllipticBits,
		},
		{
			name:          "unclassifiable key",
			key:           unclassifiableSSHPublicKey{},
			wantAlgorithm: sshAlgorithmUnknown,
			wantStatus:    sshKeyStrengthUnclassifiable,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got := classifySSHPublicKey(test.key)
			if got.algorithm != test.wantAlgorithm || got.status != test.wantStatus || got.bits != test.wantBits || got.minimum != test.wantMin {
				t.Fatalf("classifySSHPublicKey() = %+v, want algorithm=%q status=%v bits=%d minimum=%d", got, test.wantAlgorithm, test.wantStatus, test.wantBits, test.wantMin)
			}
		})
	}
}

func TestSSHKeyStrengthWarningsContainOnlySafeMetadata(t *testing.T) {
	rsaKey, err := rsa.GenerateKey(rand.Reader, 1024)
	if err != nil {
		t.Fatalf("generate RSA key: %v", err)
	}

	var logs bytes.Buffer
	previous := logrus.StandardLogger().Out
	logrus.SetOutput(&logs)
	t.Cleanup(func() { logrus.SetOutput(previous) })

	warnForSSHKeyStrength(mustSSHPublicKey(t, &rsaKey.PublicKey))
	warnForSSHKeyStrength(unclassifiableSSHPublicKey{})

	output := logs.String()
	if !strings.Contains(output, "RSA key is 1024 bits") || !strings.Contains(output, "could not be verified") {
		t.Fatalf("missing expected safe key-strength warnings: %q", output)
	}
	for _, secret := range []string{"private key", "passphrase", "token", "https://repo.example.test", "/home/test/.ssh/id_rsa"} {
		if strings.Contains(output, secret) {
			t.Fatalf("warning disclosed sensitive data %q: %q", secret, output)
		}
	}
}

func TestHardenedGitSSHAuthUsesSupportedAlgorithms(t *testing.T) {
	_, privateKey, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatalf("generate Ed25519 key: %v", err)
	}
	signer, err := ssh.NewSignerFromKey(privateKey)
	if err != nil {
		t.Fatalf("create SSH signer: %v", err)
	}

	auth := &hardenedGitSSHAuth{
		user:            "git",
		signer:          signer,
		hostKeyCallback: ssh.InsecureIgnoreHostKey(),
	}
	config, err := auth.ClientConfig()
	if err != nil {
		t.Fatalf("ClientConfig: %v", err)
	}
	if config.User != "git" || config.HostKeyCallback == nil || len(config.Auth) != 1 {
		t.Fatalf("ClientConfig did not preserve selected user, callback, and signer: %+v", config)
	}
	assertNoInsecureAlgorithms(t, "key exchange", config.KeyExchanges, ssh.InsecureAlgorithms().KeyExchanges)
	assertNoInsecureAlgorithms(t, "cipher", config.Ciphers, ssh.InsecureAlgorithms().Ciphers)
	assertNoInsecureAlgorithms(t, "MAC", config.MACs, ssh.InsecureAlgorithms().MACs)
	assertNoInsecureAlgorithms(t, "host key", config.HostKeyAlgorithms, ssh.InsecureAlgorithms().HostKeys)
}

func TestRestrictSSHSignerExcludesSSHRSA(t *testing.T) {
	privateKey, err := rsa.GenerateKey(rand.Reader, minimumRSABits)
	if err != nil {
		t.Fatalf("generate RSA key: %v", err)
	}
	signer, err := ssh.NewSignerFromKey(privateKey)
	if err != nil {
		t.Fatalf("create SSH signer: %v", err)
	}

	restricted, err := restrictSSHSigner(signer)
	if err != nil {
		t.Fatalf("restrictSSHSigner: %v", err)
	}
	multiAlgorithmSigner, ok := restricted.(ssh.MultiAlgorithmSigner)
	if !ok {
		t.Fatalf("restricted signer does not report its allowed algorithms: %T", restricted)
	}
	for _, algorithm := range multiAlgorithmSigner.Algorithms() {
		if algorithm == ssh.KeyAlgoRSA || algorithm == ssh.InsecureKeyAlgoDSA {
			t.Fatalf("legacy public-key algorithm remained enabled: %q", algorithm)
		}
	}
}

func mustSSHPublicKey(t *testing.T, key any) ssh.PublicKey {
	t.Helper()
	publicKey, err := ssh.NewPublicKey(key)
	if err != nil {
		t.Fatalf("create SSH public key: %v", err)
	}
	return publicKey
}

func assertNoInsecureAlgorithms(t *testing.T, category string, configured, insecure []string) {
	t.Helper()
	for _, configuredAlgorithm := range configured {
		for _, insecureAlgorithm := range insecure {
			if configuredAlgorithm == insecureAlgorithm {
				t.Fatalf("legacy %s algorithm remained enabled: %q", category, configuredAlgorithm)
			}
		}
	}
}
