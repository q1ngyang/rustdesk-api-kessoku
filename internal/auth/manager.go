package auth

import (
	"crypto/ed25519"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/hex"
	"encoding/pem"
	"errors"
	"fmt"
	"os"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"github.com/q1ngyang/rustdesk-api-kessoku/v2/config"
)

var (
	ErrTokenInvalid   = errors.New("access token is invalid")
	ErrTokenTooLarge  = errors.New("access token exceeds maximum size")
	ErrUnknownKey     = errors.New("access token uses an unknown key")
	ErrInvalidKeyFile = errors.New("invalid Ed25519 key file")
)

type Manager struct {
	issuer       string
	audiences    jwt.ClaimStrings
	ttl          time.Duration
	maximumTTL   time.Duration
	clockSkew    time.Duration
	maxTokenSize int
	currentID    string
	current      ed25519.PrivateKey
	public       map[string]ed25519.PublicKey
	now          func() time.Time
}

func NewManager(cfg config.Auth) (*Manager, error) {
	if !cfg.Enabled {
		return nil, nil
	}
	if strings.TrimSpace(cfg.Issuer) == "" {
		return nil, errors.New("auth issuer is required")
	}
	if len(cfg.Audiences) == 0 || !contains(cfg.Audiences, APIAudience) || !contains(cfg.Audiences, ConnectionAudience) {
		return nil, fmt.Errorf("auth audiences must include %q and %q", APIAudience, ConnectionAudience)
	}
	if cfg.CurrentKey.ID == "" || cfg.CurrentKey.PrivateKeyFile == "" {
		return nil, errors.New("auth current key id and private-key-file are required")
	}
	if cfg.MaxTokenBytes > config.MaxAccessTokenBytes {
		return nil, fmt.Errorf("auth max-token-bytes must not exceed %d", config.MaxAccessTokenBytes)
	}
	ttl := cfg.EffectiveAccessTokenTTL()
	maximumTTL := cfg.EffectiveMaximumTokenTTL()
	if ttl <= 0 || ttl > maximumTTL {
		return nil, errors.New("auth access-token-ttl must be positive and not exceed maximum-token-ttl")
	}
	privateKey, err := loadPrivateKeyFile(cfg.CurrentKey.PrivateKeyFile)
	if err != nil {
		return nil, fmt.Errorf("load current auth key: %w", err)
	}
	m := &Manager{
		issuer:       cfg.Issuer,
		audiences:    append(jwt.ClaimStrings(nil), cfg.Audiences...),
		ttl:          ttl,
		maximumTTL:   maximumTTL,
		clockSkew:    cfg.EffectiveClockSkew(),
		maxTokenSize: cfg.EffectiveMaxTokenBytes(),
		currentID:    cfg.CurrentKey.ID,
		current:      privateKey,
		public:       make(map[string]ed25519.PublicKey, 1+len(cfg.PreviousKeys)),
		now:          time.Now,
	}
	m.public[m.currentID] = append(ed25519.PublicKey(nil), privateKey.Public().(ed25519.PublicKey)...)
	for _, previous := range cfg.PreviousKeys {
		if previous.ID == "" || previous.PublicKeyFile == "" {
			return nil, errors.New("each previous auth key requires id and public-key-file")
		}
		if _, exists := m.public[previous.ID]; exists {
			return nil, fmt.Errorf("duplicate auth key id %q", previous.ID)
		}
		publicKey, err := loadPublicKeyFile(previous.PublicKeyFile)
		if err != nil {
			return nil, fmt.Errorf("load previous auth key %q: %w", previous.ID, err)
		}
		m.public[previous.ID] = publicKey
	}
	return m, nil
}

// UsesPublicKeyFingerprint lets startup validation prove that the independently
// configured control signer does not reuse a current or previous access-token
// signing key. The fingerprint contains no private material.
func (m *Manager) UsesPublicKeyFingerprint(fingerprint string) bool {
	if m == nil || fingerprint == "" {
		return false
	}
	for _, publicKey := range m.public {
		if PublicKeyFingerprint(publicKey) == fingerprint {
			return true
		}
	}
	return false
}

func PublicKeyFingerprint(publicKey ed25519.PublicKey) string {
	digest := sha256.Sum256(publicKey)
	return hex.EncodeToString(digest[:])
}

