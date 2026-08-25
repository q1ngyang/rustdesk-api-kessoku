package starry

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"math/big"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/q1ngyang/rustdesk-api-kessoku/v3/config"
	"github.com/q1ngyang/rustdesk-api-kessoku/v3/internal/starrycontrol"
)

func TestNewProviderUsesRealMTLSAndIgnoresAmbientProxy(t *testing.T) {
	const (
		instanceID      = "0191f6a0-0000-7000-8000-000000000040"
		authorizedParty = "spiffe://example.test/kessoku.test"
	)
	ca, caKey, caPEM := newProviderTestCA(t)
	serverCertificate := issueProviderTestCertificate(t, ca, caKey, "starry.test", true)
	clientCertificate := issueProviderTestCertificate(t, ca, caKey, "kessoku.test", false)
	clientCertFile, clientKeyFile := writeProviderTestKeyPair(t, clientCertificate)
	caFile := filepath.Join(t.TempDir(), "ca.pem")
	if err := os.WriteFile(caFile, caPEM, 0o600); err != nil {
		t.Fatal(err)
	}
	_, controlPrivateKey, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	controlKeyFile := writeProviderTestPrivateKey(t, controlPrivateKey)

	clientSeen := false
	server := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.TLS == nil || len(r.TLS.VerifiedChains) == 0 || len(r.TLS.PeerCertificates) == 0 {
			t.Error("request did not present a verified mTLS client certificate")
			w.WriteHeader(http.StatusForbidden)
			return
		}
		clientSeen = true
		authorization := r.Header.Get("Authorization")
		if !strings.HasPrefix(authorization, "Bearer ") || len(strings.TrimPrefix(authorization, "Bearer ")) > config.MaxAccessTokenBytes {
			t.Errorf("invalid scoped control credential header")
			w.WriteHeader(http.StatusForbidden)
			return
		}
		rawToken := strings.TrimPrefix(authorization, "Bearer ")
		parsed, parseErr := jwt.Parse(rawToken, func(token *jwt.Token) (interface{}, error) {
			return controlPrivateKey.Public().(ed25519.PublicKey), nil
		}, jwt.WithValidMethods([]string{"EdDSA"}), jwt.WithIssuer("https://api.example.test"), jwt.WithAudience("urn:starry-control:"+instanceID), jwt.WithExpirationRequired())
		claims, claimsOK := parsedClaims(parsed)
		if parseErr != nil || !claimsOK || parsed.Header["kid"] != "control-2026-01" || claims["sub"] != "service:rustdesk-api-kessoku" || claims["azp"] != authorizedParty || !controlClaimsContain(claims, "scope", "starry.control.read") || !controlActorMatches(claims, "user:42") {
			t.Errorf("invalid scoped control credential: err=%v claims=%v", parseErr, claims)
			w.WriteHeader(http.StatusForbidden)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(providerCapabilitiesJSON(instanceID, 1)))
	}))
	clientCAs := x509.NewCertPool()
	clientCAs.AppendCertsFromPEM(caPEM)
	server.TLS = &tls.Config{
		Certificates: []tls.Certificate{serverCertificate.tlsCertificate},
		ClientAuth:   tls.RequireAndVerifyClientCert,
		ClientCAs:    clientCAs,
		MinVersion:   tls.VersionTLS13,
	}
	server.StartTLS()
	defer server.Close()
	t.Setenv("HTTPS_PROXY", "http://127.0.0.1:1")

	provider, err := NewProvider(config.StarryInstance{
		ID:                 "deployment-instance",
		Enabled:            true,
		BaseURL:            server.URL,
		ExpectedInstanceID: instanceID,
		TLSServerName:      "starry.test",
		CAFile:             caFile,
		ClientCertFile:     clientCertFile,
		ClientKeyFile:      clientKeyFile,
		ControlKeyFile:     controlKeyFile,
		ControlKeyID:       "control-2026-01",
		ControlIssuer:      "https://api.example.test",
		AuthorizedParty:    authorizedParty,
	}, config.ServerControl{RequestTimeout: time.Second})
	if err != nil {
		t.Fatal(err)
	}
	ctx := starrycontrol.WithRequestMetadata(context.Background(), starrycontrol.RequestMetadata{
		ActorUserID: 42,
		RequestID:   "0191f6a0-0000-7000-8000-000000000001",
	})
	capabilities, err := provider.Capabilities(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if !clientSeen || capabilities.Instance.ID != instanceID {
		t.Fatalf("mTLS capability response = %+v, clientSeen=%v", capabilities, clientSeen)
	}
}

