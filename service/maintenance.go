package service

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"time"

	databaseSchema "github.com/q1ngyang/rustdesk-api-kessoku/v3/internal/database"
	"github.com/q1ngyang/rustdesk-api-kessoku/v3/model"
	"github.com/q1ngyang/rustdesk-api-kessoku/v3/utils"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

const (
	MaintenanceCodeSelectorInvalid      = "MAINTENANCE_SELECTOR_INVALID"
	MaintenanceCodeSchemaMismatch       = "MAINTENANCE_SCHEMA_MISMATCH"
	MaintenanceCodeUserNotFound         = "MAINTENANCE_USER_NOT_FOUND"
	MaintenanceCodeConfirmationMismatch = "MAINTENANCE_CONFIRMATION_MISMATCH"
	MaintenanceCodePasswordInvalid      = "MAINTENANCE_PASSWORD_INVALID"
	MaintenanceCodeRecoveryFailed       = "MAINTENANCE_ADMIN_RECOVERY_FAILED"
	MaintenanceCodeTwoFactorResetFailed = "MAINTENANCE_TWO_FACTOR_RESET_FAILED"
)

type MaintenanceError struct {
	Code    string
	Message string
	Cause   error
}

func (e *MaintenanceError) Error() string {
	if e == nil {
		return "maintenance operation failed"
	}
	if e.Message != "" {
		return e.Message
	}
	return e.Code
}

func (e *MaintenanceError) Unwrap() error { return e.Cause }

type MaintenanceSelector struct {
	UserID          uint
	Username        string
	ConfirmUsername string
}

type RecoverAdministratorOptions struct {
	Selector       MaintenanceSelector
	RequestID      string
	Password       string
	ResetTwoFactor bool
}

type ResetTwoFactorOptions struct {
	Selector  MaintenanceSelector
	RequestID string
}

type MaintenanceResult struct {
	RequestID              string
	UserID                 uint
	Username               string
	AuthVersion            uint64
	PasswordReset          bool
	TwoFactorReset         bool
	TwoFactorWasConfigured bool
	LoginChallengesCleared int64
	ScopesCleared          int64
	SessionsRevoked        int64
}

func (us *UserService) RecoverAdministratorContext(ctx context.Context, options RecoverAdministratorOptions) (result MaintenanceResult, operationErr error) {
	result.RequestID = options.RequestID
	if err := validateMaintenanceSelector(options.Selector); err != nil {
		return result, err
	}
	if options.Password != "" && (len(options.Password) < 12 || len(options.Password) > 128) {
		return result, maintenanceError(MaintenanceCodePasswordInvalid, "password must contain 12 to 128 bytes", nil)
	}
	if _, err := databaseSchema.RequireCurrentSchema(DB.WithContext(ctx)); err != nil {
		return result, maintenanceError(MaintenanceCodeSchemaMismatch, "database schema must exactly match this binary", err)
	}

	targetType, targetID := maintenanceAuditTarget(options.Selector)
	event, err := beginSecurityAudit(ctx, 0, options.RequestID, "maintenance.admin.recovered", targetType, targetID, map[string]interface{}{
		"source":            "local_maintenance",
		"operation":         "administrator_recovery",
		"password_reset":    options.Password != "",
		"two_factor_reset":  options.ResetTwoFactor,
		"revocation_reason": "local_administrator_recovery",
	})
	if err != nil {
		return result, err
	}
	result.RequestID = event.RequestID
	defer func() {
		if operationErr == nil {
			return
		}
		failureCode := MaintenanceCodeRecoveryFailed
		var maintenanceErr *MaintenanceError
		if errors.As(operationErr, &maintenanceErr) && maintenanceErr.Code != "" {
			failureCode = maintenanceErr.Code
		}
		if auditErr := finishSecurityAudit(event, operationErr, failureCode); auditErr != nil {
			operationErr = errors.Join(operationErr, auditErr)
		}
	}()

	passwordHash := ""
	if options.Password != "" {
		passwordHash, err = utils.EncryptPassword(options.Password)
		if err != nil {
			return result, maintenanceError(MaintenanceCodeRecoveryFailed, "hash replacement password", err)
		}
	}
	tx := DB.WithContext(ctx).Begin()
	if tx.Error != nil {
		return result, maintenanceError(MaintenanceCodeRecoveryFailed, "begin administrator recovery transaction", tx.Error)
	}
	defer tx.Rollback()
	user, err := findMaintenanceUser(tx, options.Selector, MaintenanceCodeRecoveryFailed)
	if err != nil {
		return result, err
	}
	result.UserID, result.Username = user.Id, user.Username
	if user.Username != options.Selector.ConfirmUsername {
		return result, maintenanceError(MaintenanceCodeConfirmationMismatch, "confirm-username does not exactly match the stored username", nil)
	}
	if err := rewriteSecurityAuditIntent(tx, event, "maintenance.admin.recovered", map[string]interface{}{
		"source":            "local_maintenance",
		"operation":         "administrator_recovery",
		"target_user_id":    user.Id,
		"target_username":   user.Username,
		"previous_role":     us.Role(user),
		"previous_status":   user.Status,
		"password_reset":    options.Password != "",
		"two_factor_reset":  options.ResetTwoFactor,
		"revocation_reason": "local_administrator_recovery",
	}); err != nil {
		return result, maintenanceError(MaintenanceCodeRecoveryFailed, "specialize administrator recovery audit", err)
	}

	updates := map[string]interface{}{
		"status":   model.COMMON_STATUS_ENABLE,
		"role":     model.UserRoleSuperAdmin,
		"is_admin": true,
	}
	if passwordHash != "" {
		updates["password"] = passwordHash
		result.PasswordReset = true
	}
	if err := tx.Model(&model.User{}).Where("id = ?", user.Id).Updates(updates).Error; err != nil {
		return result, maintenanceError(MaintenanceCodeRecoveryFailed, "restore administrator role and status", err)
	}
	scopeDelete := tx.Where("admin_user_id = ?", user.Id).Delete(&model.AdminResourceScope{})
	if scopeDelete.Error != nil {
		return result, maintenanceError(MaintenanceCodeRecoveryFailed, "clear administrator resource scopes", scopeDelete.Error)
	}
	result.ScopesCleared = scopeDelete.RowsAffected
	if options.ResetTwoFactor {
		configured, resetErr := resetTwoFactorRows(tx, user.Id)
		if resetErr != nil {
			return result, maintenanceError(MaintenanceCodeRecoveryFailed, "clear two-factor configuration", resetErr)
		}
		result.TwoFactorReset = true
		result.TwoFactorWasConfigured = configured
	}
	challengeDelete := tx.Where("user_id = ?", user.Id).Delete(&model.TwoFactorLoginChallenge{})
	if challengeDelete.Error != nil {
		return result, maintenanceError(MaintenanceCodeRecoveryFailed, "clear login challenges", challengeDelete.Error)
	}
	result.LoginChallengesCleared = challengeDelete.RowsAffected
	revoked, err := us.bumpAuthVersionAndRevokeCount(tx, user.Id, "local_administrator_recovery")
	if err != nil {
		return result, maintenanceError(MaintenanceCodeRecoveryFailed, "revoke recovered administrator sessions", err)
	}
	result.SessionsRevoked = revoked
	result.AuthVersion = user.AuthVersion + 1
	if err := finishSecurityAuditSuccessTx(tx, event); err != nil {
		return result, maintenanceError(MaintenanceCodeRecoveryFailed, "complete administrator recovery audit", err)
	}
	if err := tx.Commit().Error; err != nil {
		return result, maintenanceError(MaintenanceCodeRecoveryFailed, "commit administrator recovery", err)
	}
	return result, nil
}

