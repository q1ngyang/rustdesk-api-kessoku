package auth

import (
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
)

func TokenHash(token string) [sha256.Size]byte {
	return sha256.Sum256([]byte(token))
}

func TokenHashHex(token string) string {
	hash := TokenHash(token)
	return hex.EncodeToString(hash[:])
}

func ConstantTimeHashEqual(token, encodedHash string) bool {
	want, err := hex.DecodeString(encodedHash)
	if err != nil || len(want) != sha256.Size {
		return false
	}
	got := TokenHash(token)
	return subtle.ConstantTimeCompare(got[:], want) == 1
}