func parsedClaims(token *jwt.Token) (jwt.MapClaims, bool) {
	if token == nil || !token.Valid {
		return nil, false
	}
	claims, ok := token.Claims.(jwt.MapClaims)
	return claims, ok
}

func controlClaimsContain(claims jwt.MapClaims, name, expected string) bool {
	values, ok := claims[name].([]interface{})
	if !ok {
		return false
	}
	for _, value := range values {
		if value == expected {
			return true
		}
	}
	return false
}

func controlActorMatches(claims jwt.MapClaims, expected string) bool {
	actor, ok := claims["act"].(map[string]interface{})
	return ok && actor["sub"] == expected
}

type providerTestCertificate struct {
	certificatePEM []byte
	privateKeyPEM  []byte
	tlsCertificate tls.Certificate
}

func newProviderTestCA(t *testing.T) (*x509.Certificate, ed25519.PrivateKey, []byte) {
	t.Helper()
	publicKey, privateKey, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	now := time.Now()
	template := &x509.Certificate{
		SerialNumber:          big.NewInt(1),
		Subject:               pkix.Name{CommonName: "Starry provider test CA"},
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

func issueProviderTestCertificate(t *testing.T, ca *x509.Certificate, caKey ed25519.PrivateKey, identity string, server bool) providerTestCertificate {
	t.Helper()
	publicKey, privateKey, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	now := time.Now()
	template := &x509.Certificate{
		SerialNumber: big.NewInt(now.UnixNano()),
		Subject:      pkix.Name{CommonName: identity},
		NotBefore:    now.Add(-time.Minute),
		NotAfter:     now.Add(time.Hour),
		KeyUsage:     x509.KeyUsageDigitalSignature,
	}
	if server {
		template.DNSNames = []string{identity}
		template.ExtKeyUsage = []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth}
	} else {
		identityURI, _ := url.Parse("spiffe://example.test/" + identity)
		template.URIs = []*url.URL{identityURI}
		template.ExtKeyUsage = []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth}
	}
	der, err := x509.CreateCertificate(rand.Reader, template, ca, publicKey, caKey)
	if err != nil {
		t.Fatal(err)
	}
	encodedKey, err := x509.MarshalPKCS8PrivateKey(privateKey)
	if err != nil {
		t.Fatal(err)
	}
	certificatePEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der})
	privateKeyPEM := pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: encodedKey})
	tlsCertificate, err := tls.X509KeyPair(certificatePEM, privateKeyPEM)
	if err != nil {
		t.Fatal(err)
	}
	return providerTestCertificate{certificatePEM: certificatePEM, privateKeyPEM: privateKeyPEM, tlsCertificate: tlsCertificate}
}

func writeProviderTestKeyPair(t *testing.T, certificate providerTestCertificate) (string, string) {
	t.Helper()
	directory := t.TempDir()
	certificateFile := filepath.Join(directory, "client.pem")
	privateKeyFile := filepath.Join(directory, "client-key.pem")
	if err := os.WriteFile(certificateFile, certificate.certificatePEM, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(privateKeyFile, certificate.privateKeyPEM, 0o600); err != nil {
		t.Fatal(err)
	}
	return certificateFile, privateKeyFile
}

func writeProviderTestPrivateKey(t *testing.T, privateKey ed25519.PrivateKey) string {
	t.Helper()
	encoded, err := x509.MarshalPKCS8PrivateKey(privateKey)
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "control-key.pem")
	if err := os.WriteFile(path, pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: encoded}), 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}
