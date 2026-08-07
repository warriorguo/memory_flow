package mfcli

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"errors"
	"math/big"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestIsTLSVerifyError(t *testing.T) {
	verifyErr := &tls.CertificateVerificationError{Err: errors.New("OSStatus -26276")}
	tests := []struct {
		name string
		err  error
		want bool
	}{
		{"nil", nil, false},
		{"verification failure", verifyErr, true},
		{"wrapped verification failure", &connError{&url.Error{Op: "Get", URL: "https://x", Err: verifyErr}}, true},
		{"unknown authority", x509.UnknownAuthorityError{}, true},
		{"connection refused", errors.New("connect: connection refused"), false},
		{"wrapped connection error", &connError{errors.New("i/o timeout")}, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := isTLSVerifyError(tt.err); got != tt.want {
				t.Errorf("isTLSVerifyError(%v) = %v, want %v", tt.err, got, tt.want)
			}
		})
	}
}

func TestCABundlePathsHonorsSSLCertFile(t *testing.T) {
	t.Setenv("SSL_CERT_FILE", "/custom/bundle.pem")
	paths := caBundlePaths()
	if len(paths) != 1 || paths[0] != "/custom/bundle.pem" {
		t.Errorf("caBundlePaths() = %v, want the SSL_CERT_FILE value alone", paths)
	}

	t.Setenv("SSL_CERT_FILE", "")
	if paths := caBundlePaths(); len(paths) < 2 {
		t.Errorf("caBundlePaths() = %v, want the standard locations", paths)
	}
}

func TestEnableTLSFallback(t *testing.T) {
	// A bundle that cannot be read or parsed leaves the fallback off, and the
	// failure is remembered so later calls stop trying.
	resetTLSFallback()
	t.Setenv("SSL_CERT_FILE", filepath.Join(t.TempDir(), "does-not-exist.pem"))
	if enableTLSFallback() {
		t.Error("a missing bundle should not enable the fallback")
	}
	if enableTLSFallback() {
		t.Error("a second call should still report unavailable")
	}

	resetTLSFallback()
	garbage := filepath.Join(t.TempDir(), "garbage.pem")
	if err := os.WriteFile(garbage, []byte("not a certificate"), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("SSL_CERT_FILE", garbage)
	if enableTLSFallback() {
		t.Error("an unparseable bundle should not enable the fallback")
	}
	resetTLSFallback()
}

// The end-to-end recovery: a server whose certificate the platform trust store
// does not know, with the bundle naming it. The first attempt must fail
// verification and the retry must succeed — proving the fallback still
// verifies rather than skipping the check.
func TestTLSFallbackRecoversWithBundle(t *testing.T) {
	resetTLSFallback()
	defer resetTLSFallback()
	isolateHome(t)

	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"data":[],"total":0,"page":1,"page_size":1}`))
	}))
	defer srv.Close()

	// Without the bundle, verification fails: this server's root is unknown.
	c := NewClient(srv.URL, 5*time.Second)
	if _, err := c.get("/api/v1/projects", nil); err == nil {
		t.Fatal("an untrusted certificate should fail verification")
	} else if !isTLSVerifyError(err) {
		t.Fatalf("expected a verification error, got %v", err)
	}

	// Point SSL_CERT_FILE at the server's own certificate and retry.
	bundle := filepath.Join(t.TempDir(), "bundle.pem")
	encoded := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: srv.Certificate().Raw})
	if err := os.WriteFile(bundle, encoded, 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("SSL_CERT_FILE", bundle)

	resetTLSFallback()
	c2 := NewClient(srv.URL, 5*time.Second)
	if _, err := c2.get("/api/v1/projects", nil); err != nil {
		t.Fatalf("the fallback should have verified against the bundle: %v", err)
	}
}

// A client created before the fallback was installed must still recover — the
// case that broke when the fallback was first written.
func TestTLSFallbackRefreshesExistingClient(t *testing.T) {
	resetTLSFallback()
	defer resetTLSFallback()
	isolateHome(t)

	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"data":[],"total":0,"page":1,"page_size":1}`))
	}))
	defer srv.Close()

	bundle := filepath.Join(t.TempDir(), "bundle.pem")
	encoded := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: srv.Certificate().Raw})
	if err := os.WriteFile(bundle, encoded, 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("SSL_CERT_FILE", bundle)

	// Build the client first, then install the fallback behind its back, the
	// way probing during resolution does.
	c := NewClient(srv.URL, 5*time.Second)
	if !enableTLSFallback() {
		t.Fatal("the bundle should have loaded")
	}
	if _, err := c.get("/api/v1/projects", nil); err != nil {
		t.Fatalf("a client built before the fallback should still recover: %v", err)
	}
}

// Resolution probes over HTTPS, so the fallback has to work there too — this is
// the path `mf ctx` takes before any command runs.
func TestProbeUsesTLSFallback(t *testing.T) {
	resetTLSFallback()
	defer resetTLSFallback()
	isolateHome(t)

	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"data":[],"total":0,"page":1,"page_size":1}`))
	}))
	defer srv.Close()

	if probe(srv.URL) {
		t.Fatal("an untrusted certificate should not probe successfully")
	}

	bundle := filepath.Join(t.TempDir(), "bundle.pem")
	encoded := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: srv.Certificate().Raw})
	if err := os.WriteFile(bundle, encoded, 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("SSL_CERT_FILE", bundle)

	resetTLSFallback()
	if !probe(srv.URL) {
		t.Error("the probe should recover via the bundle")
	}

	// And the whole resolution chain then picks it as the remote.
	resetTLSFallback()
	t.Setenv("MEMORY_FLOW_REMOTE", srv.URL)
	var stdout, stderr strings.Builder
	if code := Main([]string{"projects"}, &stdout, &stderr); code != 0 {
		t.Errorf("resolution over HTTPS should succeed via the bundle: %s", stderr.String())
	}
}

// The fallback must never weaken verification: a certificate absent from the
// bundle still has to be rejected. The bundle here holds an unrelated CA, so
// the server's certificate cannot chain to it.
//
// (Every httptest TLS server shares one built-in certificate, so the unrelated
// CA has to be generated rather than taken from a second server.)
func TestTLSFallbackStillRejectsUntrusted(t *testing.T) {
	resetTLSFallback()
	defer resetTLSFallback()
	isolateHome(t)

	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	defer srv.Close()

	bundle := filepath.Join(t.TempDir(), "bundle.pem")
	if err := os.WriteFile(bundle, unrelatedCAPEM(t), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("SSL_CERT_FILE", bundle)

	c := NewClient(srv.URL, 5*time.Second)
	if _, err := c.get("/api/v1/projects", nil); err == nil {
		t.Error("a certificate that does not chain to the bundle must still be rejected")
	}
}

// unrelatedCAPEM returns a freshly generated, self-signed CA certificate that
// has signed nothing.
func unrelatedCAPEM(t *testing.T) []byte {
	t.Helper()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	template := &x509.Certificate{
		SerialNumber:          big.NewInt(1),
		Subject:               pkix.Name{CommonName: "Unrelated Test CA"},
		NotBefore:             time.Now().Add(-time.Hour),
		NotAfter:              time.Now().Add(time.Hour),
		IsCA:                  true,
		KeyUsage:              x509.KeyUsageCertSign,
		BasicConstraintsValid: true,
	}
	der, err := x509.CreateCertificate(rand.Reader, template, template, &key.PublicKey, key)
	if err != nil {
		t.Fatal(err)
	}
	return pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der})
}
