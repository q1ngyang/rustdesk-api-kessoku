package model

type AdminScopeType string

const (
	AdminScopeTypeGroup      AdminScopeType = "group"
	AdminScopeTypeUser       AdminScopeType = "user"
	AdminScopeTypeCollection AdminScopeType = "address_book_collection"
	AdminScopeTypePeer       AdminScopeType = "peer"
)

func (t AdminScopeType) Valid() bool {
	return t == AdminScopeTypeGroup || t == AdminScopeTypeUser || t == AdminScopeTypeCollection || t == AdminScopeTypePeer
}

// AdminResourceScope assigns one concrete resource to one scoped administrator.
// Resource-specific existence and role constraints are validated transactionally
// by AdminScopeService; the typed unique key prevents duplicate grants.
type AdminResourceScope struct {
	IdModel
	AdminUserId uint           `json:"admin_user_id" gorm:"not null;uniqueIndex:idx_admin_resource_scope,priority:1;index"`
	ScopeType   AdminScopeType `json:"scope_type" gorm:"size:40;not null;uniqueIndex:idx_admin_resource_scope,priority:2;index:idx_scope_target,priority:1"`
	ScopeId     uint           `json:"scope_id" gorm:"not null;uniqueIndex:idx_admin_resource_scope,priority:3;index:idx_scope_target,priority:2"`
	TimeModel
}

type AdminScopeSet struct {
	GroupIds      []uint `json:"group_ids"`
	UserIds       []uint `json:"user_ids"`
	CollectionIds []uint `json:"collection_ids"`
	PeerIds       []uint `json:"peer_ids"`
}

type AdminScopeDetails struct {
	AdminUser   *User                    `json:"admin_user"`
	Scope       AdminScopeSet            `json:"scope"`
	Groups      []*Group                 `json:"groups"`
	Users       []*User                  `json:"users"`
	Collections []*AddressBookCollection `json:"collections"`
	Peers       []*Peer                  `json:"peers"`
}
