package auth

import "encoding/base64"

type JWK struct {
	KeyType string `json:"kty"`
	Use     string `json:"use"`
	KeyID   string `json:"kid"`
	Curve   string `json:"crv"`
	Alg     string `json:"alg"`
	X       string `json:"x"`
}

type JWKS struct {
	Keys []JWK `json:"keys"`
}

func publicJWK(keyID string, publicKey []byte) JWK {
	return JWK{
		KeyType: "OKP",
		Use:     "sig",
		KeyID:   keyID,
		Curve:   "Ed25519",
		Alg:     AlgorithmEdDSA,
		X:       base64.RawURLEncoding.EncodeToString(publicKey),
	}
}
