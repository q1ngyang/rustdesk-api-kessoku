package service

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	mathrand "math/rand"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	internalAuth "github.com/q1ngyang/rustdesk-api-kessoku/v2/internal/auth"
	"github.com/q1ngyang/rustdesk-api-kessoku/v2/model"
	"github.com/q1ngyang/rustdesk-api-kessoku/v2/utils"
	"gorm.io/gorm"
)

type UserService struct {
}

// InfoById 根据用户id取用户信息
func (us *UserService) InfoById(id uint) *model.User {
	u := &model.User{}
	DB.Where("id = ?", id).First(u)
	return u
}

// InfoByUsername 根据用户名取用户信息
func (us *UserService) InfoByUsername(un string) *model.User {
	u := &model.User{}
	DB.Where("username = ?", un).First(u)
	return u
}

// InfoByEmail 根据邮箱取用户信息
func (us *UserService) InfoByEmail(email string) *model.User {
	u := &model.User{}
	DB.Where("email = ?", email).First(u)
	return u
}

// InfoByOpenid 根据openid取用户信息
func (us *UserService) InfoByOpenid(openid string) *model.User {
	u := &model.User{}
	DB.Where("openid = ?", openid).First(u)
	return u
}

// InfoByUsernamePassword 根据用户名密码取用户信息
func (us *UserService) InfoByUsernamePassword(username, password string) *model.User {
	if Config.Ldap.Enable {
		u, err := AllService.LdapService.Authenticate(username, password)
		if err == nil {
			return u
		}
		Logger.Errorf("LDAP authentication failed, %v", err)
		Logger.Warn("Fallback to local database")
	}
	u := &model.User{}
	DB.Where("username = ?", username).First(u)
	if u.Id == 0 {
		return u
	}
	ok, newHash, err := utils.VerifyPassword(u.Password, password)
	if err != nil || !ok {
		return &model.User{}
	}
	if newHash != "" {
		DB.Model(u).Update("password", newHash)
		u.Password = newHash
	}
	return u
}

var ErrAuthentication = errors.New("authentication failed")

// InfoByAccessToken validates the API audience and then checks authoritative
// database revocation state. Callers intentionally receive no detailed reason.
func (us *UserService) InfoByAccessToken(token string) (*model.User, *model.UserToken) {
	return us.InfoByAccessTokenContext(context.Background(), token)
}

func (us *UserService) InfoByAccessTokenContext(ctx context.Context, token string) (*model.User, *model.UserToken) {
	u, ut, _, err := us.AuthenticateAccessTokenContext(ctx, token, internalAuth.APIAudience, "")
	if err != nil {
		return &model.User{}, &model.UserToken{}
	}
	return u, ut
}

func (us *UserService) AuthenticateAccessToken(token, audience, requiredScope string) (*model.User, *model.UserToken, *internalAuth.AccessClaims, error) {
	return us.AuthenticateAccessTokenContext(context.Background(), token, audience, requiredScope)
}

func (us *UserService) AuthenticateAccessTokenContext(ctx context.Context, token, audience, requiredScope string) (*model.User, *model.UserToken, *internalAuth.AccessClaims, error) {
	u := &model.User{}
	ut := &model.UserToken{}
	if token == "" || len(token) > Config.Auth.EffectiveMaxTokenBytes() {
		return u, ut, nil, ErrAuthentication
	}
	var claims *internalAuth.AccessClaims
	strictVerificationFailed := false
	if Auth != nil {
		verified, err := Auth.VerifyAccessToken(token, internalAuth.VerifyOptions{
			Audience:      audience,
			RequiredScope: requiredScope,
		})
		if err != nil {
			if !Config.Auth.LegacyTokenReadEnabled {
				return u, ut, nil, ErrAuthentication
			}
			strictVerificationFailed = true
		} else {
			claims = verified
		}
	}

	hash := internalAuth.TokenHashHex(token)
	database := DB.WithContext(ctx)
	database.Where("token_hash = ?", hash).First(ut)
	if ut.Id == 0 && Config.Auth.LegacyTokenReadEnabled {
		database.Where("token = ?", token).First(ut)
	}
	if ut.Id == 0 {
		return u, ut, claims, ErrAuthentication
	}
	// Compatibility fallback is limited to credentials issued before the
	// EdDSA profile. A database row with a key ID is a strict JWT row; accepting
	// it after signature/kid/claims verification failed would make key removal
	// ineffective during the legacy overlap.
	if strictVerificationFailed && ut.Kid != "" {
		return u, ut, nil, ErrAuthentication
	}
	if ut.TokenHash != nil && !internalAuth.ConstantTimeHashEqual(token, *ut.TokenHash) {
		return u, ut, claims, ErrAuthentication
	}
	if ut.RevokedAt != nil || ut.ExpiredAt <= time.Now().Unix() {
		return u, ut, claims, ErrAuthentication
	}
	if claims != nil {
		if claims.UserID != uint64(ut.UserId) || ut.JTI == nil || claims.ID != *ut.JTI || claims.AuthVersion != ut.AuthVersion {
			return u, ut, claims, ErrAuthentication
		}
	}
	database.Where("id = ?", ut.UserId).First(u)
	if u.Id == 0 || !us.CheckUserEnable(u) || u.AuthVersion == 0 || ut.AuthVersion != u.AuthVersion {
		return &model.User{}, ut, claims, ErrAuthentication
	}
	return u, ut, claims, nil
}

