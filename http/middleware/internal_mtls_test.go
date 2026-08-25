package middleware

import (
	"crypto/ed25519"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"math/big"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/q1ngyang/rustdesk-api-kessoku/v3/config"
)

func TestInternalMTLSRequiresVerifiedAllowedSAN(t *testing.T) {
	allowed, _ := url.Parse("spiffe://example.com/starry/production")
	denied, _ := url.Parse("spiffe://example.com/other")
	now := time.Now()

	tests := []struct {
		name       string
		state      *tls.ConnectionState
		allowedSAN []string
		wantStatus int
	}{
		{name: "missing TLS", wantStatus: http.StatusForbidden},
		{
			name: "unverified certificate",
			state: &tls.ConnectionState{PeerCertificates: []*x509.Certificate{{
				Raw: []byte("leaf"), URIs: []*url.URL{allowed}, NotBefore: now.Add(-time.Minute), NotAfter: now.Add(time.Minute),
			}}},
			allowedSAN: []string{allowed.String()},
			wantStatus: http.StatusForbidden,
		},
		{
			name: "wrong SAN",
			state: verifiedState(&x509.Certificate{
				Raw: []byte("leaf"), URIs: []*url.URL{denied}, NotBefore: now.Add(-time.Minute), NotAfter: now.Add(time.Minute),
			}),
			allowedSAN: []string{allowed.String()},
			wantStatus: http.StatusForbidden,
		},
		{
			name: "expired certificate",
			state: verifiedState(&x509.Certificate{
				Raw: []byte("leaf"), URIs: []*url.URL{allowed}, NotBefore: now.Add(-time.Hour), NotAfter: now.Add(-time.Minute),
			}),
			allowedSAN: []string{allowed.String()},
			wantStatus: http.StatusForbidden,
		},
		{
			name: "allowed URI SAN",
			state: verifiedState(&x509.Certificate{
				Raw: []byte("leaf"), URIs: []*url.URL{allowed}, NotBefore: now.Add(-time.Minute), NotAfter: now.Add(time.Minute),
			}),
			allowedSAN: []string{allowed.String()},
			wantStatus: http.StatusNoContent,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			engine := gin.New()
			engine.Use(InternalMTLS(config.InternalAuth{AllowedURISANs: tt.allowedSAN}))
			engine.GET("/", func(c *gin.Context) { c.Status(http.StatusNoContent) })
			request := httptest.NewRequest(http.MethodGet, "/", nil)
			request.TLS = tt.state
			recorder := httptest.NewRecorder()
			engine.ServeHTTP(recorder, request)
			if recorder.Code != tt.wantStatus {
				t.Fatalf("status = %d, want %d; body=%s", recorder.Code, tt.wantStatus, recorder.Body.String())
			}
		})
	}
}

func verifiedState(leaf *x509.Certificate) *tls.ConnectionState {
	return &tls.ConnectionState{
		PeerCertificates: []*x509.Certificate{leaf},
		VerifiedChains:   [][]*x509.Certificate{{leaf}},
	}
}

func TestInternalRateLimitIsGlobalAndPerCertificate(t *testing.T) {
	limiter := &internalRateLimiter{perCert: map[string]int{}, globalLimit: 2, certLimit: 1}
	if !limiter.allow("a", 100) {
		t.Fatal("first certificate request denied")
	}
	if limiter.allow("a", 100) {
		t.Fatal("per-certificate limit not enforced")
	}
	if !limiter.allow("b", 100) {
		t.Fatal("second certificate request denied before global limit")
	}
	if limiter.allow("c", 100) {
		t.Fatal("global limit not enforced")
	}
	if !limiter.allow("a", 101) {
		t.Fatal("window did not reset")
	}
}

