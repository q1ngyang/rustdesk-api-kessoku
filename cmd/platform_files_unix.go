//go:build !windows

package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"time"

	"golang.org/x/sys/unix"
)

func openPasswordFileSecure(path string) (*os.File, error) {
	fd, err := unix.Open(path, unix.O_RDONLY|unix.O_CLOEXEC|unix.O_NOFOLLOW, 0)
	if err != nil {
		return nil, err
	}
	file := os.NewFile(uintptr(fd), path)
	if file == nil {
		_ = unix.Close(fd)
		return nil, errors.New("open password file")
	}
	return file, nil
}

func withSQLiteFileLock(ctx context.Context, path string, operation func() error) error {
	fd, err := unix.Open(path, unix.O_RDWR|unix.O_CREAT|unix.O_CLOEXEC|unix.O_NOFOLLOW, 0o600)
	if err != nil {
		return fmt.Errorf("open SQLite migration lock: %w", err)
	}
	defer unix.Close(fd)
	var fileInfo unix.Stat_t
	if err := unix.Fstat(fd, &fileInfo); err != nil {
		return fmt.Errorf("inspect SQLite migration lock: %w", err)
	}
	if fileInfo.Mode&unix.S_IFMT != unix.S_IFREG {
		return errors.New("SQLite migration lock must be a regular file")
	}
	if fileInfo.Mode&0o077 != 0 {
		return errors.New("SQLite migration lock must not be accessible by group or other users")
	}
	if ctx == nil {
		ctx = context.Background()
	}
	ticker := time.NewTicker(50 * time.Millisecond)
	defer ticker.Stop()
	for {
		err = unix.Flock(fd, unix.LOCK_EX|unix.LOCK_NB)
		if err == nil {
			break
		}
		if !errors.Is(err, unix.EWOULDBLOCK) && !errors.Is(err, unix.EAGAIN) {
			return fmt.Errorf("acquire SQLite migration lock: %w", err)
		}
		select {
		case <-ctx.Done():
			return fmt.Errorf("acquire SQLite migration lock: %w", ctx.Err())
		case <-ticker.C:
		}
	}
	defer unix.Flock(fd, unix.LOCK_UN)
	return operation()
}