func (us *UserService) ResetTwoFactorMaintenanceContext(ctx context.Context, options ResetTwoFactorOptions) (result MaintenanceResult, operationErr error) {
	result.RequestID = options.RequestID
	if err := validateMaintenanceSelector(options.Selector); err != nil {
		return result, err
	}
	if _, err := databaseSchema.RequireCurrentSchema(DB.WithContext(ctx)); err != nil {
		return result, maintenanceError(MaintenanceCodeSchemaMismatch, "database schema must exactly match this binary", err)
	}
	targetType, targetID := maintenanceAuditTarget(options.Selector)
	event, err := beginSecurityAudit(ctx, 0, options.RequestID, "maintenance.two_factor.reset", targetType, targetID, map[string]interface{}{
		"source":            "local_maintenance",
		"operation":         "two_factor_reset",
		"revocation_reason": "local_two_factor_reset",
	})
	if err != nil {
		return result, err
	}
	result.RequestID = event.RequestID
	defer func() {
		if operationErr == nil {
			return
		}
		failureCode := MaintenanceCodeTwoFactorResetFailed
		var maintenanceErr *MaintenanceError
		if errors.As(operationErr, &maintenanceErr) && maintenanceErr.Code != "" {
			failureCode = maintenanceErr.Code
		}
		if auditErr := finishSecurityAudit(event, operationErr, failureCode); auditErr != nil {
			operationErr = errors.Join(operationErr, auditErr)
		}
	}()

	tx := DB.WithContext(ctx).Begin()
	if tx.Error != nil {
		return result, maintenanceError(MaintenanceCodeTwoFactorResetFailed, "begin two-factor reset transaction", tx.Error)
	}
	defer tx.Rollback()
	user, err := findMaintenanceUser(tx, options.Selector, MaintenanceCodeTwoFactorResetFailed)
	if err != nil {
		return result, err
	}
	result.UserID, result.Username = user.Id, user.Username
	if user.Username != options.Selector.ConfirmUsername {
		return result, maintenanceError(MaintenanceCodeConfirmationMismatch, "confirm-username does not exactly match the stored username", nil)
	}
	if err := rewriteSecurityAuditIntent(tx, event, "maintenance.two_factor.reset", map[string]interface{}{
		"source":            "local_maintenance",
		"operation":         "two_factor_reset",
		"target_user_id":    user.Id,
		"target_username":   user.Username,
		"revocation_reason": "local_two_factor_reset",
	}); err != nil {
		return result, maintenanceError(MaintenanceCodeTwoFactorResetFailed, "specialize two-factor reset audit", err)
	}
	configured, err := resetTwoFactorRows(tx, user.Id)
	if err != nil {
		return result, maintenanceError(MaintenanceCodeTwoFactorResetFailed, "clear two-factor configuration", err)
	}
	result.TwoFactorReset = true
	result.TwoFactorWasConfigured = configured
	challengeDelete := tx.Where("user_id = ?", user.Id).Delete(&model.TwoFactorLoginChallenge{})
	if challengeDelete.Error != nil {
		return result, maintenanceError(MaintenanceCodeTwoFactorResetFailed, "clear login challenges", challengeDelete.Error)
	}
	result.LoginChallengesCleared = challengeDelete.RowsAffected
	revoked, err := us.bumpAuthVersionAndRevokeCount(tx, user.Id, "local_two_factor_reset")
	if err != nil {
		return result, maintenanceError(MaintenanceCodeTwoFactorResetFailed, "revoke sessions after two-factor reset", err)
	}
	result.SessionsRevoked = revoked
	result.AuthVersion = user.AuthVersion + 1
	if err := finishSecurityAuditSuccessTx(tx, event); err != nil {
		return result, maintenanceError(MaintenanceCodeTwoFactorResetFailed, "complete two-factor reset audit", err)
	}
	if err := tx.Commit().Error; err != nil {
		return result, maintenanceError(MaintenanceCodeTwoFactorResetFailed, "commit two-factor reset", err)
	}
	return result, nil
}

