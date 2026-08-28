package service

import (
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha1"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base32"
	"encoding/base64"
	"encoding/binary"
	"errors"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/q1ngyang/rustdesk-api-kessoku/v3/config"
	"github.com/q1ngyang/rustdesk-api-kessoku/v3/model"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

var (
	ErrTwoFactorUnavailable = errors.New("two-factor authentication is unavailable")
	ErrTwoFactorRequired    = errors.New("two-factor authentication is required")
	ErrTwoFactorCode        = errors.New("invalid or replayed two-factor code")
	ErrTwoFactorChallenge   = errors.New("invalid or expired two-factor challenge")
)

type TwoFactorService struct {
	config config.TwoFactor
	aead   cipher.AEAD
}

type TwoFactorChallengeBinding struct {
	Client, DeviceID, UUID, Platform string
}

func NewTwoFactorService(cfg config.TwoFactor) *TwoFactorService {
	return &TwoFactorService{config: cfg}
}

func (s *TwoFactorService) Init() error {
	if !s.config.Enabled {
		return nil
	}
	key, err := loadOrCreateTwoFactorKey(s.config.KeyFile)
	if err != nil {
		return err
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return err
	}
	s.aead, err = cipher.NewGCM(block)
	return err
}

func loadOrCreateTwoFactorKey(path string) ([]byte, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return nil, fmt.Errorf("create two-factor key directory: %w", err)
	}
	info, err := os.Lstat(path)
	if errors.Is(err, os.ErrNotExist) {
		key := make([]byte, 32)
		if _, err := rand.Read(key); err != nil {
			return nil, err
		}
		file, createErr := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
		if createErr == nil {
			if _, err := file.Write(key); err != nil {
				_ = file.Close()
				_ = os.Remove(path)
				return nil, err
			}
			if err := file.Close(); err != nil {
				_ = os.Remove(path)
				return nil, err
			}
			return key, nil
		}
		if !errors.Is(createErr, os.ErrExist) {
			return nil, createErr
		}
		info, err = os.Lstat(path)
	}
	if err != nil {
		return nil, err
	}
	if !info.Mode().IsRegular() || info.Mode()&0o077 != 0 {
		return nil, errors.New("two-factor key file must be regular and mode 0600")
	}
	key, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	if len(key) != 32 {
		return nil, errors.New("two-factor key file must contain exactly 32 bytes")
	}
	return key, nil
}

func (s *TwoFactorService) Available() bool { return s != nil && s.config.Enabled && s.aead != nil }

func (s *TwoFactorService) EnabledForUser(userID uint) bool {
	if !s.Available() || userID == 0 {
		return false
	}
	var count int64
	DB.Model(&model.UserTwoFactor{}).Where("user_id = ? AND enabled = ?", userID, true).Count(&count)
	return count == 1
}

func (s *TwoFactorService) Status(userID uint) bool { return s.EnabledForUser(userID) }

func (s *TwoFactorService) BeginSetup(user *model.User) (string, string, error) {
	if !s.Available() {
		return "", "", ErrTwoFactorUnavailable
	}
	if user == nil || user.Id == 0 {
		return "", "", ErrAuthentication
	}
	if s.EnabledForUser(user.Id) {
		return "", "", errors.New("two-factor authentication is already enabled")
	}
	raw := make([]byte, 20)
	if _, err := rand.Read(raw); err != nil {
		return "", "", err
	}
	secret := base32.StdEncoding.WithPadding(base32.NoPadding).EncodeToString(raw)
	ciphertext, err := s.encrypt(user.Id, raw)
	if err != nil {
		return "", "", err
	}
	record := &model.UserTwoFactor{}
	if err := DB.Where("user_id = ?", user.Id).FirstOrCreate(record, &model.UserTwoFactor{UserID: user.Id}).Error; err != nil {
		return "", "", err
	}
	if err := DB.Model(record).Updates(map[string]interface{}{"pending_secret_ciphertext": ciphertext, "pending_expires_at": time.Now().Add(10 * time.Minute).Unix()}).Error; err != nil {
		return "", "", err
	}
	uri := url.URL{Scheme: "otpauth", Host: "totp", Path: "/" + s.config.Issuer + ":" + user.Username}
	query := uri.Query()
	query.Set("secret", secret)
	query.Set("issuer", s.config.Issuer)
	query.Set("algorithm", "SHA1")
	query.Set("digits", "6")
	query.Set("period", "30")
	uri.RawQuery = query.Encode()
	return secret, uri.String(), nil
}

