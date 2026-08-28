package config

import (
	"errors"
	"path/filepath"
	"strings"
)

const defaultMaxImageBytes int64 = 1 << 20

// Media configures persistent, operator-owned image storage. The production
// container mounts /app/data as writable storage while the application image
// and bundled front ends remain read-only.
type Media struct {
	Directory     string `mapstructure:"directory"`
	MaxImageBytes int64  `mapstructure:"max-image-bytes"`
}

func (m *Media) Init() {
	if m.Directory == "" {
		m.Directory = "./data/media"
	}
	if m.MaxImageBytes == 0 {
		m.MaxImageBytes = defaultMaxImageBytes
	}
}

func (m Media) Validate() error {
	if m.Directory == "" {
		m.Directory = "./data/media"
	}
	if strings.TrimSpace(m.Directory) != m.Directory || strings.ContainsRune(m.Directory, '\x00') {
		return errors.New("media.directory must be a clean path")
	}
	cleaned := filepath.Clean(m.Directory)
	if cleaned == "." || cleaned == string(filepath.Separator) {
		return errors.New("media.directory must not be the working directory or filesystem root")
	}
	if m.MaxImageBytes == 0 {
		m.MaxImageBytes = defaultMaxImageBytes
	}
	if m.MaxImageBytes < 0 || m.MaxImageBytes > defaultMaxImageBytes {
		return errors.New("media.max-image-bytes must be between 1 byte and 1 MiB")
	}
	return nil
}
