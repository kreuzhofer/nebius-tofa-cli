//go:build !windows

package tofa

import (
	"fmt"
	"os"
)

func protect(path string, dir bool) error {
	if dir {
		return os.Chmod(path, 0700)
	}
	return os.Chmod(path, 0600)
}
func checkPrivate(path string, info os.FileInfo) error {
	if info.Mode().Perm()&0077 != 0 {
		return fmt.Errorf("%s is accessible to other users; restrict its permissions to 0600", path)
	}
	return nil
}
