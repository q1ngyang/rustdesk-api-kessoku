package model

type Peer struct {
	RowId    uint   `json:"row_id" gorm:"primaryKey;"`
	Id       string `json:"id"  gorm:"default:'';not null;index"`
	Cpu      string `json:"cpu"  gorm:"default:'';not null;"`
	Hostname string `json:"hostname"  gorm:"default:'';not null;"`
	Memory   string `json:"memory"  gorm:"default:'';not null;"`
	Os       string `json:"os"  gorm:"default:'';not null;"`
	Username string `json:"username"  gorm:"default:'';not null;"`
	Uuid     string `json:"uuid"  gorm:"default:'';not null;index"`
	Version  string `json:"version"  gorm:"default:'';not null;"`
	UserId   uint   `json:"user_id"  gorm:"default:0;not null;index"`
	User     *User  `json:"user,omitempty"`
	// IdentitySource records which trusted path established the ID/UUID pair.
	// It is deliberately independent from UserId: a network-discovered device
	// can later be claimed by an authenticated RustDesk account.
	IdentitySource     string `json:"identity_source" gorm:"size:32;default:'';not null;index"`
	IdentityVerifiedAt int64  `json:"identity_verified_at" gorm:"default:0;not null;index"`
	// LastSysinfoTime is not UpdatedAt. Heartbeats update the peer frequently,
	// while this timestamp tells the server when a complete client inventory
	// was last accepted and when a refresh should be requested.
	LastSysinfoTime int64  `json:"last_sysinfo_time" gorm:"default:0;not null;index"`
	LastOnlineTime  int64  `json:"last_online_time"  gorm:"default:0;not null;"`
	LastOnlineIp    string `json:"last_online_ip"  gorm:"default:'';not null;"`
	GroupId         uint   `json:"group_id"  gorm:"default:0;not null;index"`
	Alias           string `json:"alias" gorm:"default:'';not null;index"`
	TimeModel
}

const (
	PeerIdentitySourceLogin  = "native_login"
	PeerIdentitySourceStarry = "starry_registry"
	PeerIdentitySourceManual = "manual"
)

type PeerList struct {
	Peers []*Peer `json:"list"`
	Pagination
}
