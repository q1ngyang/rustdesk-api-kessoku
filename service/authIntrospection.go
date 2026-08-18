package service

import (
	"errors"
	"strconv"

	internalAuth "github.com/q1ngyang/rustdesk-api-kessoku/v2/internal/auth"
)

type AuthIntrospectionService struct{}

type IntrospectionResult struct {
	Active      bool   `json:"active"`
	Subject     string `json:"sub,omitempty"`
	JTI         string `json:"jti,omitempty"`
	ExpiresAt   int64  `json:"exp,omitempty"`
	AuthVersion uint64 `json:"auth_version,omitempty"`
	Reason      string `json:"reason"`
}

func (s *AuthIntrospectionService) JWKS() (internalAuth.JWKS, error) {
	if Auth == nil {
		return internalAuth.JWKS{}, errors.New("Ed25519 auth profile is disabled")
	}
	return Auth.JWKS(), nil
}

func (s *AuthIntrospectionService) Introspect(token string) IntrospectionResult {
	if Auth == nil {
		return IntrospectionResult{Active: false, Reason: "inactive"}
	}
	user, userToken, claims, err := AllService.UserService.AuthenticateAccessToken(
		token,
		internalAuth.ConnectionAudience,
		internalAuth.ConnectScope,
	)
	if err != nil || claims == nil || user.Id == 0 || userToken.Id == 0 {
		// Detailed failure causes never cross the trust boundary and never include
		// the token. Metrics can classify this at the verifier/database layers.
		return IntrospectionResult{Active: false, Reason: "inactive"}
	}
	return IntrospectionResult{
		Active:      true,
		Subject:     strconv.FormatUint(claims.UserID, 10),
		JTI:         claims.ID,
		ExpiresAt:   claims.ExpiresAt.Unix(),
		AuthVersion: claims.AuthVersion,
		Reason:      "active",
	}
}
