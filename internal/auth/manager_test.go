package auth

import (
	"crypto/ed25519"
	"crypto/rand"
	"crypto/x509"
	"encoding/pem"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/q1ngyang/rustdesk-api-kessoku/v2/config"
)

func TestAccessTokenStrictProfile(t *testing.T) {
	manager, privateKey := testManager(t, "current", nil)
	now := time.Date(2026, 8, 18, 8, 0, 0, 0, time.UTC)
	manager.now = func() time.Time { return now }

	issued, err := manager.IssueAccessToken(42, 3)
	if err != nil {
		t.Fatal(err)
	}
	claims, err := manager.VerifyAccessToken(issued.Token, VerifyOptions{
		Audience:      ConnectionAudience,
		RequiredScope: ConnectScope,
	})
	if err != nil {
		t.Fatalf("verify issued token: %v", err)
	}
	if claims.UserID != 42 || claims.Subject != "42" || claims.AuthVersion != 3 {
		t.Fatalf("unexpected claims: %+v", claims)
	}

	tests := []struct {
		name   string
		mutate func(*AccessClaims, map[string]interface{})
	}{
		{name: "wrong issuer", mutate: func(c *AccessClaims, _ map[string]interface{}) { c.Issuer = "https://wrong.invalid" }},
		{name: "wrong audience", mutate: func(c *AccessClaims, _ map[string]interface{}) { c.Audience = jwt.ClaimStrings{"other"} }},
		{name: "wrong type", mutate: func(_ *AccessClaims, h map[string]interface{}) { h["typ"] = "JWT" }},
		{name: "unknown kid", mutate: func(_ *AccessClaims, h map[string]interface{}) { h["kid"] = "missing" }},
		{name: "subject mismatch", mutate: func(c *AccessClaims, _ map[string]interface{}) { c.Subject = "43" }},
		{name: "wrong token use", mutate: func(c *AccessClaims, _ map[string]interface{}) { c.TokenUse = "refresh" }},
		{name: "missing scope", mutate: func(c *AccessClaims, _ map[string]interface{}) { c.Scope = []string{"profile:read"} }},
		{name: "future nbf", mutate: func(c *AccessClaims, _ map[string]interface{}) {
			c.NotBefore = jwt.NewNumericDate(now.Add(2 * time.Minute))
		}},
		{name: "excessive lifetime", mutate: func(c *AccessClaims, _ map[string]interface{}) {
			c.ExpiresAt = jwt.NewNumericDate(now.Add(2 * time.Hour))
		}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			candidate := *claims
			candidate.Scope = append([]string(nil), claims.Scope...)
			headers := map[string]interface{}{"kid": "current", "typ": AccessTokenType}
			tt.mutate(&candidate, headers)
			token := jwt.NewWithClaims(jwt.SigningMethodEdDSA, candidate)
			for key, value := range headers {
				token.Header[key] = value
			}
			signed, err := token.SignedString(privateKey)
			if err != nil {
				t.Fatal(err)
			}
			if _, err := manager.VerifyAccessToken(signed, VerifyOptions{Audience: ConnectionAudience, RequiredScope: ConnectScope}); err == nil {
				t.Fatal("invalid token accepted")
			}
		})
	}

	manager.now = func() time.Time { return now.Add(16 * time.Minute) }
	if _, err := manager.VerifyAccessToken(issued.Token, VerifyOptions{Audience: APIAudience}); err == nil {
		t.Fatal("expired token accepted")
	}
	if _, err := manager.VerifyAccessToken(string(make([]byte, 8193)), VerifyOptions{Audience: APIAudience}); !errors.Is(err, ErrTokenTooLarge) {
		t.Fatalf("oversize error = %v, want ErrTokenTooLarge", err)
	}
}

func TestAlgorithmSubstitutionIsRejected(t *testing.T) {
	manager, _ := testManager(t, "current", nil)
	now := time.Now().UTC()
	claims := AccessClaims{
		UserID:      42,
		TokenUse:    AccessTokenUse,
		Scope:       []string{ConnectScope},
		AuthVersion: 1,
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer:    manager.issuer,
			Subject:   "42",
			Audience:  manager.audiences,
			ID:        "0191f6a0-0000-7000-8000-000000000001",
			IssuedAt:  jwt.NewNumericDate(now),
			NotBefore: jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(10 * time.Minute)),
		},
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	token.Header["kid"] = "current"
	token.Header["typ"] = AccessTokenType
	signed, err := token.SignedString([]byte(manager.public["current"]))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := manager.VerifyAccessToken(signed, VerifyOptions{Audience: APIAudience}); err == nil {
		t.Fatal("HS256 algorithm substitution accepted")
	}
}

func TestCurrentAndPreviousKeyRotation(t *testing.T) {
	oldManager, oldPrivate := testManager(t, "old", nil)
	issued, err := oldManager.IssueAccessToken(7, 1)
	if err != nil {
		t.Fatal(err)
	}
	newManager, _ := testManager(t, "new", map[string]ed25519.PublicKey{"old": oldPrivate.Public().(ed25519.PublicKey)})
	if _, err := newManager.VerifyAccessToken(issued.Token, VerifyOptions{Audience: APIAudience}); err != nil {
		t.Fatalf("previous key rejected during overlap: %v", err)
	}
	keys := newManager.JWKS().Keys
	if len(keys) != 2 || keys[0].KeyID != "new" || keys[1].KeyID != "old" {
		t.Fatalf("unexpected JWKS rotation order: %+v", keys)
	}
	withoutPrevious, _ := testManager(t, "newer", nil)
	if _, err := withoutPrevious.VerifyAccessToken(issued.Token, VerifyOptions{Audience: APIAudience}); err == nil {
		t.Fatal("removed previous key still accepted")
	}
}

func testManager(t *testing.T, currentID string, previous map[string]ed25519.PublicKey) (*Manager, ed25519.PrivateKey) {
	t.Helper()
	_, privateKey, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	directory := t.TempDir()
	privatePath := filepath.Join(directory, "current.pem")
	encoded, err := x509.MarshalPKCS8PrivateKey(privateKey)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(privatePath, pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: encoded}), 0o600); err != nil {
		t.Fatal(err)
	}
	previousConfig := make([]config.AuthKey, 0, len(previous))
	for keyID, publicKey := range previous {
		publicPath := filepath.Join(directory, keyID+".pem")
		encoded, err := x509.MarshalPKIXPublicKey(publicKey)
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(publicPath, pem.EncodeToMemory(&pem.Block{Type: "PUBLIC KEY", Bytes: encoded}), 0o600); err != nil {
			t.Fatal(err)
		}
		previousConfig = append(previousConfig, config.AuthKey{ID: keyID, PublicKeyFile: publicPath})
	}
	manager, err := NewManager(config.Auth{
		Enabled:         true,
		Issuer:          "https://api.example.test",
		Audiences:       []string{APIAudience, ConnectionAudience},
		AccessTokenTTL:  15 * time.Minute,
		MaximumTokenTTL: time.Hour,
		ClockSkew:       30 * time.Second,
		MaxTokenBytes:   8192,
		CurrentKey: config.AuthKey{
			ID:             currentID,
			PrivateKeyFile: privatePath,
		},
		PreviousKeys: previousConfig,
	})
	if err != nil {
		t.Fatal(err)
	}
	return manager, privateKey
}
