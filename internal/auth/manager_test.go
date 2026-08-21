package auth

import (
	"crypto/ed25519"
	"crypto/rand"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
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
		{name: "invalid jti", mutate: func(c *AccessClaims, _ map[string]interface{}) { c.ID = "not-a-uuid" }},
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

func TestAccessTokenWireContractMatchesStarryVerifier(t *testing.T) {
	manager, _ := testManager(t, "wire-contract", nil)
	issued, err := manager.IssueAccessToken(42, 7)
	if err != nil {
		t.Fatal(err)
	}
	segments := strings.Split(issued.Token, ".")
	if len(segments) != 3 {
		t.Fatalf("compact JWT segment count = %d, want 3", len(segments))
	}
	decode := func(segment string, destination interface{}) {
		t.Helper()
		raw, err := base64.RawURLEncoding.DecodeString(segment)
		if err != nil {
			t.Fatal(err)
		}
		if err := json.Unmarshal(raw, destination); err != nil {
			t.Fatal(err)
		}
	}
	header := struct {
		Algorithm string `json:"alg"`
		KeyID     string `json:"kid"`
		Type      string `json:"typ"`
	}{}
	decode(segments[0], &header)
	if header.Algorithm != AlgorithmEdDSA || header.KeyID != "wire-contract" || header.Type != AccessTokenType {
		t.Fatalf("unexpected protected header: %+v", header)
	}
	claims := struct {
		UserID      uint64   `json:"user_id"`
		Subject     string   `json:"sub"`
		Audience    []string `json:"aud"`
		Scope       []string `json:"scope"`
		TokenUse    string   `json:"token_use"`
		AuthVersion uint64   `json:"auth_version"`
		JTI         string   `json:"jti"`
	}{}
	decode(segments[1], &claims)
	if claims.UserID != 42 || claims.Subject != "42" || claims.AuthVersion != 7 || claims.TokenUse != AccessTokenUse {
		t.Fatalf("unexpected access-token wire claims: %+v", claims)
	}
	if len(claims.Audience) != 2 || claims.Audience[0] != APIAudience || claims.Audience[1] != ConnectionAudience {
		t.Fatalf("unexpected audience array: %#v", claims.Audience)
	}
	if len(claims.Scope) != 1 || claims.Scope[0] != ConnectScope {
		t.Fatalf("unexpected scope array: %#v", claims.Scope)
	}
	if parsed, err := uuid.Parse(claims.JTI); err != nil || parsed.Version() != 7 {
		t.Fatalf("JTI %q is not UUIDv7: %v", claims.JTI, err)
	}
}

func TestConnectionTokenKeepsStarryWireProfileButCannotAuthorizeAPI(t *testing.T) {
	manager, _ := testManager(t, "connection-wire", nil)
	now := time.Date(2026, 8, 21, 7, 0, 0, 0, time.UTC)
	manager.now = func() time.Time { return now }
	issued, err := manager.IssueConnectionToken(42, 7, 10*time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	claims, err := manager.VerifyAccessToken(issued.Token, VerifyOptions{Audience: ConnectionAudience, RequiredScope: ConnectScope})
	if err != nil {
		t.Fatalf("Starry profile rejected connection token: %v", err)
	}
	if len(claims.Audience) != 1 || claims.Audience[0] != ConnectionAudience || claims.TokenUse != AccessTokenUse || !claims.HasScope(ConnectScope) {
		t.Fatalf("unexpected connection claims: %+v", claims)
	}
	if claims.ExpiresAt.Time.Sub(claims.IssuedAt.Time) != 10*time.Minute {
		t.Fatalf("connection lifetime = %s", claims.ExpiresAt.Time.Sub(claims.IssuedAt.Time))
	}
	if _, err := manager.VerifyAccessToken(issued.Token, VerifyOptions{Audience: APIAudience}); err == nil {
		t.Fatal("connection-only token authorized the Kessoku API")
	}
	if _, err := manager.IssueConnectionToken(42, 7, 0); err == nil {
		t.Fatal("zero connection lifetime accepted")
	}
	if _, err := manager.IssueConnectionToken(42, 7, manager.maximumTTL+time.Second); err == nil {
		t.Fatal("excessive connection lifetime accepted")
	}
}

func TestNilManagerCannotIssueAnyToken(t *testing.T) {
	var manager *Manager
	if _, err := manager.IssueAccessToken(1, 1); err == nil {
		t.Fatal("nil manager issued an access token")
	}
	if _, err := manager.IssueConnectionToken(1, 1, time.Minute); err == nil {
		t.Fatal("nil manager issued a connection token")
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

func TestManagerRejectsTokenLimitAboveContractMaximum(t *testing.T) {
	_, err := NewManager(config.Auth{
		Enabled:       true,
		Issuer:        "https://api.example.test",
		Audiences:     []string{APIAudience, ConnectionAudience},
		MaxTokenBytes: config.MaxAccessTokenBytes + 1,
		CurrentKey:    config.AuthKey{ID: "current", PrivateKeyFile: "not-read"},
	})
	if err == nil {
		t.Fatal("token limit above 8 KiB accepted")
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
