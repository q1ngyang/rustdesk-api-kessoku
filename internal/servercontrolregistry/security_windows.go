//go:build windows

package servercontrolregistry

import "os"

func ownedByCurrentUser(_ os.FileInfo) bool { return true }