// GenerateToken creates an EdDSA JWT when the signer is configured. The
// disabled-profile fallback is a cryptographically random opaque token, never
// the historical predictable MD5 construction.
func (us *UserService) GenerateToken(u *model.User) (internalAuth.IssuedToken, error) {
	if u == nil || u.Id == 0 {
		return internalAuth.IssuedToken{}, errors.New("cannot issue token for empty user")
	}
	if u.AuthVersion == 0 {
		u.AuthVersion = 1
		if err := DB.Model(u).Update("auth_version", u.AuthVersion).Error; err != nil {
			return internalAuth.IssuedToken{}, err
		}
	}
	if Auth != nil {
		return Auth.IssueAccessToken(u.Id, u.AuthVersion)
	}
	random := make([]byte, 32)
	if _, err := rand.Read(random); err != nil {
		return internalAuth.IssuedToken{}, fmt.Errorf("generate opaque access token: %w", err)
	}
	jti, err := uuid.NewV7()
	if err != nil {
		return internalAuth.IssuedToken{}, fmt.Errorf("generate opaque token id: %w", err)
	}
	now := time.Now().UTC()
	return internalAuth.IssuedToken{
		Token:       base64.RawURLEncoding.EncodeToString(random),
		JTI:         jti.String(),
		IssuedAt:    now.Unix(),
		ExpiresAt:   us.UserTokenExpireTimestamp(),
		AuthVersion: u.AuthVersion,
	}, nil
}

// Login 登录
func (us *UserService) Login(u *model.User, llog *model.LoginLog) *model.UserToken {
	issued, err := us.GenerateToken(u)
	if err != nil {
		Logger.Errorf("issue access token: %v", err)
		return nil
	}
	hash := internalAuth.TokenHashHex(issued.Token)
	jti := issued.JTI
	ut := &model.UserToken{
		UserId:      u.Id,
		DeviceUuid:  llog.Uuid,
		DeviceId:    llog.DeviceId,
		JTI:         &jti,
		Kid:         issued.KeyID,
		TokenHash:   &hash,
		AuthVersion: issued.AuthVersion,
		IssuedAt:    issued.IssuedAt,
		ExpiredAt:   issued.ExpiresAt,
	}
	tx := DB.Begin()
	if tx.Error != nil {
		Logger.Errorf("begin access token transaction: %v", tx.Error)
		return nil
	}
	if err := tx.Create(ut).Error; err != nil {
		tx.Rollback()
		Logger.Errorf("persist access token metadata: %v", err)
		return nil
	}
	llog.UserTokenId = ut.Id
	if err := tx.Create(llog).Error; err != nil {
		tx.Rollback()
		Logger.Errorf("persist login audit: %v", err)
		return nil
	}
	if err := tx.Commit().Error; err != nil {
		Logger.Errorf("commit access token transaction: %v", err)
		return nil
	}
	// Token is returned exactly once and was not present during persistence.
	ut.Token = issued.Token
	if llog.Uuid != "" {
		AllService.PeerService.UuidBindUserId(llog.DeviceId, llog.Uuid, u.Id)
	}
	return ut
}

// CurUser 获取当前用户
func (us *UserService) CurUser(c *gin.Context) *model.User {
	user, _ := c.Get("curUser")
	u, ok := user.(*model.User)
	if !ok {
		return nil
	}
	return u
}

