// Package securehttp provides the TLS policy shared by network clients.
package securehttp

import (
	"crypto/ecdsa"
	"crypto/ed25519"
	"crypto/mldsa"
	"crypto/rsa"
	"crypto/tls"
	"crypto/x509"
	"errors"
	"net/http"
	"net/url"
)

var ErrHTTPSRequired = errors.New("remote source requires HTTPS")

func RequireHTTPS(u *url.URL) error {
	if u.Scheme != "https" || u.Hostname() == "" {
		return ErrHTTPSRequired
	}
	return nil
}

var defaultTransport = http.DefaultTransport.(*http.Transport)

// NewClient retains the standard proxy defaults and any explicitly configured
// trust roots, but does not inherit settings that weaken TLS verification.
func NewClient() *http.Client {
	var roots *x509.CertPool
	if transport, ok := http.DefaultTransport.(*http.Transport); ok && transport.TLSClientConfig != nil {
		roots = transport.TLSClientConfig.RootCAs
	}
	return newClient(roots)
}

func newClient(roots *x509.CertPool) *http.Client {
	transport := defaultTransport.Clone()
	transport.TLSClientConfig = ClientTLS(roots)
	return &http.Client{
		Transport: transport,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if len(via) >= 10 {
				return errors.New("stopped after 10 redirects")
			}
			return RequireHTTPS(req.URL)
		},
	}
}

func ClientTLS(roots *x509.CertPool) *tls.Config {
	return &tls.Config{
		RootCAs:    roots,
		MinVersion: tls.VersionTLS12,
		CipherSuites: []uint16{
			tls.TLS_ECDHE_ECDSA_WITH_AES_128_GCM_SHA256,
			tls.TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256,
			tls.TLS_ECDHE_ECDSA_WITH_AES_256_GCM_SHA384,
			tls.TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384,
			tls.TLS_ECDHE_ECDSA_WITH_CHACHA20_POLY1305_SHA256,
			tls.TLS_ECDHE_RSA_WITH_CHACHA20_POLY1305_SHA256,
		},
		VerifyConnection: verifyChainKeys,
	}
}

func ValidateClientCertificates(certificates []tls.Certificate) error {
	for _, certificate := range certificates {
		for _, raw := range certificate.Certificate {
			cert, err := x509.ParseCertificate(raw)
			if err != nil {
				return err
			}
			if !acceptableCertificateKey(cert) {
				return errors.New("TLS client certificates require supported key types and RSA keys of at least 2048 bits")
			}
		}
	}
	return nil
}

func verifyChainKeys(state tls.ConnectionState) error {
	for _, chain := range state.VerifiedChains {
		acceptable := true
		for _, cert := range chain {
			if !acceptableCertificateKey(cert) {
				acceptable = false
				break
			}
		}
		if acceptable && len(chain) > 0 {
			return nil
		}
	}
	return errors.New("TLS requires a verified certificate chain with supported key types and RSA keys of at least 2048 bits")
}

func acceptableCertificateKey(cert *x509.Certificate) bool {
	switch key := cert.PublicKey.(type) {
	case *rsa.PublicKey:
		return key.N.BitLen() >= 2048
	case *ecdsa.PublicKey:
		return key.Curve.Params().BitSize >= 224
	case ed25519.PublicKey, *mldsa.PublicKey:
		return true
	default:
		return false
	}
}
