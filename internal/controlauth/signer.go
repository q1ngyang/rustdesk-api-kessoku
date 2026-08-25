package controlauth

import (
	"crypto/ed25519"
	"crypto/x509"
	"encoding/base64"
	"encoding/hex"
	"encoding/pem"
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	internalAuth "github.com/q1ngyang/rustdesk-api-kessoku/v3/internal/auth"
)

const (
	ServiceSubject = "service:rustdesk-api-kessoku"
	maxLifetime    = 5 * time.Minute
)

type Actor struct {
	Subject string `json:"sub"`
}

// PrivateKeyPublicFingerprint returns a non-secret identifier for startup
// keyring-isolation checks.
func PrivateKeyPublicFingerprint(path string) (string, error) {
	privateKey, err := loadPrivateKey(path)
	if err != nil {
		return "", err
	}
	return internalAuth.PublicKeyFingerprint(privateKey.Public().(ed25519.PublicKey)), nil
}

type Claims struct {
	AuthorizedParty string   `json:"azp"`
	Scope           []string `json:"scope"`
	Actor           Actor    `json:"act"`
	jwt.RegisteredClaims
}

type Signer struct {
	issuer          string
	authorizedParty string
	keyID           string
	privateKey      ed25519.PrivateKey
	lifetime        time.Duration
	now             func() time.Time
}

func NewSigner(issuer, authorizedParty, keyID, privateKeyFile string, lifetime time.Duration) (*Signer, error) {
	if issuer == "" || authorizedParty == "" || keyID == "" || privateKeyFile == "" {
		return nil, errors.New("control issuer, authorized party, key id, and private key file are required")
	}
	if lifetime <= 0 {
		lifetime = 2 * time.Minute
	}
	if lifetime > maxLifetime {
		return nil, errors.New("control JWT lifetime exceeds five minutes")
	}
	key, err := loadPrivateKey(privateKeyFile)
	if err != nil {
		return nil, fmt.Errorf("load control signing key: %w", err)
	}
	return &Signer{
		issuer:          issuer,
		authorizedParty: authorizedParty,
		keyID:           keyID,
		privateKey:      key,
		lifetime:        lifetime,
		now:             time.Now,
	}, nil
}

func (s *Signer) Sign(instanceID, scope string, actorUserID uint) (string, error) {
	if s == nil || instanceID == "" || scope == "" || actorUserID == 0 {
		return "", errors.New("control JWT requires signer, instance, scope, and actor")
	}
	now := s.now().UTC()
	jti, err := uuid.NewV7()
	if err != nil {
		return "", fmt.Errorf("generate control token id: %w", err)
	}
	claims := Claims{
		AuthorizedParty: s.authorizedParty,
		Scope:           []string{scope},
		Actor:           Actor{Subject: "user:" + strconv.FormatUint(uint64(actorUserID), 10)},
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer:    s.issuer,
			Subject:   ServiceSubject,
			Audience:  jwt.ClaimStrings{"urn:starry-control:" + instanceID},
			IssuedAt:  jwt.NewNumericDate(now),
			NotBefore: jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(s.lifetime)),
			ID:        jti.String(),
		},
	}
	token := jwt.NewWithClaims(jwt.SigningMethodEdDSA, claims)
	token.Header["kid"] = s.keyID
	token.Header["typ"] = "JWT"
	signed, err := token.SignedString(s.privateKey)
	if err != nil {
		return "", fmt.Errorf("sign control JWT: %w", err)
	}
	return signed, nil
}

func loadPrivateKey(path string) (ed25519.PrivateKey, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	if block, _ := pem.Decode(raw); block != nil {
		parsed, err := x509.ParsePKCS8PrivateKey(block.Bytes)
		if err != nil {
			return nil, errors.New("invalid PKCS#8 Ed25519 private key")
		}
		key, ok := parsed.(ed25519.PrivateKey)
		if !ok || len(key) != ed25519.PrivateKeySize {
			return nil, errors.New("control private key is not Ed25519")
		}
		return append(ed25519.PrivateKey(nil), key...), nil
	}
	value := strings.TrimSpace(string(raw))
	var decoded []byte
	if candidate, err := base64.RawStdEncoding.DecodeString(value); err == nil {
		decoded = candidate
	} else if candidate, err := base64.StdEncoding.DecodeString(value); err == nil {
		decoded = candidate
	} else if candidate, err := hex.DecodeString(value); err == nil {
		decoded = candidate
	}
	if len(decoded) == ed25519.SeedSize {
		return ed25519.NewKeyFromSeed(decoded), nil
	}
	if len(decoded) == ed25519.PrivateKeySize {
		return append(ed25519.PrivateKey(nil), decoded...), nil
	}
	return nil, errors.New("invalid Ed25519 control private key")
}
