package admin

type AuditQuery struct {
	PeerId   string `form:"peer_id"`
	FromPeer string `form:"from_peer"`
	PageQuery
}

type AuditConnLogIds struct {
	Ids []uint `json:"ids" binding:"required,max=1000,dive,gt=0"`
}
type AuditFileLogIds struct {
	Ids []uint `json:"ids" binding:"required,max=1000,dive,gt=0"`
}
