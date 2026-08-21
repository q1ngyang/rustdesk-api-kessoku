package model

// ServerCmd is retained only so upgrades can preserve/export the historical
// server_cmds table. No service or controller may list, create, or execute
// these values.
type ServerCmd struct {
	IdModel
	Cmd     string `json:"-" gorm:"default:'';not null;"`
	Alias   string `json:"-" gorm:"default:'';not null;"`
	Option  string `json:"-" gorm:"default:'';not null;"`
	Explain string `json:"-" gorm:"default:'';not null;"`
	Target  string `json:"-" gorm:"default:'';not null;"`
	TimeModel
}
