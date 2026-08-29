package controlauth

import (
	"crypto/ed25519"
	"crypto/rand"
	"crypto/x509"
	"encoding/pem"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

func TestSignerProducesShortScopedServiceJWT(t *testing.T) {
	publicKey, privateKey, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	encoded, err := x509.MarshalPKCS8PrivateKey(privateKey)
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "control.pem")
	if err := os.WriteFile(path, pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: encoded}), 0o600); err != nil {
		t.Fatal(err)
	}
	signer, err := NewSigner("https://control-issuer.example", "kessoku-production", "control-1", path, 2*time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	now := time.Date(2026, 8, 18, 8, 0, 0, 0, time.UTC)
	signer.now = func() time.Time { return now }
	signed, err := signer.Sign("0191f6a0-0000-7000-8000-000000000001", "starry.config.apply", 42)
	if err != nil {
		t.Fatal(err)
	}
	claims := &Claims{}
	token, err := jwt.ParseWithClaims(signed, claims, func(token *jwt.Token) (interface{}, error) {
		return publicKey, nil
	}, jwt.WithValidMethods([]string{"EdDSA"}), jwt.WithTimeFunc(func() time.Time { return now }))
	if err != nil || !token.Valid {
		t.Fatalf("verify control JWT: %v", err)
	}
	if claims.Subject != ServiceSubject || claims.AuthorizedParty != "kessoku-production" || claims.Actor == nil || claims.Actor.Subject != "user:42" {
		t.Fatalf("unexpected control claims: %+v", claims)
	}
	if len(claims.Audience) != 1 || claims.Audience[0] != "urn:starry-control:0191f6a0-0000-7000-8000-000000000001" {
		t.Fatalf("unexpected audience: %v", claims.Audience)
	}
	if len(claims.Scope) != 1 || claims.Scope[0] != "starry.config.apply" {
		t.Fatalf("unexpected scope: %v", claims.Scope)
	}
	if claims.ExpiresAt.Sub(claims.IssuedAt.Time) != 2*time.Minute {
		t.Fatalf("control lifetime = %s", claims.ExpiresAt.Sub(claims.IssuedAt.Time))
	}
	if token.Header["kid"] != "control-1" || token.Header["alg"] != "EdDSA" {
		t.Fatalf("unexpected header: %+v", token.Header)
	}
	serviceSigned, err := signer.SignService("0191f6a0-0000-7000-8000-000000000001", "starry.peer.verify")
	if err != nil {
		t.Fatal(err)
	}
	serviceClaims := &Claims{}
	if _, err := jwt.ParseWithClaims(serviceSigned, serviceClaims, func(token *jwt.Token) (interface{}, error) {
		return publicKey, nil
	}, jwt.WithValidMethods([]string{"EdDSA"}), jwt.WithTimeFunc(func() time.Time { return now })); err != nil {
		t.Fatal(err)
	}
	if serviceClaims.Actor != nil || serviceClaims.Subject != ServiceSubject {
		t.Fatalf("service token unexpectedly delegated an actor: %+v", serviceClaims)
	}
	if _, err := NewSigner("issuer", "azp", "kid", path, 6*time.Minute); err == nil {
		t.Fatal("control lifetime over five minutes accepted")
	}
}
