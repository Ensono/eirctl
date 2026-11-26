package securehttp

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/tls"
	"crypto/x509"
	"errors"
	"math/big"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestTLSNegotiation(t *testing.T) {
	for _, tc := range []struct {
		name        string
		version     uint16
		cipher      uint16
		wantConnect bool
	}{
		{"TLS 1.3", tls.VersionTLS13, 0, true},
		{"TLS 1.2 ECDHE AEAD", tls.VersionTLS12, tls.TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256, true},
		{"TLS 1.2 CBC only", tls.VersionTLS12, tls.TLS_ECDHE_RSA_WITH_AES_128_CBC_SHA, false},
		{"TLS 1.2 static RSA only", tls.VersionTLS12, tls.TLS_RSA_WITH_AES_128_GCM_SHA256, false},
		{"TLS 1.1", tls.VersionTLS11, tls.TLS_ECDHE_RSA_WITH_AES_128_CBC_SHA, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			server := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(http.StatusNoContent)
			}))
			server.TLS = &tls.Config{MinVersion: tc.version, MaxVersion: tc.version}
			if tc.cipher != 0 {
				server.TLS.CipherSuites = []uint16{tc.cipher}
			}
			server.StartTLS()
			defer server.Close()
			root := x509.NewCertPool()
			root.AddCert(server.Certificate())
			resp, err := newClient(root).Get(server.URL)
			if err != nil {
				if tc.wantConnect {
					t.Fatal(err)
				}
				return
			}
			defer resp.Body.Close()
			if !tc.wantConnect || resp.TLS.Version != tc.version ||
				(tc.cipher != 0 && resp.TLS.CipherSuite != tc.cipher) {
				t.Fatalf("unexpected negotiated TLS version %#x, cipher %#x", resp.TLS.Version, resp.TLS.CipherSuite)
			}
		})
	}
}

func TestTLSRejectsWeakKeyInVerifiedChain(t *testing.T) {
	root, err := rsa.GenerateKey(rand.Reader, 1024)
	if err != nil {
		t.Fatal(err)
	}
	leaf, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	now := time.Now()
	caTemplate := &x509.Certificate{
		SerialNumber: big.NewInt(1), NotBefore: now.Add(-time.Hour), NotAfter: now.Add(time.Hour),
		IsCA: true, BasicConstraintsValid: true, KeyUsage: x509.KeyUsageCertSign,
	}
	caDER, err := x509.CreateCertificate(rand.Reader, caTemplate, caTemplate, &root.PublicKey, root)
	if err != nil {
		t.Fatal(err)
	}
	ca, err := x509.ParseCertificate(caDER)
	if err != nil {
		t.Fatal(err)
	}
	leafTemplate := &x509.Certificate{
		SerialNumber: big.NewInt(2), NotBefore: ca.NotBefore, NotAfter: ca.NotAfter,
		IPAddresses: []net.IP{net.IPv4(127, 0, 0, 1)}, KeyUsage: x509.KeyUsageDigitalSignature,
		ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
	}
	leafDER, err := x509.CreateCertificate(rand.Reader, leafTemplate, ca, &leaf.PublicKey, root)
	if err != nil {
		t.Fatal(err)
	}
	server := httptest.NewUnstartedServer(http.NotFoundHandler())
	server.TLS = &tls.Config{
		Certificates: []tls.Certificate{{Certificate: [][]byte{leafDER, caDER}, PrivateKey: leaf}},
	}
	server.StartTLS()
	defer server.Close()
	pool := x509.NewCertPool()
	pool.AddCert(ca)
	_, err = newClient(pool).Get(server.URL)
	if err == nil || !strings.Contains(err.Error(), "RSA keys of at least 2048 bits") {
		t.Fatalf("got %v, want minimum key length error", err)
	}
}

func TestRejectHTTPSRedirectToHTTP(t *testing.T) {
	target := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		t.Error("followed a downgrade redirect")
	}))
	defer target.Close()
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, target.URL, http.StatusFound)
	}))
	defer server.Close()
	pool := x509.NewCertPool()
	pool.AddCert(server.Certificate())
	if _, err := newClient(pool).Get(server.URL); !errors.Is(err, ErrHTTPSRequired) {
		t.Fatalf("got %v, want HTTPS requirement error", err)
	}
}
