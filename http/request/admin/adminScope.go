package admin

import "github.com/q1ngyang/rustdesk-api-kessoku/v3/model"

type AdminScopeUpdateForm struct {
	UserId        uint   `json:"user_id" validate:"required,gt=0"`
	GroupIds      []uint `json:"group_ids" binding:"max=1000,dive,gt=0"`
	UserIds       []uint `json:"user_ids" binding:"max=1000,dive,gt=0"`
	CollectionIds []uint `json:"collection_ids" binding:"max=1000,dive,gt=0"`
	PeerIds       []uint `json:"peer_ids" binding:"max=1000,dive,gt=0"`
}

func (f *AdminScopeUpdateForm) ToScopeSet() model.AdminScopeSet {
	return model.AdminScopeSet{
		GroupIds: f.GroupIds, UserIds: f.UserIds, CollectionIds: f.CollectionIds, PeerIds: f.PeerIds,
	}
}

type AdminScopeOptionQuery struct {
	PageQuery
	Type model.AdminScopeType `form:"type" binding:"required"`
	Q    string               `form:"q" binding:"max=128"`
}
