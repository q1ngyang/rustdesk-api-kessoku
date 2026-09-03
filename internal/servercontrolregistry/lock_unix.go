//go:build !windows

package servercontrolregistry

import (
	"context"
	"errors"
	"fmt"
	"os"
	"time"

	"golang.org/x/sys/unix"
)

func withFileLock(ctx context.Context, path string, operation func() error) error {
	info, statErr := os.Lstat(path)
	created := errors.Is(statErr, os.ErrNotExist)
	if statErr == nil {
		if !info.Mode().IsRegular() || info.Mode()&os.ModeSymlink != 0 || info.Mode().Perm() != 0o600 || !ownedByCurrentUser(info) {
			return fmt.Errorf("%w: %s must be an owner-owned 0600 regular file", ErrUnsafePermissions, path)
		}
	} else if !created {
		return fmt.Errorf("inspect registry lock: %w", statErr)
	}
	fd, err := unix.Open(path, unix.O_RDWR|unix.O_CREAT|unix.O_CLOEXEC|unix.O_NOFOLLOW, 0o600)
	if err != nil {
		return fmt.Errorf("open registry lock: %w", err)
	}
	defer unix.Close(fd)
	if created {
		if err := os.Chmod(path, 0o600); err != nil {
			return fmt.Errorf("protect registry lock: %w", err)
		}
	}
	var stat unix.Stat_t
	if err := unix.Fstat(fd, &stat); err != nil || stat.Mode&unix.S_IFMT != unix.S_IFREG || stat.Mode&0o077 != 0 {
		return ErrUnsafePermissions
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
			return fmt.Errorf("acquire registry lock: %w", err)
		}
		select {
		case <-ctx.Done():
			return fmt.Errorf("acquire registry lock: %w", ctx.Err())
		case <-ticker.C:
		}
	}
	defer unix.Flock(fd, unix.LOCK_UN)
	return operation()
}
