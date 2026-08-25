package service

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"sort"

	internalAuth "github.com/q1ngyang/rustdesk-api-kessoku/v3/internal/auth"
	"github.com/q1ngyang/rustdesk-api-kessoku/v3/model"
	"gorm.io/gorm"
)

type authKeyringAuditMetadata struct {
	CurrentKeyID string   `json:"current_key_id"`
	KeyIDs       []string `json:"key_ids"`
	Digest       string   `json:"digest"`
}

// RecordAuthKeyringStartup persists only key IDs and a digest of public JWKS
// material. It is called after migrations so a keyring change is observable
// without ever storing private key material.
func RecordAuthKeyringStartup(manager *internalAuth.Manager) error {
	if manager == nil {
		return nil
	}
	if DB == nil {
		return errors.New("authentication keyring audit database is unavailable")
	}
	jwks := manager.JWKS()
	keyIDs := make([]string, 0, len(jwks.Keys))
	currentKeyID := ""
	for index, key := range jwks.Keys {
		if index == 0 {
			currentKeyID = key.KeyID
		}
		keyIDs = append(keyIDs, key.KeyID)
	}
	sort.Strings(keyIDs)
	publicJSON, err := json.Marshal(jwks)
	if err != nil {
		return err
	}
	digest := sha256.Sum256(publicJSON)
	metadata := authKeyringAuditMetadata{
		CurrentKeyID: currentKeyID,
		KeyIDs:       keyIDs,
		Digest:       "sha256:" + hex.EncodeToString(digest[:]),
	}

	action := "auth.keyring.loaded"
	last := &model.AdminAuditEvent{}
	query := DB.Where("target_type = ? AND action IN ? AND result = ?", "auth_keyring", []string{"auth.keyring.loaded", "auth.keyring.rotated"}, "success").Order("id DESC").First(last)
	if query.Error == nil {
		previous := authKeyringAuditMetadata{}
		if json.Unmarshal([]byte(last.Metadata), &previous) != nil || previous.Digest != metadata.Digest {
			action = "auth.keyring.rotated"
		}
	} else if !errors.Is(query.Error, gorm.ErrRecordNotFound) {
		return fmt.Errorf("read previous authentication keyring audit: %w", query.Error)
	}
	audit, err := beginSecurityAudit(nil, 0, "", action, "auth_keyring", currentKeyID, map[string]interface{}{
		"current_key_id": metadata.CurrentKeyID,
		"key_ids":        metadata.KeyIDs,
		"digest":         metadata.Digest,
	})
	if err != nil {
		return err
	}
	return finishSecurityAudit(audit, nil, "AUTH_KEYRING_AUDIT_FAILED")
}
