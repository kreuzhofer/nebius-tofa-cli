//go:build !windows

package tofa_test

import (
	"github.com/kreuzhofer/nebius-tofa-cli/internal/tofa"
	"os"
	"path/filepath"
	"testing"
)

func TestUnsafeFilesAndRecoverySymlinksAreRejected(t *testing.T) {
	dir := t.TempDir()
	v := &vault{values: map[string]string{}}
	s := tofa.Store{Dir: dir, Vault: v}
	if err := s.Login("project", "dummy-key", "file"); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(filepath.Join(dir, "credentials.yml"), 0644); err != nil {
		t.Fatal(err)
	}
	if _, _, err := s.Credentials(); err == nil {
		t.Fatal("accepted publicly readable credentials")
	}
	other := t.TempDir()
	if err := os.Symlink(other, filepath.Join(dir, "keyring-refs")); err != nil {
		t.Fatal(err)
	}
	if err := s.Logout(); err == nil {
		t.Fatal("followed a recovery-directory symlink")
	}
}
