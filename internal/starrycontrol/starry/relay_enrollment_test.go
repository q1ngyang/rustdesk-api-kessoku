package starry

import (
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/hex"
	"encoding/pem"
	"math/big"
	"testing"
	"time"

	"github.com/q1ngyang/rustdesk-api-kessoku/v3/internal/starrycontrol"
)

func TestRelayEnrollmentPrepareValidationMatchesFrozenAgent(t *testing.T) {
	wss := "wss://relay.example:21119/ws/telemetry"
	port := 22119
	valid := starrycontrol.RelayEnrollmentPrepareRequest{
		Version: 1, NodeID: "relay-sg_1.example", RelayServer: "relay.example:21117", PublicEndpoint: "relay.example:21117",
		RelayPool: "primary", Profile: "native-wss-fastmedia", WSSEndpoint: &wss,
		ActivateAfterHealth: true, MaxSessions: 1000, CapacityBandwidthBPS: 1_000_000_000,
		FastMediaUDPPort: &port, ExpiresInSeconds: 600,
	}
	if !validRelayEnrollmentPrepareRequest(valid) {
		t.Fatal("frozen valid Relay enrollment request was rejected")
	}
	withoutExplicitExpiry := valid
	withoutExplicitExpiry.ExpiresInSeconds = 0
	if !validRelayEnrollmentPrepareRequest(withoutExplicitExpiry) {
		t.Fatal("omitted expiry did not preserve Starry's frozen 600-second default")
	}
	tests := []struct {
		name   string
		mutate func(*starrycontrol.RelayEnrollmentPrepareRequest)
	}{
		{"unsafe node id", func(value *starrycontrol.RelayEnrollmentPrepareRequest) { value.NodeID = "../relay" }},
		{"hidden pool", func(value *starrycontrol.RelayEnrollmentPrepareRequest) { value.RelayPool = ".hidden" }},
		{"endpoint drift", func(value *starrycontrol.RelayEnrollmentPrepareRequest) { value.PublicEndpoint = "other.example:21117" }},
		{"missing endpoint port", func(value *starrycontrol.RelayEnrollmentPrepareRequest) {
			value.RelayServer, value.PublicEndpoint = "relay.example", "relay.example"
		}},
		{"telemetry query", func(value *starrycontrol.RelayEnrollmentPrepareRequest) {
			endpoint := wss + "?token=forbidden"
			value.WSSEndpoint = &endpoint
		}},
		{"native with WSS", func(value *starrycontrol.RelayEnrollmentPrepareRequest) {
			value.Profile = "native"
			value.FastMediaUDPPort = nil
		}},
		{"WSS without endpoint", func(value *starrycontrol.RelayEnrollmentPrepareRequest) {
			value.Profile = "native-wss"
			value.WSSEndpoint = nil
			value.FastMediaUDPPort = nil
		}},
		{"FastMedia without UDP", func(value *starrycontrol.RelayEnrollmentPrepareRequest) { value.FastMediaUDPPort = nil }},
		{"UDP on non-FastMedia", func(value *starrycontrol.RelayEnrollmentPrepareRequest) { value.Profile = "native-wss" }},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			candidate := valid
			test.mutate(&candidate)
			if validRelayEnrollmentPrepareRequest(candidate) {
				t.Fatal("invalid frozen Relay enrollment relationship was accepted")
			}
		})
	}
}

func TestRelayEnrollmentCredentialsBindCSRKeyAndCA(t *testing.T) {
	now := time.Now().UTC().Truncate(time.Second)
	caPublic, caPrivate, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	caTemplate := &x509.Certificate{
		SerialNumber: big.NewInt(1), Subject: pkix.Name{CommonName: "Relay enrollment CA"},
		NotBefore: now.Add(-time.Minute), NotAfter: now.Add(time.Hour), IsCA: true, BasicConstraintsValid: true,
		KeyUsage: x509.KeyUsageCertSign | x509.KeyUsageDigitalSignature,
	}
	caDER, err := x509.CreateCertificate(rand.Reader, caTemplate, caTemplate, caPublic, caPrivate)
	if err != nil {
		t.Fatal(err)
	}
	ca, err := x509.ParseCertificate(caDER)
	if err != nil {
		t.Fatal(err)
	}
	leafPublic, leafPrivate, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	leafTemplate := &x509.Certificate{
		SerialNumber: big.NewInt(2), Subject: pkix.Name{CommonName: "relay-sg"},
		NotBefore: now.Add(-time.Minute), NotAfter: now.Add(30 * time.Minute), BasicConstraintsValid: true,
		KeyUsage: x509.KeyUsageDigitalSignature,
	}
	leafDER, err := x509.CreateCertificate(rand.Reader, leafTemplate, ca, leafPublic, caPrivate)
	if err != nil {
		t.Fatal(err)
	}
	publicDER, err := x509.MarshalPKIXPublicKey(leafPrivate.Public())
	if err != nil {
		t.Fatal(err)
	}
	fingerprint := sha256.Sum256(publicDER)
	bundle := starrycontrol.RelayEnrollmentBundle{
		NodeCertificatePEM: string(pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: leafDER})),
		RelayCAPEM:         string(pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: caDER})),
	}
	expected := "sha256:" + hex.EncodeToString(fingerprint[:])
	if err := validateRelayEnrollmentCredentials(bundle, expected, now); err != nil {
		t.Fatalf("valid Relay certificate binding rejected: %v", err)
	}
	if err := validateRelayEnrollmentCredentials(bundle, "sha256:"+hex.EncodeToString(make([]byte, 32)), now); err == nil {
		t.Fatal("Relay certificate with a different CSR key fingerprint was accepted")
	}
	if err := validateRelayEnrollmentCredentials(bundle, expected, now.Add(2*time.Hour)); err == nil {
		t.Fatal("expired Relay certificate was accepted")
	}
}
