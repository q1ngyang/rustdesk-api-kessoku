package buildinfo

import (
	"runtime"
	"runtime/debug"
	"strings"
)

const (
	Component      = "kessoku"
	DatabaseSchema = uint(313)
)

// Version, GitCommit, and BuildTime are intentionally variables so release
// builds can replace them with deterministic values through -ldflags -X.
// The defaults keep local and test builds useful without requiring Git or a
// configured runtime environment.
var (
	Version   = "3.0.8"
	GitCommit = "unknown"
	BuildTime = "unknown"
)

type Details struct {
	Component      string `json:"component"`
	Version        string `json:"version"`
	DatabaseSchema uint   `json:"database_schema"`
	GitCommit      string `json:"git_commit"`
	BuildTime      string `json:"build_time"`
	GoVersion      string `json:"go_version"`
}

func Current() Details {
	commit := strings.TrimSpace(GitCommit)
	if commit == "" || commit == "unknown" {
		commit = vcsRevision()
	}
	if commit == "" {
		commit = "unknown"
	}
	buildTime := strings.TrimSpace(BuildTime)
	if buildTime == "" {
		buildTime = "unknown"
	}
	version := strings.TrimSpace(Version)
	if version == "" {
		version = "devel"
	}
	return Details{
		Component:      Component,
		Version:        version,
		DatabaseSchema: DatabaseSchema,
		GitCommit:      commit,
		BuildTime:      buildTime,
		GoVersion:      runtime.Version(),
	}
}

func vcsRevision() string {
	info, ok := debug.ReadBuildInfo()
	if !ok {
		return ""
	}
	revision := ""
	modified := false
	for _, setting := range info.Settings {
		switch setting.Key {
		case "vcs.revision":
			revision = setting.Value
		case "vcs.modified":
			modified = setting.Value == "true"
		}
	}
	if revision != "" && modified {
		return revision + "-dirty"
	}
	return revision
}