func (s *TwoFactorService) ConfirmSetup(ctx context.Context, user *model.User, requestID, code string) (operationErr error) {
	if !s.Available() {
		return ErrTwoFactorUnavailable
	}
	if user == nil || user.Id == 0 {
		return ErrAuthentication
	}
	event, err := beginSecurityAudit(ctx, user.Id, requestID, "auth.two_factor.enabled", "user", strconv.FormatUint(uint64(user.Id), 10), map[string]interface{}{"revocation_reason": "two_factor_enabled"})
	if err != nil {
		return err
	}
	defer finalizeSecurityAudit(event, &operationErr, "AUTH_TWO_FACTOR_ENABLE_FAILED")
	tx := DB.WithContext(ctx).Begin()
	if tx.Error != nil {
		return tx.Error
	}
	defer tx.Rollback()
	record := &model.UserTwoFactor{}
	if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("user_id = ?", user.Id).First(record).Error; err != nil {
		return ErrTwoFactorCode
	}
	if record.Enabled || record.PendingExpiresAt < time.Now().Unix() || record.PendingSecretCiphertext == "" {
		return ErrTwoFactorCode
	}
	secret, err := s.decrypt(user.Id, record.PendingSecretCiphertext)
	if err != nil {
		return ErrTwoFactorCode
	}
	defer wipeBytes(secret)
	step, ok := verifyTOTP(secret, code, time.Now(), 0)
	if !ok {
		return ErrTwoFactorCode
	}
	if err := tx.Model(record).Updates(map[string]interface{}{"secret_ciphertext": record.PendingSecretCiphertext, "pending_secret_ciphertext": "", "pending_expires_at": 0, "enabled": true, "last_used_step": step}).Error; err != nil {
		return err
	}
	if err := AllService.UserService.bumpAuthVersionAndRevoke(tx, user.Id, "two_factor_enabled"); err != nil {
		return err
	}
	return tx.Commit().Error
}

func (s *TwoFactorService) Disable(ctx context.Context, user *model.User, requestID, code string) (operationErr error) {
	if !s.Available() {
		return ErrTwoFactorUnavailable
	}
	if user == nil || user.Id == 0 {
		return ErrAuthentication
	}
	event, err := beginSecurityAudit(ctx, user.Id, requestID, "auth.two_factor.disabled", "user", strconv.FormatUint(uint64(user.Id), 10), map[string]interface{}{"revocation_reason": "two_factor_disabled"})
	if err != nil {
		return err
	}
	defer finalizeSecurityAudit(event, &operationErr, "AUTH_TWO_FACTOR_DISABLE_FAILED")
	tx := DB.WithContext(ctx).Begin()
	if tx.Error != nil {
		return tx.Error
	}
	defer tx.Rollback()
	record := &model.UserTwoFactor{}
	if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("user_id = ? AND enabled = ?", user.Id, true).First(record).Error; err != nil {
		return ErrTwoFactorCode
	}
	secret, err := s.decrypt(user.Id, record.SecretCiphertext)
	if err != nil {
		return ErrTwoFactorCode
	}
	defer wipeBytes(secret)
	_, ok := verifyTOTP(secret, code, time.Now(), record.LastUsedStep)
	if !ok {
		return ErrTwoFactorCode
	}
	if err := tx.Model(record).Updates(map[string]interface{}{"secret_ciphertext": "", "pending_secret_ciphertext": "", "pending_expires_at": 0, "enabled": false, "last_used_step": 0}).Error; err != nil {
		return err
	}
	if err := AllService.UserService.bumpAuthVersionAndRevoke(tx, user.Id, "two_factor_disabled"); err != nil {
		return err
	}
	return tx.Commit().Error
}

func (s *TwoFactorService) CreateLoginChallenge(user *model.User, binding TwoFactorChallengeBinding) (string, error) {
	if !s.EnabledForUser(user.Id) {
		return "", ErrTwoFactorRequired
	}
	now := time.Now()
	// Challenges are short lived and contain no recoverable token, so remove
	// expired rows opportunistically before adding another one.
	if err := DB.Where("expires_at < ?", now.Unix()).Delete(&model.TwoFactorLoginChallenge{}).Error; err != nil {
		return "", err
	}
	random := make([]byte, 32)
	if _, err := rand.Read(random); err != nil {
		return "", err
	}
	token := base64.RawURLEncoding.EncodeToString(random)
	digest := sha256.Sum256([]byte(token))
	record := &model.TwoFactorLoginChallenge{TokenHash: fmt.Sprintf("%x", digest[:]), UserID: user.Id, Username: user.Username, Client: binding.Client, DeviceID: binding.DeviceID, UUID: binding.UUID, Platform: binding.Platform, ExpiresAt: now.Add(s.config.EffectiveChallengeTTL()).Unix()}
	if err := DB.Create(record).Error; err != nil {
		return "", err
	}
	return token, nil
}