func (m *Manager) IssueAccessToken(userID uint, authVersion uint64) (IssuedToken, error) {
	if m == nil {
		return IssuedToken{}, errors.New("Ed25519 access-token signer is not configured")
	}
	return m.issueAccessToken(userID, authVersion, m.audiences, m.ttl)
}

// IssueConnectionToken preserves the Starry patch-v1.2.0 at+jwt wire profile
// while narrowing the token to the rustdesk-connect audience and a caller-
// bounded lifetime. It is deliberately not valid at Kessoku API/admin routes.
func (m *Manager) IssueConnectionToken(userID uint, authVersion uint64, ttl time.Duration) (IssuedToken, error) {
	if m == nil {
		return IssuedToken{}, errors.New("Ed25519 access-token signer is not configured")
	}
	if ttl <= 0 || ttl > m.maximumTTL {
		return IssuedToken{}, errors.New("connection token lifetime must be positive and not exceed maximum token lifetime")
	}
	return m.issueAccessToken(userID, authVersion, jwt.ClaimStrings{ConnectionAudience}, ttl)
}

func (m *Manager) issueAccessToken(userID uint, authVersion uint64, audiences jwt.ClaimStrings, ttl time.Duration) (IssuedToken, error) {
	if m == nil || len(m.current) != ed25519.PrivateKeySize {
		return IssuedToken{}, errors.New("Ed25519 access-token signer is not configured")
	}
	if userID == 0 || authVersion == 0 {
		return IssuedToken{}, errors.New("user id and auth version must be non-zero")
	}
	now := m.now().UTC()
	jti, err := uuid.NewV7()
	if err != nil {
		return IssuedToken{}, fmt.Errorf("generate token id: %w", err)
	}
	if len(audiences) == 0 || ttl <= 0 || ttl > m.maximumTTL {
		return IssuedToken{}, errors.New("access token audience and valid lifetime are required")
	}
	expires := now.Add(ttl)
	claims := AccessClaims{
		UserID:      uint64(userID),
		TokenUse:    AccessTokenUse,
		Scope:       []string{ConnectScope},
		AuthVersion: authVersion,
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer:    m.issuer,
			Subject:   strconv.FormatUint(uint64(userID), 10),
			Audience:  append(jwt.ClaimStrings(nil), audiences...),
			ExpiresAt: jwt.NewNumericDate(expires),
			NotBefore: jwt.NewNumericDate(now),
			IssuedAt:  jwt.NewNumericDate(now),
			ID:        jti.String(),
		},
	}
	token := jwt.NewWithClaims(jwt.SigningMethodEdDSA, claims)
	token.Header["kid"] = m.currentID
	token.Header["typ"] = AccessTokenType
	signed, err := token.SignedString(m.current)
	if err != nil {
		return IssuedToken{}, fmt.Errorf("sign access token: %w", err)
	}
	if len(signed) > m.maxTokenSize {
		return IssuedToken{}, ErrTokenTooLarge
	}
	return IssuedToken{
		Token:       signed,
		JTI:         jti.String(),
		KeyID:       m.currentID,
		IssuedAt:    now.Unix(),
		ExpiresAt:   expires.Unix(),
		AuthVersion: authVersion,
	}, nil
}

