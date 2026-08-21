package service

import (
	"crypto/ed25519"
	"crypto/rand"
	"crypto/x509"
	"encoding/base64"
	"encoding/pem"
	"os"
	"path/filepath"
	"testing"

	"github.com/q1ngyang/rustdesk-api-kessoku/v2/config"
	internalAuth "github.com/q1ngyang/rustdesk-api-kessoku/v2/internal/auth"
)

func TestControlKeyMaterialMustDifferFromAccessKeyring(t *testing.T) {
	_, privateKey, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	directory := t.TempDir()
	accessPath := filepath.Join(directory, "access.pem")
	controlPath := filepath.Join(directory, "control.seed")
	encoded, err := x509.MarshalPKCS8PrivateKey(privateKey)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(accessPath, pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: encoded}), 0o600); err != nil {
		t.Fatal(err)
	}
	// Use a distinct path and encoding so path/content equality checks cannot
	// accidentally satisfy this test.
	seed := base64.RawStdEncoding.EncodeToString(privateKey.Seed())
	if err := os.WriteFile(controlPath, []byte(seed), 0o600); err != nil {
		t.Fatal(err)
	}
	manager, err := internalAuth.NewManager(config.Auth{
		Enabled: true, Issuer: "https://api.example.test",
		Audiences:  []string{internalAuth.APIAudience, internalAuth.ConnectionAudience},
		CurrentKey: config.AuthKey{ID: "access", PrivateKeyFile: accessPath},
	})
	if err != nil {
		t.Fatal(err)
	}
	cfg := &config.Config{ServerControl: config.ServerControl{Instances: []config.StarryInstance{{
		ID: "starry-1", Enabled: true, ControlKeyFile: controlPath,
	}}}}
	control := NewStarryControlService(cfg, nil, manager)
	instances := control.Instances()
	if len(instances) != 1 || instances[0].ErrorCode != "CONTROL_KEYRING_NOT_ISOLATED" || instances[0].Available {
		t.Fatalf("keyring reuse was not rejected: %+v", instances)
	}
}