func TestInternalMTLSWithRealHandshakeAndSANAuthorization(t *testing.T) {
	ca, caKey, caPEM := newTestCertificateAuthority(t)
	serverCertificate := issueTestCertificate(t, ca, caKey, "internal-server", nil, true)
	allowedURI, _ := url.Parse("spiffe://example.com/starry/production")
	deniedURI, _ := url.Parse("spiffe://example.com/starry/other")
	allowedCertificate := issueTestCertificate(t, ca, caKey, "allowed-client", allowedURI, false)
	deniedCertificate := issueTestCertificate(t, ca, caKey, "denied-client", deniedURI, false)

	clientRoots := x509.NewCertPool()
	if !clientRoots.AppendCertsFromPEM(caPEM) {
		t.Fatal("failed to load test client CA")
	}
	engine := gin.New()
	engine.Use(InternalMTLS(config.InternalAuth{AllowedURISANs: []string{allowedURI.String()}}))
	engine.GET("/", func(c *gin.Context) { c.Status(http.StatusNoContent) })
	server := httptest.NewUnstartedServer(engine)
	server.TLS = &tls.Config{
		Certificates: []tls.Certificate{serverCertificate},
		ClientAuth:   tls.RequireAndVerifyClientCert,
		ClientCAs:    clientRoots,
		MinVersion:   tls.VersionTLS13,
	}
	server.StartTLS()
	defer server.Close()

	request := func(certificate *tls.Certificate) (*http.Response, error) {
		transport := server.Client().Transport.(*http.Transport).Clone()
		transport.TLSClientConfig = transport.TLSClientConfig.Clone()
		if certificate != nil {
			transport.TLSClientConfig.Certificates = []tls.Certificate{*certificate}
		}
		client := &http.Client{Transport: transport, Timeout: 2 * time.Second}
		return client.Get(server.URL + "/")
	}

	response, err := request(&allowedCertificate)
	if err != nil {
		t.Fatalf("allowed mTLS request failed: %v", err)
	}
	response.Body.Close()
	if response.StatusCode != http.StatusNoContent {
		t.Fatalf("allowed status = %d, want %d", response.StatusCode, http.StatusNoContent)
	}
	response, err = request(&deniedCertificate)
	if err != nil {
		t.Fatalf("CA-valid denied-SAN request did not reach authorization middleware: %v", err)
	}
	response.Body.Close()
	if response.StatusCode != http.StatusForbidden {
		t.Fatalf("denied SAN status = %d, want %d", response.StatusCode, http.StatusForbidden)
	}
	if response, err = request(nil); err == nil {
		response.Body.Close()
		t.Fatal("request without a client certificate completed TLS handshake")
	}
}

func newTestCertificateAuthority(t *testing.T) (*x509.Certificate, ed25519.PrivateKey, []byte) {
	t.Helper()
	publicKey, privateKey, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	now := time.Now()
	template := &x509.Certificate{
		SerialNumber:          big.NewInt(1),
		Subject:               pkix.Name{CommonName: "Kessoku test CA"},
		NotBefore:             now.Add(-time.Minute),
		NotAfter:              now.Add(time.Hour),
		IsCA:                  true,
		BasicConstraintsValid: true,
		KeyUsage:              x509.KeyUsageCertSign | x509.KeyUsageDigitalSignature,
	}
	der, err := x509.CreateCertificate(rand.Reader, template, template, publicKey, privateKey)
	if err != nil {
		t.Fatal(err)
	}
	certificate, err := x509.ParseCertificate(der)
	if err != nil {
		t.Fatal(err)
	}
	return certificate, privateKey, pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der})
}

func issueTestCertificate(t *testing.T, ca *x509.Certificate, caKey ed25519.PrivateKey, name string, uri *url.URL, server bool) tls.Certificate {
	t.Helper()
	publicKey, privateKey, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	now := time.Now()
	template := &x509.Certificate{
		SerialNumber: big.NewInt(now.UnixNano()),
		Subject:      pkix.Name{CommonName: name},
		NotBefore:    now.Add(-time.Minute),
		NotAfter:     now.Add(time.Hour),
		KeyUsage:     x509.KeyUsageDigitalSignature,
	}
	if server {
		template.ExtKeyUsage = []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth}
		template.IPAddresses = []net.IP{net.ParseIP("127.0.0.1")}
	} else {
		template.ExtKeyUsage = []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth}
		template.URIs = []*url.URL{uri}
	}
	der, err := x509.CreateCertificate(rand.Reader, template, ca, publicKey, caKey)
	if err != nil {
		t.Fatal(err)
	}
	encodedKey, err := x509.MarshalPKCS8PrivateKey(privateKey)
	if err != nil {
		t.Fatal(err)
	}
	certificate, err := tls.X509KeyPair(
		pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der}),
		pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: encodedKey}),
	)
	if err != nil {
		t.Fatal(err)
	}
	return certificate
}
