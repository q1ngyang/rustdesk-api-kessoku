package admin

type ShareRecordQuery struct {
	UserId uint `json:"user_id" form:"user_id"`
	PageQuery
}

type ShareRecordForm struct {
	Id     uint `json:"id" form:"id"`
	UserId uint `json:"user_id" form:"user_id"`
}

type PeerShareRecordBatchDeleteForm struct {
	Ids []uint `json:"ids" binding:"required,max=1000,dive,gt=0"`
}
