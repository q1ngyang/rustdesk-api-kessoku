package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestPasswordFromFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "password")
	if err := os.WriteFile(path, []byte("correct horse battery staple\r\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	got, err := passwordFromFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if got != "correct horse battery staple" {
		t.Fatalf("password = %q", got)
	}
}

func TestPasswordFromFileRejectsUnsafeInputs(t *testing.T) {
	tests := []struct {
		name     string
		value    string
		mode     os.FileMode
		contains string
	}{
		{name: "short", value: "too-short", mode: 0o600, contains: "12 to 128"},
		{name: "long", value: strings.Repeat("x", 129), mode: 0o600, contains: "12 to 128"},
		{name: "group-readable", value: "long-enough-password", mode: 0o640, contains: "group or other"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "password")
			if err := os.WriteFile(path, []byte(tc.value), tc.mode); err != nil {
				t.Fatal(err)
			}
			_, err := passwordFromFile(path)
			if err == nil || !strings.Contains(err.Error(), tc.contains) {
				t.Fatalf("error = %v, want substring %q", err, tc.contains)
			}
		})
	}
}

func TestPasswordFromFileRejectsSymlink(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "target")
	if err := os.WriteFile(target, []byte("correct horse battery staple\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, "password")
	if err := os.Symlink(target, path); err != nil {
		t.Fatal(err)
	}
	_, err := passwordFromFile(path)
	if err == nil || !strings.Contains(err.Error(), "regular file") {
		t.Fatalf("error = %v, want regular file rejection", err)
	}
}