func (m *Manager) VerifyAccessToken(tokenString string, options VerifyOptions) (*AccessClaims, error) {
	if m == nil {
		return nil, ErrTokenInvalid
	}
	if tokenString == "" || len(tokenString) > m.maxTokenSize {
		if len(tokenString) > m.maxTokenSize {
			return nil, ErrTokenTooLarge
		}
		return nil, ErrTokenInvalid
	}
	if options.Audience == "" {
		return nil, errors.New("expected audience is required")
	}
	claims := &AccessClaims{}
	parser := jwt.NewParser(
		jwt.WithValidMethods([]string{AlgorithmEdDSA}),
		jwt.WithIssuer(m.issuer),
		jwt.WithAudience(options.Audience),
		jwt.WithExpirationRequired(),
		jwt.WithIssuedAt(),
		jwt.WithLeeway(m.clockSkew),
		jwt.WithTimeFunc(m.now),
		jwt.WithStrictDecoding(),
	)
	token, err := parser.ParseWithClaims(tokenString, claims, func(token *jwt.Token) (interface{}, error) {
		if token.Method != jwt.SigningMethodEdDSA || token.Header["alg"] != AlgorithmEdDSA {
			return nil, ErrTokenInvalid
		}
		if typ, ok := token.Header["typ"].(string); !ok || typ != AccessTokenType {
			return nil, ErrTokenInvalid
		}
		keyID, ok := token.Header["kid"].(string)
		if !ok || keyID == "" {
			return nil, ErrUnknownKey
		}
		key, ok := m.public[keyID]
		if !ok {
			return nil, ErrUnknownKey
		}
		return key, nil
	})
	if err != nil || !token.Valid {
		return nil, fmt.Errorf("%w: %v", ErrTokenInvalid, err)
	}
	if claims.TokenUse != AccessTokenUse || claims.UserID == 0 || claims.AuthVersion == 0 {
		return nil, ErrTokenInvalid
	}
	if _, err := uuid.Parse(claims.ID); err != nil {
		return nil, ErrTokenInvalid
	}
	if claims.Subject != strconv.FormatUint(claims.UserID, 10) {
		return nil, ErrTokenInvalid
	}
	if options.RequiredScope != "" && !claims.HasScope(options.RequiredScope) {
		return nil, ErrTokenInvalid
	}
	if claims.IssuedAt == nil || claims.NotBefore == nil || claims.ExpiresAt == nil {
		return nil, ErrTokenInvalid
	}
	if claims.ExpiresAt.Time.Sub(claims.IssuedAt.Time) > m.maximumTTL || claims.ExpiresAt.Time.Before(claims.IssuedAt.Time) {
		return nil, ErrTokenInvalid
	}
	return claims, nil
}

func (m *Manager) JWKS() JWKS {
	if m == nil {
		return JWKS{Keys: []JWK{}}
	}
	keys := make([]JWK, 0, len(m.public))
	keys = append(keys, publicJWK(m.currentID, m.public[m.currentID]))
	previousIDs := make([]string, 0, len(m.public)-1)
	for keyID := range m.public {
		if keyID == m.currentID {
			continue
		}
		previousIDs = append(previousIDs, keyID)
	}
	sort.Strings(previousIDs)
	for _, keyID := range previousIDs {
		keys = append(keys, publicJWK(keyID, m.public[keyID]))
	}
	return JWKS{Keys: keys}
}

func contains(values []string, expected string) bool {
	for _, value := range values {
		if value == expected {
			return true
		}
	}
	return false
}

func loadPrivateKeyFile(path string) (ed25519.PrivateKey, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	if block, _ := pem.Decode(raw); block != nil {
		parsed, err := x509.ParsePKCS8PrivateKey(block.Bytes)
		if err != nil {
			return nil, ErrInvalidKeyFile
		}
		key, ok := parsed.(ed25519.PrivateKey)
		if !ok || len(key) != ed25519.PrivateKeySize {
			return nil, ErrInvalidKeyFile
		}
		return append(ed25519.PrivateKey(nil), key...), nil
	}
	decoded, err := decodeKeyMaterial(raw)
	if err != nil {
		return nil, err
	}
	switch len(decoded) {
	case ed25519.SeedSize:
		return ed25519.NewKeyFromSeed(decoded), nil
	case ed25519.PrivateKeySize:
		return append(ed25519.PrivateKey(nil), decoded...), nil
	default:
		return nil, ErrInvalidKeyFile
	}
}

func loadPublicKeyFile(path string) (ed25519.PublicKey, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	if block, _ := pem.Decode(raw); block != nil {
		parsed, err := x509.ParsePKIXPublicKey(block.Bytes)
		if err != nil {
			return nil, ErrInvalidKeyFile
		}
		key, ok := parsed.(ed25519.PublicKey)
		if !ok || len(key) != ed25519.PublicKeySize {
			return nil, ErrInvalidKeyFile
		}
		return append(ed25519.PublicKey(nil), key...), nil
	}
	decoded, err := decodeKeyMaterial(raw)
	if err != nil || len(decoded) != ed25519.PublicKeySize {
		return nil, ErrInvalidKeyFile
	}
	return append(ed25519.PublicKey(nil), decoded...), nil
}

func decodeKeyMaterial(raw []byte) ([]byte, error) {
	value := strings.TrimSpace(string(raw))
	if decoded, err := base64.RawStdEncoding.DecodeString(value); err == nil {
		return decoded, nil
	}
	if decoded, err := base64.StdEncoding.DecodeString(value); err == nil {
		return decoded, nil
	}
	if decoded, err := hex.DecodeString(value); err == nil {
		return decoded, nil
	}
	return nil, ErrInvalidKeyFile
}
