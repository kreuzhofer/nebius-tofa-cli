package tofa_test

import (
	"errors"
	"github.com/kreuzhofer/nebius-tofa-cli/internal/tofa"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

type vault struct {
	values map[string]string
	fail   bool
}

func (v *vault) Set(ref, key string) error {
	if v.fail {
		return errors.New("locked")
	}
	v.values[ref] = key
	return nil
}
func (v *vault) Get(ref string) (string, error) {
	if v.fail {
		return "", errors.New("locked")
	}
	s, ok := v.values[ref]
	if !ok {
		return "", tofa.ErrMissing
	}
	return s, nil
}
func (v *vault) Delete(ref string) error {
	if v.fail {
		return errors.New("locked")
	}
	delete(v.values, ref)
	return nil
}

func TestLoginPersistsWithoutPuttingKeyInPreferences(t *testing.T) {
	dir := t.TempDir()
	v := &vault{values: map[string]string{}}
	s := tofa.Store{Dir: dir, Vault: v}
	if err := s.Login("project-one", "dummy-secret", "keyring"); err != nil {
		t.Fatal(err)
	}
	c, key, err := s.Credentials()
	if err != nil {
		t.Fatal(err)
	}
	if c.ProjectID != "project-one" || key != "dummy-secret" {
		t.Fatal("login was not restored")
	}
	b, err := os.ReadFile(filepath.Join(dir, "config.yml"))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(b), "dummy-secret") {
		t.Fatal("key leaked into preferences")
	}
	v.fail = true
	if err := s.Login("project-two", "replacement", "keyring"); err == nil {
		t.Fatal("accepted locked vault")
	}
	if _, err := os.Stat(filepath.Join(dir, "credentials.yml")); !os.IsNotExist(err) {
		t.Fatal("silent plaintext fallback")
	}
	v.fail = false
	c, key, err = s.Credentials()
	if err != nil || c.ProjectID != "project-one" || key != "dummy-secret" {
		t.Fatal("failed replacement lost previous login")
	}
	if err := s.Logout(); err != nil {
		t.Fatal(err)
	}
	if _, _, err := s.Credentials(); err == nil {
		t.Fatal("logout retained credential")
	}
	if err := s.Logout(); err != nil {
		t.Fatal("logout must be idempotent", err)
	}
	c, err = s.Config()
	if err != nil || c.ProjectID != "project-one" {
		t.Fatal("logout removed preferences")
	}
}

func TestFileFallbackAndFailedLogoutAreExplicit(t *testing.T) {
	v := &vault{values: map[string]string{}}
	dir := t.TempDir()
	s := tofa.Store{Dir: dir, Vault: v}
	if err := s.Login("project", "file-secret", "file"); err != nil {
		t.Fatal(err)
	}
	if _, key, err := s.Credentials(); err != nil || key != "file-secret" {
		t.Fatal("explicit file login failed", err)
	}
	if err := s.Login("project", "vault-secret", "keyring"); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(dir, "credentials.yml")); !os.IsNotExist(err) {
		t.Fatal("old plaintext key not removed")
	}
	v.fail = true
	if err := s.Logout(); err == nil {
		t.Fatal("inaccessible keyring reported logout success")
	}
	c, err := s.Config()
	if err != nil || c.Reference == "" {
		t.Fatal("failed logout lost recovery reference")
	}
	v.fail = false
	if err := s.Logout(); err != nil {
		t.Fatal(err)
	}
}

func TestConfigSaveFailureKeepsPreviousCredential(t *testing.T) {
	dir := t.TempDir()
	v := &vault{values: map[string]string{}}
	s := tofa.Store{Dir: dir, Vault: v}
	if err := s.Login("original", "old-key", "keyring"); err != nil {
		t.Fatal(err)
	}
	// Simulate a filesystem replacement during the native-store write, before config commit.
	// Move the original aside during sabotage so restoring it also preserves ACLs.
	var err error
	s.Vault = &sabotageVault{vault: v, path: filepath.Join(dir, "config.yml")}
	if err := s.Login("replacement", "new-key", "keyring"); err == nil {
		t.Fatal("expected configuration save failure")
	}
	if len(v.values) != 1 {
		t.Fatal("replacement key was not rolled back")
	}
	os.Remove(filepath.Join(dir, "config.yml"))
	if err = os.Rename(filepath.Join(dir, "config.yml.backup"), filepath.Join(dir, "config.yml")); err != nil {
		t.Fatal(err)
	}
	s.Vault = v
	c, key, err := s.Credentials()
	if err != nil || c.ProjectID != "original" || key != "old-key" {
		t.Fatal("old login not preserved", err)
	}
}

type sabotageVault struct {
	*vault
	path string
}

func (v *sabotageVault) Set(ref, key string) error {
	if err := v.vault.Set(ref, key); err != nil {
		return err
	}
	if err := os.Rename(v.path, v.path+".backup"); err != nil {
		return err
	}
	return os.Mkdir(v.path, 0700)
}