func (us *UserService) List(page, pageSize uint, where func(tx *gorm.DB)) (res *model.UserList) {
	res = &model.UserList{}
	res.Page = int64(page)
	res.PageSize = int64(pageSize)
	tx := DB.Model(&model.User{})
	if where != nil {
		where(tx)
	}
	tx.Count(&res.Total)
	tx.Scopes(Paginate(page, pageSize))
	tx.Find(&res.Users)
	return
}

func (us *UserService) ListByIds(ids []uint) (res []*model.User) {
	DB.Where("id in ?", ids).Find(&res)
	return res
}

// ListByGroupId 根据组id取用户列表
func (us *UserService) ListByGroupId(groupId, page, pageSize uint) (res *model.UserList) {
	res = us.List(page, pageSize, func(tx *gorm.DB) {
		tx.Where("group_id = ?", groupId)
	})
	return
}

// ListIdsByGroupId 根据组id取用户id列表
func (us *UserService) ListIdsByGroupId(groupId uint) (ids []uint) {
	DB.Model(&model.User{}).Where("group_id = ?", groupId).Pluck("id", &ids)
	return ids

}

// ListIdAndNameByGroupId 根据组id取用户id和用户名列表
func (us *UserService) ListIdAndNameByGroupId(groupId uint) (res []*model.User) {
	DB.Model(&model.User{}).Where("group_id = ?", groupId).Select("id, username").Find(&res)
	return res
}

// CheckUserEnable 判断用户是否禁用
func (us *UserService) CheckUserEnable(u *model.User) bool {
	return u.Status == model.COMMON_STATUS_ENABLE
}

// Create 创建
func (us *UserService) Create(u *model.User) error {
	// The initial username should be formatted, and the username should be unique
	if us.IsUsernameExists(u.Username) {
		return errors.New("UsernameExists")
	}
	u.Username = us.formatUsername(u.Username)
	var err error
	u.Password, err = utils.EncryptPassword(u.Password)
	if err != nil {
		return err
	}
	res := DB.Create(u).Error
	return res
}

func (us *UserService) CreateContext(ctx context.Context, actorUserID uint, requestID string, u *model.User) (operationErr error) {
	if u == nil {
		return errors.New("cannot create empty user")
	}
	event, auditErr := beginSecurityAudit(ctx, actorUserID, requestID, "auth.user.created", "user", us.formatUsername(u.Username), map[string]interface{}{
		"group_id": u.GroupId,
		"is_admin": us.IsAdmin(u),
		"status":   u.Status,
	})
	if auditErr != nil {
		return auditErr
	}
	defer finalizeSecurityAudit(event, &operationErr, "AUTH_USER_CREATE_FAILED")
	return us.Create(u)
}

// GetUuidByToken 根据token和user取uuid
func (us *UserService) GetUuidByToken(u *model.User, token string) string {
	return us.getUuidByTokenContext(context.Background(), u, token)
}

func (us *UserService) getUuidByTokenContext(ctx context.Context, u *model.User, token string) string {
	ut := &model.UserToken{}
	hash := internalAuth.TokenHashHex(token)
	database := DB.WithContext(ctx)
	err := database.Where("user_id = ? AND token_hash = ?", u.Id, hash).First(ut).Error
	if err != nil && Config.Auth.LegacyTokenReadEnabled {
		err = database.Where("user_id = ? AND token = ?", u.Id, token).First(ut).Error
	}
	if err != nil {
		return ""
	}
	return ut.DeviceUuid
}

// Logout revokes one JTI. It does not increment the user-wide auth version.
func (us *UserService) Logout(u *model.User, token string) error {
	return us.LogoutContext(context.Background(), u, token)
}

func (us *UserService) LogoutContext(ctx context.Context, u *model.User, token string) error {
	if u == nil || u.Id == 0 {
		return nil
	}
	uuid := us.getUuidByTokenContext(ctx, u, token)
	now := time.Now().Unix()
	hash := internalAuth.TokenHashHex(token)
	database := DB.WithContext(ctx)
	tx := database.Model(&model.UserToken{}).
		Where("user_id = ? AND token_hash = ? AND revoked_at IS NULL", u.Id, hash).
		Updates(map[string]interface{}{"revoked_at": now, "revoked_reason": "logout"})
	if tx.Error == nil && tx.RowsAffected == 0 && Config.Auth.LegacyTokenReadEnabled {
		tx = database.Model(&model.UserToken{}).
			Where("user_id = ? AND token = ? AND revoked_at IS NULL", u.Id, token).
			Updates(map[string]interface{}{"revoked_at": now, "revoked_reason": "logout"})
	}
	err := tx.Error
	if err != nil {
		return err
	}
	if uuid != "" {
		AllService.PeerService.UuidUnbindUserId(uuid, u.Id)
	}
	return nil
}

