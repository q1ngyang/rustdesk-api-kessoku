package model

const (
	AuditActionNew   = "new"
	AuditActionClose = "close"
)

type AuditConn struct {
	IdModel
	UserId             uint   `json:"user_id" gorm:"default:0;not null;index"`
	Client             string `json:"client" gorm:"size:32;default:'';not null;index"`
	Action             string `json:"action" gorm:"default:'';not null;"`
	ConnId             int64  `json:"conn_id" gorm:"default:0;not null;index"`
	PeerId             string `json:"peer_id" gorm:"default:'';not null;index"`
	FromPeer           string `json:"from_peer" gorm:"default:'';not null;"`
	FromName           string `json:"from_name" gorm:"default:'';not null;"`
	Ip                 string `json:"ip" gorm:"default:'';not null;"`
	ControllerUsername string `json:"controller_username" gorm:"default:'';not null;index"`
	ControlledUsername string `json:"controlled_username" gorm:"default:'';not null;index"`
	ControlledIP       string `json:"controlled_ip" gorm:"default:'';not null;"`
	SessionId          string `json:"session_id" gorm:"default:'';not null;"`
	Type               int    `json:"type" gorm:"default:0;not null;"`
	Uuid               string `json:"uuid" gorm:"default:'';not null;"`
	CloseTime          int64  `json:"close_time" gorm:"default:0;not null;"`
	TimeModel
}

type AuditConnList struct {
	AuditConns []*AuditConn `json:"list"`
	Pagination
}

type AuditFile struct {
	IdModel
	FromPeer           string `json:"from_peer" gorm:"default:'';not null;index"`
	Info               string `json:"info" gorm:"default:'';not null;"`
	IsFile             bool   `json:"is_file" gorm:"default:0;not null;"`
	Path               string `json:"path" gorm:"default:'';not null;"`
	PeerId             string `json:"peer_id" gorm:"default:'';not null;index"`
	Type               int    `json:"type" gorm:"default:0;not null;"`
	Uuid               string `json:"uuid" gorm:"default:'';not null;"`
	Ip                 string `json:"ip" gorm:"default:'';not null;"`
	Num                int    `json:"num" gorm:"default:0;not null;"`
	FromName           string `json:"from_name" gorm:"default:'';not null;"`
	ControllerUsername string `json:"controller_username" gorm:"default:'';not null;index"`
	ControlledUsername string `json:"controlled_username" gorm:"default:'';not null;index"`
	ControlledIP       string `json:"controlled_ip" gorm:"default:'';not null;"`
	// ControlledPaths is derived from the official RustDesk audit path plus
	// info.files. Both source fields remain stored verbatim for audit fidelity.
	ControlledPaths []string `json:"controlled_paths,omitempty" gorm:"-"`
	TimeModel
}

type AuditFileList struct {
	AuditFiles []*AuditFile `json:"list"`
	Pagination
}
