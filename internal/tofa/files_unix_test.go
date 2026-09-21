//go:build !windows

package tofa_test

import (
	"bytes"
	"github.com/kreuzhofer/nebius-tofa-cli/internal/tofa"
	"os"
	"path/filepath"
	"testing"
)

func TestUnsafeFilesAndRecoverySymlinksAreRejected(t *testing.T) {
	dir := t.TempDir()
	v := &vault{values: map[string]string{}, availability: tofa.ErrVaultAbsent}
	s := tofa.Store{Dir: dir, Vault: v}
	a := tofa.App{Dir: dir, Vault: v, Out: &bytes.Buffer{}, Prompt: func(string, bool) (string, error) { return "synthetic", nil }}
	if err := a.Run([]string{"auth", "login"}); err != nil {
		t.Fatal(err)
	}
	for path, want := range map[string]os.FileMode{dir: 0700, filepath.Join(dir, "credentials.yml"): 0600} {
		info, err := os.Stat(path)
		if err != nil {
			t.Fatal(err)
		}
		if info.Mode().Perm() != want {
			t.Fatalf("mode %o, want %o", info.Mode().Perm(), want)
		}
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
