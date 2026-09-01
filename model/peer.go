package model

type Peer struct {
	RowId    uint   `json:"row_id" gorm:"primaryKey;"`
	Id       string `json:"id"  gorm:"default:'';not null;index;uniqueIndex:uidx_peers_id_v313"`
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
	LastSysinfoTime           int64  `json:"last_sysinfo_time" gorm:"default:0;not null;index"`
	LastOnlineTime            int64  `json:"last_online_time"  gorm:"default:0;not null;"`
	LastOnlineIp              string `json:"last_online_ip"  gorm:"default:'';not null;"`
	PresenceActivationEpoch   uint64 `json:"-" gorm:"default:0;not null;index"`
	PresenceActivationID      string `json:"-" gorm:"size:128;default:'';not null"`
	PresenceActivationRetired bool   `json:"-" gorm:"default:false;not null"`
	PresenceOnlineUntil       int64  `json:"presence_online_until" gorm:"default:0;not null;index"`
	PresenceV2SeenAt          int64  `json:"-" gorm:"default:0;not null;index"`
	Online                    bool   `json:"online" gorm:"-"`
	GroupId                   uint   `json:"group_id"  gorm:"default:0;not null;index"`
	Alias                     string `json:"alias" gorm:"default:'';not null;index"`
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

type PeerPresenceLease struct {
	RowID               uint   `json:"row_id" gorm:"primaryKey"`
	LeaseID             string `json:"lease_id" gorm:"size:32;not null;uniqueIndex"`
	PeerRowID           uint   `json:"peer_row_id" gorm:"not null;index:idx_presence_peer_expiry,priority:1;index:idx_presence_peer_activation,priority:1"`
	NetworkIdentityUUID string `json:"-" gorm:"size:256;not null;index"`
	ActivationEpoch     uint64 `json:"activation_epoch" gorm:"not null;index:idx_presence_peer_activation,priority:2"`
	ActivationID        string `json:"activation_id" gorm:"size:128;not null;index:idx_presence_peer_activation,priority:3"`
	TokenHash           string `json:"-" gorm:"size:64;not null;uniqueIndex"`
	StartedAt           int64  `json:"started_at" gorm:"not null"`
	RenewedAt           int64  `json:"renewed_at" gorm:"not null"`
	ExpiresAt           int64  `json:"expires_at" gorm:"not null;index:idx_presence_peer_expiry,priority:2"`
	EndedAt             int64  `json:"ended_at" gorm:"default:0;not null;index"`
	LastOnlineIP        string `json:"last_online_ip" gorm:"default:'';not null"`
	TimeModel
}