func validateMaintenanceSelector(selector MaintenanceSelector) error {
	if (selector.UserID == 0) == (selector.Username == "") {
		return maintenanceError(MaintenanceCodeSelectorInvalid, "exactly one of user-id or username is required", nil)
	}
	if selector.ConfirmUsername == "" {
		return maintenanceError(MaintenanceCodeSelectorInvalid, "confirm-username is required", nil)
	}
	return nil
}

func findMaintenanceUser(tx *gorm.DB, selector MaintenanceSelector, failureCode string) (*model.User, error) {
	user := &model.User{}
	query := tx.Clauses(clause.Locking{Strength: "UPDATE"})
	if selector.UserID != 0 {
		query = query.Where("id = ?", selector.UserID)
	} else {
		query = query.Where("username = ?", selector.Username)
	}
	if err := query.First(user).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, maintenanceError(MaintenanceCodeUserNotFound, "user not found", err)
		}
		return nil, maintenanceError(failureCode, "query maintenance target", err)
	}
	return user, nil
}

func resetTwoFactorRows(tx *gorm.DB, userID uint) (bool, error) {
	var count int64
	if err := tx.Model(&model.UserTwoFactor{}).Where("user_id = ?", userID).Count(&count).Error; err != nil {
		return false, err
	}
	if err := tx.Where("user_id = ?", userID).Delete(&model.UserTwoFactor{}).Error; err != nil {
		return false, err
	}
	return count > 0, nil
}

func (us *UserService) bumpAuthVersionAndRevokeCount(tx *gorm.DB, userID uint, reason string) (int64, error) {
	result := tx.Model(&model.User{}).Where("id = ?", userID).
		UpdateColumn("auth_version", gorm.Expr("auth_version + ?", 1))
	if result.Error != nil {
		return 0, result.Error
	}
	if result.RowsAffected != 1 {
		return 0, errors.New("user not found while revoking sessions")
	}
	revoked := tx.Model(&model.UserToken{}).Where("user_id = ? AND revoked_at IS NULL", userID).
		Updates(map[string]interface{}{"revoked_at": time.Now().Unix(), "revoked_reason": reason})
	return revoked.RowsAffected, revoked.Error
}

func maintenanceAuditTarget(selector MaintenanceSelector) (string, string) {
	if selector.UserID != 0 {
		return "user", strconv.FormatUint(uint64(selector.UserID), 10)
	}
	return "user", selector.Username
}

func maintenanceError(code, message string, cause error) error {
	return &MaintenanceError{Code: code, Message: message, Cause: cause}
}

func MaintenanceErrorCode(err error, fallback string) string {
	var maintenanceErr *MaintenanceError
	if errors.As(err, &maintenanceErr) && maintenanceErr.Code != "" {
		return maintenanceErr.Code
	}
	return fallback
}

func MaintenanceErrorMessage(err error) string {
	var maintenanceErr *MaintenanceError
	if errors.As(err, &maintenanceErr) && maintenanceErr.Message != "" {
		return maintenanceErr.Message
	}
	if err == nil {
		return ""
	}
	return fmt.Sprintf("maintenance operation failed: %v", err)
}