// Delete 删除用户和oauth信息
func (us *UserService) Delete(u *model.User) error {
	return us.DeleteContext(context.Background(), 0, "", u)
}

func (us *UserService) DeleteContext(ctx context.Context, actorUserID uint, requestID string, u *model.User) (operationErr error) {
	if u == nil || u.Id == 0 {
		return errors.New("cannot delete empty user")
	}
	event, auditErr := beginSecurityAudit(ctx, actorUserID, requestID, "auth.user.deleted", "user", strconv.FormatUint(uint64(u.Id), 10), map[string]interface{}{
		"revocation_reason": "user_deleted",
	})
	if auditErr != nil {
		return auditErr
	}
	defer finalizeSecurityAudit(event, &operationErr, "AUTH_USER_DELETE_FAILED")

	tx := DB.WithContext(ctx).Begin()
	if tx.Error != nil {
		return tx.Error
	}
	defer tx.Rollback()
	if err := us.acquireEnabledAdminInvariant(tx); err != nil {
		return err
	}
	currentUser := &model.User{}
	if err := tx.First(currentUser, u.Id).Error; err != nil {
		return err
	}
	userCount, err := us.getAdminUserCountTx(tx)
	if err != nil {
		return err
	}
	if userCount <= 1 && us.IsAdmin(currentUser) && currentUser.Status == model.COMMON_STATUS_ENABLE {
		return errors.New("The last enabled admin user cannot be deleted")
	}
	if err := us.revokeUserTokens(tx, currentUser.Id, "user_deleted", time.Now().Unix()); err != nil {
		return err
	}
	// 删除用户
	if err := tx.Delete(currentUser).Error; err != nil {
		return err
	}
	// 删除关联的 OAuth 信息
	if err := tx.Where("user_id = ?", currentUser.Id).Delete(&model.UserThird{}).Error; err != nil {
		return err
	}
	if err := tx.Where("user_id = ?", currentUser.Id).Delete(&model.LdapIdentity{}).Error; err != nil {
		return err
	}
	//  删除关联的ab
	if err := tx.Where("user_id = ?", currentUser.Id).Delete(&model.AddressBook{}).Error; err != nil {
		return err
	}
	//  删除关联的abc
	if err := tx.Where("user_id = ?", currentUser.Id).Delete(&model.AddressBookCollection{}).Error; err != nil {
		return err
	}
	//  删除关联的abcr
	if err := tx.Where("user_id = ?", currentUser.Id).Delete(&model.AddressBookCollectionRule{}).Error; err != nil {
		return err
	}
	if err := tx.Commit().Error; err != nil {
		return err
	}
	// 删除关联的peer
	if err := AllService.PeerService.EraseUserId(currentUser.Id); err != nil {
		Logger.Warn("User deleted successfully, but failed to unlink peer.")
		return nil
	}
	return nil
}

// Update 更新
func (us *UserService) Update(u *model.User) error {
	return us.UpdateContext(context.Background(), 0, "", u)
}

