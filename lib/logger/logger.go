package logger

import (
	nested "github.com/antonfisher/nested-logrus-formatter"
	log "github.com/sirupsen/logrus"
	"gopkg.in/natefinch/lumberjack.v2"
	"io"
	"os"
)

const (
	DebugMode   = "debug"
	ReleaseMode = "release"
)

type Config struct {
	Path         string
	Level        string
	ReportCaller bool
	MaxSizeMB    int
	MaxBackups   int
	MaxAgeDays   int
	Compress     bool
	LocalTime    bool
}

func New(c *Config) *log.Logger {
	log.SetFormatter(&nested.Formatter{
		// HideKeys:        true,
		TimestampFormat: "[2006-01-02 15:04:05]",
		NoColors:        true,
		NoFieldsColors:  true,
		//FieldsOrder:     []string{"name", "age"},
	})

	// 日志文件
	f := c.Path
	var write io.Writer
	if f != "" {
		probe, err := os.OpenFile(f, os.O_WRONLY|os.O_CREATE|os.O_APPEND, 0644)
		if err != nil {
			panic("open log file fail!")
		}
		_ = probe.Close()
		maxSize := c.MaxSizeMB
		if maxSize <= 0 {
			maxSize = 20
		}
		maxBackups := c.MaxBackups
		if maxBackups <= 0 {
			maxBackups = 5
		}
		maxAge := c.MaxAgeDays
		if maxAge <= 0 {
			maxAge = 14
		}
		rotating := &lumberjack.Logger{Filename: f, MaxSize: maxSize, MaxBackups: maxBackups, MaxAge: maxAge, Compress: c.Compress, LocalTime: c.LocalTime}
		write = io.MultiWriter(rotating, os.Stdout)
	} else {
		write = os.Stdout
	}

	log.SetOutput(write)

	log.SetReportCaller(c.ReportCaller)

	level, err2 := log.ParseLevel(c.Level)
	if err2 != nil {
		level = log.DebugLevel
	}
	log.SetLevel(level)

	return log.StandardLogger()
}
