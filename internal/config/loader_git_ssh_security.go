package config

import (
	"crypto/ecdsa"
	"crypto/ed25519"
	"crypto/rsa"
	"fmt"

	gitssh "github.com/go-git/go-git/v5/plumbing/transport/ssh"
	"github.com/sirupsen/logrus"
	"golang.org/x/crypto/ssh"
)

const (
	minimumRSABits      = 2048
	minimumEllipticBits = 224
	ed25519SecurityBits = 256
	sshAlgorithmRSA     = "RSA"
	sshAlgorithmECDSA   = "ECDSA"
	sshAlgorithmEd25519 = "Ed25519"
	sshAlgorithmUnknown = "unclassifiable"
)

type sshKeyStrengthStatus int

const (
	sshKeyStrengthVerified sshKeyStrengthStatus = iota
	sshKeyStrengthUndersized
	sshKeyStrengthUnclassifiable
)

// sshKeyStrength contains only public algorithm and bit-length metadata that
// is safe to include in a diagnostic.
type sshKeyStrength struct {
	algorithm string
	bits      int
	minimum   int
	status    sshKeyStrengthStatus
}

// classifySSHPublicKey derives strength metadata from the public part of an
// already parsed SSH identity. Unknown key forms deliberately remain usable.
func classifySSHPublicKey(publicKey ssh.PublicKey) sshKeyStrength {
	cryptoPublicKey, ok := publicKey.(ssh.CryptoPublicKey)
	if !ok {
		return sshKeyStrength{algorithm: sshAlgorithmUnknown, status: sshKeyStrengthUnclassifiable}
	}

	switch key := cryptoPublicKey.CryptoPublicKey().(type) {
	case *rsa.PublicKey:
		return classifySSHKeyStrength(sshAlgorithmRSA, key.N.BitLen(), minimumRSABits)
	case *ecdsa.PublicKey:
		return classifySSHKeyStrength(sshAlgorithmECDSA, key.Curve.Params().BitSize, minimumEllipticBits)
	case ed25519.PublicKey:
		return classifySSHKeyStrength(sshAlgorithmEd25519, ed25519SecurityBits, minimumEllipticBits)
	default:
		return sshKeyStrength{algorithm: sshAlgorithmUnknown, status: sshKeyStrengthUnclassifiable}
	}
}

func classifySSHKeyStrength(algorithm string, bits, minimum int) sshKeyStrength {
	status := sshKeyStrengthVerified
	if bits < minimum {
		status = sshKeyStrengthUndersized
	}
	return sshKeyStrength{algorithm: algorithm, bits: bits, minimum: minimum, status: status}
}

func warnForSSHKeyStrength(publicKey ssh.PublicKey) {
	strength := classifySSHPublicKey(publicKey)
	switch strength.status {
	case sshKeyStrengthUndersized:
		logrus.Warnf("SSH identity strength warning: %s key is %d bits; recommended minimum is %d bits. Authentication will continue.", strength.algorithm, strength.bits, strength.minimum)
	case sshKeyStrengthUnclassifiable:
		logrus.Warn("SSH identity strength warning: identity key strength could not be verified. Authentication will continue.")
	}
}

// hardenedGitSSHAuth configures go-git's SSH transport without inheriting the
// compatibility defaults in golang.org/x/crypto/ssh.
type hardenedGitSSHAuth struct {
	user            string
	signer          ssh.Signer
	hostKeyCallback ssh.HostKeyCallback
}

func (a *hardenedGitSSHAuth) Name() string {
	return gitssh.PublicKeysName
}

func (a *hardenedGitSSHAuth) String() string {
	return fmt.Sprintf("user: %s, name: %s", a.user, a.Name())
}

func (a *hardenedGitSSHAuth) ClientConfig() (*ssh.ClientConfig, error) {
	signer, err := restrictSSHSigner(a.signer)
	if err != nil {
		return nil, err
	}
	algorithms := ssh.SupportedAlgorithms()
	return &ssh.ClientConfig{
		User: a.user,
		Auth: []ssh.AuthMethod{ssh.PublicKeys(signer)},
		Config: ssh.Config{
			KeyExchanges: algorithms.KeyExchanges,
			Ciphers:      algorithms.Ciphers,
			MACs:         algorithms.MACs,
		},
		HostKeyCallback:   a.hostKeyCallback,
		HostKeyAlgorithms: algorithms.HostKeys,
	}, nil
}

// restrictSSHSigner prevents RSA SHA-1 (ssh-rsa) and other legacy identity
// algorithms from being selected during public-key authentication.
func restrictSSHSigner(signer ssh.Signer) (ssh.Signer, error) {
	algorithmSigner, ok := signer.(ssh.AlgorithmSigner)
	if !ok {
		return nil, fmt.Errorf("SSH identity does not support explicit public-key authentication algorithms")
	}

	var algorithms []string
	switch signer.PublicKey().Type() {
	case ssh.KeyAlgoRSA:
		algorithms = []string{ssh.KeyAlgoRSASHA512, ssh.KeyAlgoRSASHA256}
	case ssh.KeyAlgoECDSA256, ssh.KeyAlgoECDSA384, ssh.KeyAlgoECDSA521, ssh.KeyAlgoED25519:
		algorithms = []string{signer.PublicKey().Type()}
	default:
		return nil, fmt.Errorf("SSH identity algorithm is not permitted by the Git transport policy")
	}
	return ssh.NewSignerWithAlgorithms(algorithmSigner, algorithms)
}