func (us *UserService) UpdateContext(ctx context.Context, actorUserID uint, requestID string, u *model.User) (operationErr error) {
	if u == nil || u.Id == 0 {
		return errors.New("cannot update empty user")
	}
	if u.Status != model.COMMON_STATUS_ENABLE && u.Status != model.COMMON_STATUS_DISABLED {
		return errors.New("invalid user status")
	}
	event, auditErr := beginSecurityAudit(ctx, actorUserID, requestID, "auth.user.update_requested", "user", strconv.FormatUint(uint64(u.Id), 10), map[string]interface{}{
		"requested_is_admin": u.IsAdmin,
		"requested_status":   u.Status,
	})
	if auditErr != nil {
		return auditErr
	}
	failureCode := "AUTH_USER_UPDATE_FAILED"
	defer func() { finalizeSecurityAudit(event, &operationErr, failureCode) }()
	tx := DB.WithContext(ctx).Begin()
	if tx.Error != nil {
		return tx.Error
	}
	defer tx.Rollback()
	if err := us.acquireEnabledAdminInvariant(tx); err != nil {
		return err
	}
	currentUser := &model.User{}
	if err := tx.First(currentUser, u.Id).Error; err != nil {
		return err
	}
	currentIsAdmin := us.IsAdmin(currentUser)
	requestedIsAdmin := currentIsAdmin
	if u.IsAdmin != nil {
		requestedIsAdmin = *u.IsAdmin
	}
	roleChanged := currentIsAdmin != requestedIsAdmin
	statusChanged := currentUser.Status != u.Status
	willDisable := currentUser.Status == model.COMMON_STATUS_ENABLE && u.Status == model.COMMON_STATUS_DISABLED
	action := "auth.user.updated"
	if statusChanged {
		action = "auth.user.status_changed"
		failureCode = "AUTH_USER_STATUS_CHANGE_FAILED"
	}
	if willDisable && !roleChanged {
		action = "auth.user.disabled"
		failureCode = "AUTH_USER_DISABLE_FAILED"
	} else if roleChanged {
		action = "auth.user.role_changed"
		failureCode = "AUTH_USER_ROLE_CHANGE_FAILED"
	}
	if err := rewriteSecurityAuditIntent(tx, event, action, map[string]interface{}{
		"from_is_admin": currentIsAdmin,
		"to_is_admin":   requestedIsAdmin,
		"from_status":   currentUser.Status,
		"to_status":     u.Status,
	}); err != nil {
		return err
	}
	removesEnabledAdmin := currentIsAdmin && currentUser.Status == model.COMMON_STATUS_ENABLE && (!requestedIsAdmin || u.Status == model.COMMON_STATUS_DISABLED)
	if removesEnabledAdmin {
		adminCount, err := us.getAdminUserCountTx(tx)
		if err != nil {
			return err
		}
		if adminCount <= 1 {
			return errors.New("The last enabled admin user cannot be disabled or demoted")
		}
	}
	if err := tx.Model(&model.User{}).Where("id = ?", u.Id).Updates(u).Error; err != nil {
		return err
	}
	if willDisable || roleChanged {
		reason := "user_disabled"
		if roleChanged {
			reason = "authorization_changed"
		}
		if err := us.bumpAuthVersionAndRevoke(tx, u.Id, reason); err != nil {
			return err
		}
	}
	return tx.Commit().Error
}

// FlushToken invalidates every session using one atomic auth-version bump.
func (us *UserService) FlushToken(u *model.User) error {
	return us.FlushTokenContext(context.Background(), 0, "", u)
}

func (us *UserService) FlushTokenContext(ctx context.Context, actorUserID uint, requestID string, u *model.User) (operationErr error) {
	if u == nil || u.Id == 0 {
		return nil
	}
	event, auditErr := beginSecurityAudit(ctx, actorUserID, requestID, "auth.sessions.global_revoked", "user", strconv.FormatUint(uint64(u.Id), 10), map[string]interface{}{
		"revocation_reason": "global_logout",
	})
	if auditErr != nil {
		return auditErr
	}
	defer finalizeSecurityAudit(event, &operationErr, "AUTH_GLOBAL_REVOCATION_FAILED")
	tx := DB.Begin()
	if tx.Error != nil {
		return tx.Error
	}
	if err := us.bumpAuthVersionAndRevoke(tx, u.Id, "global_logout"); err != nil {
		tx.Rollback()
		return err
	}
	if err := tx.Commit().Error; err != nil {
		return err
	}
	u.AuthVersion++
	return nil
}

// FlushTokenByUuid revokes only the sessions for one device.
func (us *UserService) FlushTokenByUuid(uuid string) error {
	now := time.Now().Unix()
	return DB.Model(&model.UserToken{}).Where("device_uuid = ? AND revoked_at IS NULL", uuid).
		Updates(map[string]interface{}{"revoked_at": now, "revoked_reason": "device_removed"}).Error
}

// FlushTokenByUuids 清空token
func (us *UserService) FlushTokenByUuids(uuids []string) error {
	if len(uuids) == 0 {
		return nil
	}
	now := time.Now().Unix()
	return DB.Model(&model.UserToken{}).Where("device_uuid IN ? AND revoked_at IS NULL", uuids).
		Updates(map[string]interface{}{"revoked_at": now, "revoked_reason": "device_removed"}).Error
}