func (s *TwoFactorService) CompleteLoginChallenge(token, username, code string, binding TwoFactorChallengeBinding) (*model.User, error) {
	if !s.Available() || len(token) < 32 || len(token) > 128 {
		return nil, ErrTwoFactorChallenge
	}
	digest := sha256.Sum256([]byte(token))
	tokenHash := fmt.Sprintf("%x", digest[:])
	tx := DB.Begin()
	if tx.Error != nil {
		return nil, tx.Error
	}
	defer tx.Rollback()
	challenge := &model.TwoFactorLoginChallenge{}
	if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("token_hash = ?", tokenHash).First(challenge).Error; err != nil {
		return nil, ErrTwoFactorChallenge
	}
	if challenge.UsedAt != 0 || challenge.ExpiresAt < time.Now().Unix() || challenge.Attempts >= 5 || challenge.Username != username || challenge.Client != binding.Client || challenge.DeviceID != binding.DeviceID || challenge.UUID != binding.UUID || challenge.Platform != binding.Platform {
		return nil, ErrTwoFactorChallenge
	}
	factor := &model.UserTwoFactor{}
	if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("user_id = ? AND enabled = ?", challenge.UserID, true).First(factor).Error; err != nil {
		return nil, ErrTwoFactorChallenge
	}
	secret, err := s.decrypt(challenge.UserID, factor.SecretCiphertext)
	if err != nil {
		return nil, ErrTwoFactorChallenge
	}
	defer wipeBytes(secret)
	step, ok := verifyTOTP(secret, code, time.Now(), factor.LastUsedStep)
	if !ok {
		if err := tx.Model(challenge).UpdateColumn("attempts", gorm.Expr("attempts + 1")).Error; err != nil {
			return nil, err
		}
		if err := tx.Commit().Error; err != nil {
			return nil, err
		}
		return nil, ErrTwoFactorCode
	}
	now := time.Now().Unix()
	if err := tx.Model(factor).UpdateColumn("last_used_step", step).Error; err != nil {
		return nil, err
	}
	if err := tx.Model(challenge).Updates(map[string]interface{}{"used_at": now, "attempts": gorm.Expr("attempts + 1")}).Error; err != nil {
		return nil, err
	}
	user := &model.User{}
	if err := tx.First(user, challenge.UserID).Error; err != nil {
		return nil, err
	}
	if err := tx.Commit().Error; err != nil {
		return nil, err
	}
	return user, nil
}

func (s *TwoFactorService) encrypt(userID uint, plaintext []byte) (string, error) {
	if !s.Available() {
		return "", ErrTwoFactorUnavailable
	}
	nonce := make([]byte, s.aead.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		return "", err
	}
	aad := []byte(fmt.Sprintf("kessoku:totp:user:%d", userID))
	sealed := s.aead.Seal(nonce, nonce, plaintext, aad)
	return base64.RawStdEncoding.EncodeToString(sealed), nil
}

func (s *TwoFactorService) decrypt(userID uint, encoded string) ([]byte, error) {
	ciphertext, err := base64.RawStdEncoding.DecodeString(encoded)
	if err != nil || len(ciphertext) <= s.aead.NonceSize() {
		return nil, ErrTwoFactorUnavailable
	}
	nonce := ciphertext[:s.aead.NonceSize()]
	return s.aead.Open(nil, nonce, ciphertext[s.aead.NonceSize():], []byte(fmt.Sprintf("kessoku:totp:user:%d", userID)))
}

func verifyTOTP(secret []byte, code string, now time.Time, lastUsed int64) (int64, bool) {
	if len(code) != 6 || strings.Trim(code, "0123456789") != "" {
		return 0, false
	}
	wanted, _ := strconv.Atoi(code)
	current := now.Unix() / 30
	for offset := int64(-1); offset <= 1; offset++ {
		step := current + offset
		if step <= lastUsed {
			continue
		}
		counter := make([]byte, 8)
		binary.BigEndian.PutUint64(counter, uint64(step))
		mac := hmac.New(sha1.New, secret)
		_, _ = mac.Write(counter)
		sum := mac.Sum(nil)
		index := sum[len(sum)-1] & 0x0f
		value := (binary.BigEndian.Uint32(sum[index:index+4]) & 0x7fffffff) % 1000000
		if subtle.ConstantTimeEq(int32(value), int32(wanted)) == 1 {
			return step, true
		}
	}
	return 0, false
}

func wipeBytes(value []byte) {
	for index := range value {
		value[index] = 0
	}
}
