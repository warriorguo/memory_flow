package mfcli

import (
	"crypto/tls"
	"crypto/x509"
	"errors"
	"net/http"
	"os"
	"sync"
)

// Certificate verification normally goes through the operating system's trust
// store. That is not always reachable: inside a restricted sandbox, macOS trust
// evaluation fails outright, and every HTTPS request dies with an OSStatus
// error even though the server is presenting a perfectly valid chain.
//
// Rather than fail — or, worse, skip verification — mf retries once against a
// PEM bundle of root certificates. Verification still happens; only the source
// of the roots changes. If no bundle can be found, the original error stands.

// tlsFallback holds the bundle-based roots, installed at most once per process.
var tlsFallback struct {
	sync.Mutex
	pool    *x509.CertPool
	enabled bool
	tried   bool
}

// resetTLSFallback clears the process-wide fallback state. Tests only.
func resetTLSFallback() {
	tlsFallback.Lock()
	defer tlsFallback.Unlock()
	tlsFallback.pool, tlsFallback.enabled, tlsFallback.tried = nil, false, false
}

// newTransport builds a transport that honors the proxy environment and, once
// the fallback is enabled, verifies against the loaded bundle.
func newTransport() *http.Transport {
	tr := http.DefaultTransport.(*http.Transport).Clone()

	tlsFallback.Lock()
	defer tlsFallback.Unlock()
	if tlsFallback.enabled {
		tr.TLSClientConfig = &tls.Config{RootCAs: tlsFallback.pool}
	}
	return tr
}

// enableTLSFallback loads a CA bundle and reports whether requests can be
// retried against it. It is safe to call repeatedly: the bundle is loaded at
// most once, and a failure to find one is remembered so later calls stop
// trying.
//
// It keeps reporting true once a bundle is loaded, because a caller holding a
// transport built before the fallback existed still needs to rebuild and retry.
// Callers retry at most once, so this cannot loop.
func enableTLSFallback() bool {
	tlsFallback.Lock()
	defer tlsFallback.Unlock()

	if tlsFallback.enabled {
		return true
	}
	if tlsFallback.tried {
		return false
	}
	tlsFallback.tried = true

	for _, path := range caBundlePaths() {
		pem, err := os.ReadFile(path)
		if err != nil {
			continue
		}
		pool := x509.NewCertPool()
		if !pool.AppendCertsFromPEM(pem) {
			continue
		}
		tlsFallback.pool = pool
		tlsFallback.enabled = true
		return true
	}
	return false
}

// caBundlePaths lists PEM bundles to try, most authoritative first.
// SSL_CERT_FILE is the conventional override and wins outright; the rest are
// the standard locations on macOS and common Linux distributions.
func caBundlePaths() []string {
	if f := os.Getenv("SSL_CERT_FILE"); f != "" {
		return []string{f}
	}
	return []string{
		"/etc/ssl/cert.pem",                  // macOS, and some BSDs
		"/etc/ssl/certs/ca-certificates.crt", // Debian, Ubuntu, Alpine
		"/etc/pki/tls/certs/ca-bundle.crt",   // Fedora, RHEL
	}
}

// isTLSVerifyError reports whether err is a certificate verification failure,
// as opposed to a connection, timeout, or protocol error. Only verification
// failures are worth retrying with different roots.
func isTLSVerifyError(err error) bool {
	if err == nil {
		return false
	}
	var verifyErr *tls.CertificateVerificationError
	if errors.As(err, &verifyErr) {
		return true
	}
	// Predates the typed error, and covers wrapped equivalents.
	var unknownAuthority x509.UnknownAuthorityError
	return errors.As(err, &unknownAuthority)
}
