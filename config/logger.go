package config

import "errors"

type Logger struct {
	Path         string
	Level        string
	ReportCaller bool `mapstructure:"report-caller"`
	MaxSizeMB    int  `mapstructure:"max-size-mb"`
	MaxBackups   int  `mapstructure:"max-backups"`
	MaxAgeDays   int  `mapstructure:"max-age-days"`
	Compress     bool `mapstructure:"compress"`
	LocalTime    bool `mapstructure:"local-time"`
}

func (l Logger) Validate() error {
	if l.Path == "" {
		return nil
	}
	if l.MaxSizeMB < 0 || l.MaxSizeMB > 1024 {
		return errors.New("logger max-size-mb must not exceed 1024")
	}
	if l.MaxBackups < 0 || l.MaxBackups > 100 {
		return errors.New("logger max-backups must not exceed 100")
	}
	if l.MaxAgeDays < 0 || l.MaxAgeDays > 3650 {
		return errors.New("logger max-age-days must not exceed 3650")
	}
	return nil
}
