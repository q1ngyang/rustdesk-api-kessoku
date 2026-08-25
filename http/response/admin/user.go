package admin

import "github.com/q1ngyang/rustdesk-api-kessoku/v3/model"

type LoginPayload struct {
	Username   string         `json:"username"`
	Email      string         `json:"email"`
	Avatar     string         `json:"avatar"`
	Token      string         `json:"token"`
	RouteNames []string       `json:"route_names"`
	Nickname   string         `json:"nickname"`
	Role       model.UserRole `json:"role"`
}

func (lp *LoginPayload) FromUser(user *model.User) {
	lp.Username = user.Username
	lp.Email = user.Email
	lp.Avatar = user.Avatar
	lp.Nickname = user.Nickname
	lp.Role = user.EffectiveRole()
}

type UserOauthItem struct {
	Op     string `json:"op"`
	Status int    `json:"status"`
}

// GroupDirectory payloads intentionally expose only the identifiers and
// display names required by the address-book sharing UI.
type GroupDirectoryGroup struct {
	Id   uint   `json:"id"`
	Name string `json:"name"`
}

type GroupDirectoryUser struct {
	Id       uint   `json:"id"`
	Username string `json:"username"`
	GroupId  uint   `json:"group_id"`
}
