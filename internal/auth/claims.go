package auth

import "github.com/golang-jwt/jwt/v5"

const (
	AlgorithmEdDSA     = "EdDSA"
	AccessTokenType    = "at+jwt"
	AccessTokenUse     = "access"
	APIAudience        = "kessoku-api"
	ConnectionAudience = "rustdesk-connect"
	ConnectScope       = "connect:initiate"
)

type AccessClaims struct {
	UserID      uint64   `json:"user_id"`
	TokenUse    string   `json:"token_use"`
	Scope       []string `json:"scope"`
	AuthVersion uint64   `json:"auth_version"`
	jwt.RegisteredClaims
}

func (c *AccessClaims) HasScope(required string) bool {
	for _, scope := range c.Scope {
		if scope == required {
			return true
		}
	}
	return false
}

type VerifyOptions struct {
	Audience      string
	RequiredScope string
}

type IssuedToken struct {
	Token       string
	JTI         string
	KeyID       string
	IssuedAt    int64
	ExpiresAt   int64
	AuthVersion uint64
}