// UpdatePassword 更新密码
func (us *UserService) UpdatePassword(u *model.User, password string) error {
	return us.UpdatePasswordContext(context.Background(), 0, "", u, password)
}

func (us *UserService) UpdatePasswordContext(ctx context.Context, actorUserID uint, requestID string, u *model.User, password string) (operationErr error) {
	if u == nil || u.Id == 0 {
		return errors.New("cannot update password for empty user")
	}
	if len(password) < 12 || len(password) > 128 {
		return errors.New("password must contain 12 to 128 bytes")
	}
	event, auditErr := beginSecurityAudit(ctx, actorUserID, requestID, "auth.password.changed", "user", strconv.FormatUint(uint64(u.Id), 10), map[string]interface{}{
		"revocation_reason": "password_changed",
	})
	if auditErr != nil {
		return auditErr
	}
	defer finalizeSecurityAudit(event, &operationErr, "AUTH_PASSWORD_CHANGE_FAILED")

	var err error
	u.Password, err = utils.EncryptPassword(password)
	if err != nil {
		return err
	}
	tx := DB.Begin()
	if tx.Error != nil {
		return tx.Error
	}
	if err = tx.Model(u).Update("password", u.Password).Error; err != nil {
		tx.Rollback()
		return err
	}
	if err = us.bumpAuthVersionAndRevoke(tx, u.Id, "password_changed"); err != nil {
		tx.Rollback()
		return err
	}
	if err = tx.Commit().Error; err != nil {
		return err
	}
	u.AuthVersion++
	return nil
}

func (us *UserService) bumpAuthVersionAndRevoke(tx *gorm.DB, userID uint, reason string) error {
	result := tx.Model(&model.User{}).Where("id = ?", userID).
		UpdateColumn("auth_version", gorm.Expr("auth_version + ?", 1))
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected != 1 {
		return errors.New("user not found while revoking sessions")
	}
	return us.revokeUserTokens(tx, userID, reason, time.Now().Unix())
}

func (us *UserService) revokeUserTokens(tx *gorm.DB, userID uint, reason string, now int64) error {
	return tx.Model(&model.UserToken{}).Where("user_id = ? AND revoked_at IS NULL", userID).
		Updates(map[string]interface{}{"revoked_at": now, "revoked_reason": reason}).Error
}

// IsAdmin 是否管理员
func (us *UserService) IsAdmin(u *model.User) bool {
	return u != nil && u.IsAdmin != nil && *u.IsAdmin
}

// RouteNames
func (us *UserService) RouteNames(u *model.User) []string {
	if us.IsAdmin(u) {
		return model.AdminRouteNames
	}
	return model.UserRouteNames
}

// InfoByOauthId 根据oauth的name和openId取用户信息
func (us *UserService) InfoByOauthId(op string, openId string) *model.User {
	ut := AllService.OauthService.UserThirdInfo(op, openId)
	if ut.Id == 0 {
		return nil
	}
	u := us.InfoById(ut.UserId)
	if u.Id == 0 {
		return nil
	}
	return u
}

