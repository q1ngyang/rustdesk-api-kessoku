//go:build windows

package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"time"

	"golang.org/x/sys/windows"
)

func openPasswordFileSecure(path string) (*os.File, error) {
	// Lstat/SameFile checks in passwordFromFile run before any content read, so
	// a path replacement or symlink never becomes password input on Windows.
	return os.Open(path)
}

func withSQLiteFileLock(ctx context.Context, path string, operation func() error) error {
	before, beforeErr := os.Lstat(path)
	if beforeErr != nil && !errors.Is(beforeErr, os.ErrNotExist) {
		return fmt.Errorf("inspect SQLite migration lock: %w", beforeErr)
	}
	if beforeErr == nil && !before.Mode().IsRegular() {
		return errors.New("SQLite migration lock must be a regular file")
	}
	file, err := os.OpenFile(path, os.O_RDWR|os.O_CREATE, 0o600)
	if err != nil {
		return fmt.Errorf("open SQLite migration lock: %w", err)
	}
	defer file.Close()
	opened, err := file.Stat()
	if err != nil {
		return fmt.Errorf("inspect opened SQLite migration lock: %w", err)
	}
	current, err := os.Lstat(path)
	if err != nil || !current.Mode().IsRegular() || !os.SameFile(opened, current) {
		return errors.New("SQLite migration lock changed while opening")
	}

	if ctx == nil {
		ctx = context.Background()
	}
	handle := windows.Handle(file.Fd())
	overlapped := &windows.Overlapped{}
	ticker := time.NewTicker(50 * time.Millisecond)
	defer ticker.Stop()
	for {
		err = windows.LockFileEx(handle, windows.LOCKFILE_EXCLUSIVE_LOCK|windows.LOCKFILE_FAIL_IMMEDIATELY, 0, 1, 0, overlapped)
		if err == nil {
			break
		}
		if !errors.Is(err, windows.ERROR_LOCK_VIOLATION) {
			return fmt.Errorf("acquire SQLite migration lock: %w", err)
		}
		select {
		case <-ctx.Done():
			return fmt.Errorf("acquire SQLite migration lock: %w", ctx.Err())
		case <-ticker.C:
		}
	}
	defer windows.UnlockFileEx(handle, 0, 1, 0, overlapped)
	return operation()
}
