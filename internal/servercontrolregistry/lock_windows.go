//go:build windows

package servercontrolregistry

import (
	"context"
	"os"
)

// Windows builds serialize through an exclusive-create sentinel. Kessoku's
// supported server packages are Unix; this keeps cross-compilation fail-closed.
func withFileLock(_ context.Context, path string, operation func() error) error {
	file, err := os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
	if err != nil {
		return err
	}
	_ = file.Close()
	defer os.Remove(path)
	return operation()
}