// RegisterByOauth 注册
func (us *UserService) RegisterByOauth(oauthUser *model.OauthUser, op string) (error, *model.User) {
	Lock.Lock("registerByOauth")
	defer Lock.UnLock("registerByOauth")
	ut := AllService.OauthService.UserThirdInfo(op, oauthUser.OpenId)
	if ut.Id != 0 {
		return nil, us.InfoById(ut.UserId)
	}
	err, oauthType := AllService.OauthService.GetTypeByOp(op)
	if err != nil {
		return err, nil
	}
	//check if this email has been registered
	email := oauthUser.Email
	// Existing local or LDAP identities may only be linked by an email
	// address that the upstream provider explicitly verified.
	if email != "" && oauthUser.VerifiedEmail {
		email = strings.ToLower(email)
		// update email to oauthUser, in case it contain upper case
		oauthUser.Email = email
		// call this, if find user by email, it will update the email to local database
		user, ldapErr := AllService.LdapService.GetUserInfoByEmailLocal(email)
		// If we enable ldap, and the error is not ErrLdapUserNotFound, return the error because we could not sure if the user is not found in ldap
		if !(errors.Is(ldapErr, ErrLdapNotEnabled) || errors.Is(ldapErr, ErrLdapUserNotFound) || ldapErr == nil) {
			return ldapErr, user
		}
		if user.Id == 0 {
			// this means the user is not found in ldap, maybe ldao is not enabled
			user = us.InfoByEmail(email)
		}
		if user.Id != 0 {
			ut.FromOauthUser(user.Id, oauthUser, oauthType, op)
			if err := DB.Create(ut).Error; err != nil {
				return err, nil
			}
			return nil, user
		}
	}

	tx := DB.Begin()
	if tx.Error != nil {
		return tx.Error, nil
	}
	ut = &model.UserThird{}
	ut.FromOauthUser(0, oauthUser, oauthType, op)
	// The initial username should be formatted
	username := us.formatUsername(oauthUser.Username)
	usernameUnique := us.GenerateUsernameByOauth(username)
	user := &model.User{
		Username: usernameUnique,
		GroupId:  1,
	}
	oauthUser.ToUser(user, false)
	if err := tx.Create(user).Error; err != nil || user.Id == 0 {
		tx.Rollback()
		if err != nil {
			return errors.Join(errors.New("OauthRegisterFailed"), err), user
		}
		return errors.New("OauthRegisterFailed"), user
	}
	ut.UserId = user.Id
	if err := tx.Create(ut).Error; err != nil {
		tx.Rollback()
		return errors.Join(errors.New("OauthRegisterFailed"), err), user
	}
	if err := tx.Commit().Error; err != nil {
		return errors.Join(errors.New("OauthRegisterFailed"), err), user
	}
	return nil, user
}

// GenerateUsernameByOauth 生成用户名
func (us *UserService) GenerateUsernameByOauth(name string) string {
	for us.IsUsernameExists(name) {
		name += strconv.Itoa(mathrand.Intn(10)) // Append a random digit (0-9)
	}
	return name
}

// UserThirdsByUserId
func (us *UserService) UserThirdsByUserId(userId uint) (res []*model.UserThird) {
	DB.Where("user_id = ?", userId).Find(&res)
	return res
}

func (us *UserService) UserThirdInfo(userId uint, op string) *model.UserThird {
	ut := &model.UserThird{}
	DB.Where("user_id = ? and op = ?", userId, op).First(ut)
	return ut
}

// FindLatestUserIdFromLoginLogByUuid 根据uuid和设备id查找最后登录的用户id
func (us *UserService) FindLatestUserIdFromLoginLogByUuid(uuid string, deviceId string) uint {
	llog := &model.LoginLog{}
	DB.Where("uuid = ? and device_id = ?", uuid, deviceId).Order("id desc").First(llog)
	return llog.UserId
}

// IsPasswordEmptyById 根据用户id判断密码是否为空，主要用于第三方登录的自动注册
func (us *UserService) IsPasswordEmptyById(id uint) bool {
	u := &model.User{}
	if DB.Where("id = ?", id).First(u).Error != nil {
		return false
	}
	return u.Password == ""
}

// IsPasswordEmptyByUsername 根据用户id判断密码是否为空，主要用于第三方登录的自动注册
func (us *UserService) IsPasswordEmptyByUsername(username string) bool {
	u := &model.User{}
	if DB.Where("username = ?", username).First(u).Error != nil {
		return false
	}
	return u.Password == ""
}

// IsPasswordEmptyByUser 判断密码是否为空，主要用于第三方登录的自动注册
func (us *UserService) IsPasswordEmptyByUser(u *model.User) bool {
	return us.IsPasswordEmptyById(u.Id)
}

// Register 注册, 如果用户名已存在则返回nil
func (us *UserService) Register(username string, email string, password string, status model.StatusCode) *model.User {
	if len(username) < 2 || len(username) > 32 || len(email) > 254 || len(password) < 12 || len(password) > 128 {
		return nil
	}
	u := &model.User{
		Username: username,
		Email:    email,
		Password: password,
		GroupId:  1,
		Status:   status,
	}
	err := us.Create(u)
	if err != nil {
		return nil
	}
	return u
}

func (us *UserService) TokenList(page uint, size uint, f func(tx *gorm.DB)) *model.UserTokenList {
	res := &model.UserTokenList{}
	res.Page = int64(page)
	res.PageSize = int64(size)
	tx := DB.Model(&model.UserToken{})
	if f != nil {
		f(tx)
	}
	tx.Count(&res.Total)
	tx.Scopes(Paginate(page, size))
	tx.Find(&res.UserTokens)
	return res
}

