package model

import "gorm.io/gorm"

type UserRole string

const (
	UserRoleUser       UserRole = "user"
	UserRoleAdmin      UserRole = "admin"
	UserRoleSuperAdmin UserRole = "super_admin"
)

func (r UserRole) Valid() bool {
	return r == UserRoleUser || r == UserRoleAdmin || r == UserRoleSuperAdmin
}

type User struct {
	IdModel
	Username string `json:"username" gorm:"default:'';not null;uniqueIndex"`
	Email    string `json:"email" gorm:"default:'';not null;index"`
	// Email	string     	`json:"email" `
	Password string `json:"-" gorm:"default:'';not null;"`
	Nickname string `json:"nickname" gorm:"default:'';not null;"`
	Avatar   string `json:"avatar" gorm:"default:'';not null;"`
	// Presentation preferences are account scoped so the administration
	// console and a separately hosted WebClient can stay in sync without
	// attempting to share cookies across unrelated domains.
	PreferenceLanguage string   `json:"preference_language" gorm:"size:16;default:'';not null;"`
	PreferenceTheme    string   `json:"preference_theme" gorm:"size:16;default:'';not null;"`
	GroupId            uint     `json:"group_id" gorm:"default:0;not null;index"`
	Role               UserRole `json:"role" gorm:"size:24;default:'user';not null;index"`
	// IsAdmin is retained as a compatibility mirror for v2 clients and data.
	// Authorization decisions use Role after the v3 migration.
	IsAdmin *bool      `json:"is_admin" gorm:"default:0;not null;"`
	Status  StatusCode `json:"status" gorm:"default:1;not null;"`
	// AuthVersion invalidates every previously issued token when incremented.
	AuthVersion uint64 `json:"auth_version" gorm:"default:1;not null;"`
	Remark      string `json:"remark" gorm:"default:'';not null;"`
	TimeModel
}

// NormalizeRole converts legacy is_admin-only users and keeps the compatibility
// flag synchronized for older clients. A missing role plus is_admin=true keeps
// the historical meaning: unrestricted administrator (now super administrator).
func (u *User) NormalizeRole() {
	if !u.Role.Valid() {
		if u.IsAdmin != nil && *u.IsAdmin {
			u.Role = UserRoleSuperAdmin
		} else {
			u.Role = UserRoleUser
		}
	}
	isAdmin := u.Role == UserRoleAdmin || u.Role == UserRoleSuperAdmin
	u.IsAdmin = &isAdmin
}

func (u *User) EffectiveRole() UserRole {
	if u == nil {
		return UserRoleUser
	}
	if u.Role.Valid() {
		return u.Role
	}
	if u.IsAdmin != nil && *u.IsAdmin {
		return UserRoleSuperAdmin
	}
	return UserRoleUser
}

func (u *User) BeforeSave(_ *gorm.DB) error {
	u.NormalizeRole()
	return nil
}

// BeforeSave 钩子用于确保 email 字段有合理的默认值
//func (u *User) BeforeSave(tx *gorm.DB) (err error) {
//	// 如果 email 为空，设置为默认值
//	if u.Email == "" {
//		u.Email = fmt.Sprintf("%s@example.com", u.Username)
//	}
//	return nil
//}

type UserList struct {
	Users []*User `json:"list,omitempty"`
	Pagination
}

var UserRouteNames = []string{
	"MyTagList", "MyAddressBookList", "MyInfo", "MyAddressBookCollection", "MyPeer", "MyShareRecordList", "MyLoginLog",
}
var ScopedAdminRouteNames = []string{
	"Peer", "UserGroup", "UserList", "UserAdd", "UserEdit", "UserAddressBookName", "UserAddressBook", "UserTag",
}
var AdminRouteNames = []string{"*"}