func (us *UserService) TokenInfoById(id uint) *model.UserToken {
	ut := &model.UserToken{}
	DB.Where("id = ?", id).First(ut)
	return ut
}

func (us *UserService) DeleteToken(l *model.UserToken) error {
	return us.DeleteTokenContext(context.Background(), 0, "", l)
}

func (us *UserService) DeleteTokenContext(ctx context.Context, actorUserID uint, requestID string, l *model.UserToken) (operationErr error) {
	if l == nil || l.Id == 0 {
		return nil
	}
	event, auditErr := beginSecurityAudit(ctx, actorUserID, requestID, "auth.session.admin_revoked", "user_token", strconv.FormatUint(uint64(l.Id), 10), map[string]interface{}{
		"user_id": l.UserId,
	})
	if auditErr != nil {
		return auditErr
	}
	defer finalizeSecurityAudit(event, &operationErr, "AUTH_SESSION_REVOCATION_FAILED")
	now := time.Now().Unix()
	return DB.Model(&model.UserToken{}).Where("id = ? AND revoked_at IS NULL", l.Id).
		Updates(map[string]interface{}{"revoked_at": now, "revoked_reason": "admin_revoked"}).Error
}

// Helper functions, used for formatting username
func (us *UserService) formatUsername(username string) string {
	username = strings.ReplaceAll(username, " ", "")
	username = strings.ToLower(username)
	return username
}

// Helper functions, getUserCount
func (us *UserService) getUserCount() int64 {
	var count int64
	DB.Model(&model.User{}).Count(&count)
	return count
}

// helper functions, getAdminUserCount
func (us *UserService) getAdminUserCount() int64 {
	count, _ := us.getAdminUserCountTx(DB)
	return count
}

func (us *UserService) getAdminUserCountTx(tx *gorm.DB) (int64, error) {
	var count int64
	err := tx.Model(&model.User{}).Where("is_admin = ? AND status = ?", true, model.COMMON_STATUS_ENABLE).Count(&count).Error
	return count, err
}

func (us *UserService) acquireEnabledAdminInvariant(tx *gorm.DB) error {
	result := tx.Model(&model.SecurityInvariantLock{}).
		Where("name = ?", "enabled-admin").
		UpdateColumn("generation", gorm.Expr("generation + ?", 1))
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected != 1 {
		return errors.New("enabled-administrator invariant lock is unavailable")
	}
	return nil
}

// UserTokenExpireTimestamp 生成用户token过期时间
func (us *UserService) UserTokenExpireTimestamp() int64 {
	exp := Config.App.TokenExpire
	if exp == 0 {
		//默认七天
		exp = 7 * 24 * time.Hour
	}
	return time.Now().Add(exp).Unix()
}

func (us *UserService) BatchDeleteUserToken(ids []uint) error {
	return us.BatchDeleteUserTokenContext(context.Background(), 0, "", ids)
}

func (us *UserService) BatchDeleteUserTokenContext(ctx context.Context, actorUserID uint, requestID string, ids []uint) (operationErr error) {
	if len(ids) == 0 {
		return nil
	}
	event, auditErr := beginSecurityAudit(ctx, actorUserID, requestID, "auth.session.batch_admin_revoked", "user_token", "batch", map[string]interface{}{
		"count": len(ids),
	})
	if auditErr != nil {
		return auditErr
	}
	defer finalizeSecurityAudit(event, &operationErr, "AUTH_SESSION_REVOCATION_FAILED")
	now := time.Now().Unix()
	return DB.Model(&model.UserToken{}).Where("id IN ? AND revoked_at IS NULL", ids).
		Updates(map[string]interface{}{"revoked_at": now, "revoked_reason": "admin_revoked"}).Error
}

// IsUsernameExists 判断用户名是否存在, it will check the internal database and LDAP(if enabled)
func (us *UserService) IsUsernameExists(username string) bool {
	return us.IsUsernameExistsLocal(username) || AllService.LdapService.IsUsernameExists(username)
}

func (us *UserService) IsUsernameExistsLocal(username string) bool {
	u := &model.User{}
	DB.Where("username = ?", username).First(u)
	return u.Id != 0
}

func (us *UserService) IsEmailExistsLdap(email string) bool {
	return AllService.LdapService.IsEmailExists(email)
}
