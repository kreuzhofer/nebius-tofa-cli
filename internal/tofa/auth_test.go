package tofa_test

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"testing"

	"github.com/godbus/dbus/v5"
	"github.com/kreuzhofer/nebius-tofa-cli/internal/tofa"
	"github.com/zalando/go-keyring"
)

func TestLoginWithoutOverrideRetainsSavedFileBackend(t *testing.T) {
	v := &vault{values: map[string]string{}}
	s := tofa.Store{Dir: t.TempDir(), Vault: v}
	if err := s.Login("original", "old-key", "file"); err != nil {
		t.Fatal(err)
	}
	var out bytes.Buffer
	a := tofa.App{Dir: s.Dir, Vault: v, Out: &out, Prompt: func(_ string, secret bool) (string, error) {
		if secret {
			return "replacement-key", nil
		}
		return "replacement-project", nil
	}}
	if err := a.Run([]string{"auth", "login"}); err != nil {
		t.Fatal(err)
	}
	c, key, err := s.Credentials()
	if err != nil || c.Backend != "file" || key != "replacement-key" || c.ProjectID != "replacement-project" {
		t.Fatalf("saved backend not retained: config=%+v, error=%v", c, err)
	}
	if !strings.Contains(out.String(), "saved file") || strings.Contains(out.String(), "explicitly selected") {
		t.Fatalf("misleading selection notice: %s", &out)
	}
}

type vault struct {
	values       map[string]string
	fail         bool
	availability error
}

func (v *vault) Availability() error { return v.availability }

func TestFreshLoginAutomaticallyUsesFilesOnlyWhenVaultAbsent(t *testing.T) {
	v := &vault{values: map[string]string{}, availability: tofa.ErrVaultAbsent}
	dir := t.TempDir()
	var out bytes.Buffer
	a := tofa.App{Dir: dir, Vault: v, Out: &out, Prompt: func(_ string, secret bool) (string, error) {
		if secret {
			return "synthetic-key", nil
		}
		return "synthetic-project", nil
	}}
	if err := a.Run([]string{"auth", "login"}); err != nil {
		t.Fatal(err)
	}
	s := tofa.Store{Dir: dir, Vault: v}
	c, key, err := s.Credentials()
	if err != nil || c.Backend != "file" || key != "synthetic-key" {
		t.Fatalf("automatic file storage failed: config=%+v, error=%v", c, err)
	}
	for _, want := range []string{"automatically", "unencrypted", filepath.Join(dir, "credentials.yml")} {
		if !strings.Contains(out.String(), want) {
			t.Fatalf("missing %q in notice: %s", want, &out)
		}
	}
	if strings.Contains(out.String(), "synthetic-key") {
		t.Fatal("key leaked")
	}
}

func TestNativeVaultSelectionThroughLogin(t *testing.T) {
	// Replace only the OS keyring boundary: these tests never access real keys.
	t.Cleanup(keyring.MockInit)
	for _, tt := range []struct {
		name    string
		failure error
		backend string
	}{
		{"available", nil, "keyring"},
		{"unsupported", keyring.ErrUnsupportedPlatform, "file"},
		{"absent service", dbus.NewError("org.freedesktop.DBus.Error.ServiceUnknown", nil), "file"},
		{"absent service wire error", fmt.Errorf("lookup: %w", dbus.Error{Name: "org.freedesktop.DBus.Error.ServiceUnknown"}), "file"},
		{"missing macOS helper", &os.PathError{Op: "fork/exec", Path: "/usr/bin/security", Err: os.ErrNotExist}, "file"},
		{"locked", dbus.NewError("org.freedesktop.Secret.Error.IsLocked", nil), ""},
		{"denied", dbus.NewError("org.freedesktop.DBus.Error.AccessDenied", nil), ""},
		{"Windows denied", syscall.Errno(5), ""},                   // ERROR_ACCESS_DENIED
		{"Windows missing logon session", syscall.Errno(1312), ""}, // ERROR_NO_SUCH_LOGON_SESSION
		{"uncertain", errors.New("synthetic-private-detail"), ""},
		{"broken session bus", &os.PathError{Op: "dial", Path: "/missing/session-bus", Err: os.ErrNotExist}, ""},
		{"service activation failure", dbus.NewError("org.freedesktop.DBus.Error.Spawn.ExecFailed", nil), ""},
	} {
		t.Run(tt.name, func(t *testing.T) {
			keyring.MockInitWithError(tt.failure)
			var out bytes.Buffer
			dir := t.TempDir()
			prompted := false
			a := tofa.App{Dir: dir, Vault: tofa.NativeVault{}, Out: &out, Prompt: func(_ string, secret bool) (string, error) {
				prompted = true
				if secret {
					return "synthetic-key", nil
				}
				return "project", nil
			}}
			err := a.Run([]string{"auth", "login"})
			if tt.backend == "" {
				if err == nil || !strings.Contains(err.Error(), "--storage file") || prompted {
					t.Fatalf("expected actionable error before credential input, got %v, prompted=%v", err, prompted)
				}
				if strings.Contains(err.Error(), "synthetic-private-detail") {
					t.Fatal("raw vault error leaked")
				}
			} else {
				if err != nil {
					t.Fatal(err)
				}
				s := tofa.Store{Dir: dir, Vault: tofa.NativeVault{}}
				c, key, err := s.Credentials()
				if err != nil || c.Backend != tt.backend || key != "synthetic-key" {
					t.Fatalf("wrong saved login: config=%+v, error=%v", c, err)
				}
			}
			if tt.backend != "file" {
				if _, err := os.Stat(filepath.Join(dir, "credentials.yml")); !os.IsNotExist(err) {
					t.Fatal("unexpected plaintext file")
				}
			}
		})
	}
}

func TestLoginBackendOverridesAndFailures(t *testing.T) {
	for _, tt := range []struct {
		name, saved, override, want string
		availability                error
		writeFailure                bool
	}{
		{"fresh available", "", "", "keyring", nil, false},
		{"saved file despite uncertainty", "file", "", "file", errors.New("unknown"), true},
		{"saved vault disappears", "keyring", "", "", tofa.ErrVaultAbsent, true},
		{"explicit file despite locked vault", "keyring", "file", "file", errors.New("locked"), false},
		{"explicit vault replaces file", "file", "keyring", "keyring", nil, false},
		{"explicit absent vault", "", "keyring", "", tofa.ErrVaultAbsent, true},
		{"vault write fails after available probe", "", "", "", nil, true},
	} {
		t.Run(tt.name, func(t *testing.T) {
			v := &vault{values: map[string]string{}}
			s := tofa.Store{Dir: t.TempDir(), Vault: v}
			if tt.saved != "" {
				if err := s.Login("old-project", "old-key", tt.saved); err != nil {
					t.Fatal(err)
				}
			}
			v.availability, v.fail = tt.availability, tt.writeFailure
			var out bytes.Buffer
			a := tofa.App{Dir: s.Dir, Vault: v, Out: &out, Prompt: func(_ string, secret bool) (string, error) {
				if secret {
					return "new-key", nil
				}
				return "new-project", nil
			}}
			args := []string{"auth", "login"}
			if tt.override != "" {
				args = append(args, "--storage", tt.override)
			}
			err := a.Run(args)
			v.fail = false
			if tt.want == "" {
				if err == nil || !strings.Contains(err.Error(), "--storage file") {
					t.Fatalf("expected actionable failure, got %v", err)
				}
				c, err := s.Config()
				if err != nil || c.Backend != tt.saved {
					t.Fatalf("failed login changed backend: %+v, %v", c, err)
				}
				if tt.saved != "" {
					if _, key, err := s.Credentials(); err != nil || key != "old-key" {
						t.Fatal("failed login lost old credential", err)
					}
				}
			} else {
				if err != nil {
					t.Fatal(err)
				}
				c, key, err := s.Credentials()
				if err != nil || c.Backend != tt.want || key != "new-key" {
					t.Fatalf("wrong backend: %+v, %v", c, err)
				}
			}
			if tt.want != "file" && tt.saved != "file" {
				if _, err := os.Stat(filepath.Join(s.Dir, "credentials.yml")); !os.IsNotExist(err) {
					t.Fatal("unexpected plaintext file")
				}
			}
		})
	}
}

type failedNotice struct{}

func (failedNotice) Write([]byte) (int, error) { return 0, errors.New("output unavailable") }

func TestAutomaticFileSelectionRequiresVisibleNotice(t *testing.T) {
	v := &vault{values: map[string]string{}, availability: tofa.ErrVaultAbsent}
	dir := t.TempDir()
	a := tofa.App{Dir: dir, Vault: v, Out: failedNotice{}, Prompt: func(_ string, secret bool) (string, error) {
		if secret {
			return "synthetic-key", nil
		}
		return "project", nil
	}}
	if err := a.Run([]string{"auth", "login"}); err == nil {
		t.Fatal("saved unencrypted key without visible notice")
	}
	if _, err := os.Stat(filepath.Join(dir, "credentials.yml")); !os.IsNotExist(err) {
		t.Fatal("wrote plaintext after notice failure")
	}
}

func TestBackendReplacementKeepsFailedCleanupRecoverable(t *testing.T) {
	v := &vault{values: map[string]string{}}
	s := tofa.Store{Dir: t.TempDir(), Vault: v}
	if err := s.Login("old-project", "old-key", "keyring"); err != nil {
		t.Fatal(err)
	}
	v.fail = true
	var out bytes.Buffer
	a := tofa.App{Dir: s.Dir, Vault: v, Out: &out, Prompt: func(_ string, secret bool) (string, error) {
		if secret {
			return "new-key", nil
		}
		return "new-project", nil
	}}
	if err := a.Run([]string{"auth", "login", "--storage", "file"}); err == nil || !strings.Contains(err.Error(), "old credential cleanup pending") {
		t.Fatalf("cleanup failure not reported: %v", err)
	}
	c, key, err := s.Credentials()
	if err != nil || c.Backend != "file" || key != "new-key" {
		t.Fatal("new credential not usable after pending cleanup", err)
	}
	if err := s.Logout(); err == nil {
		t.Fatal("cleanup falsely succeeded while vault locked")
	}
	v.fail = false
	if err := s.Logout(); err != nil {
		t.Fatal("could not recover cleanup", err)
	}
	if len(v.values) != 0 {
		t.Fatal("old vault credential was orphaned")
	}
	if _, _, err := s.Credentials(); err == nil {
		t.Fatal("logout retained credentials")
	}
}

func TestSavedCredentialsDoNotMigrateWhenVaultAvailabilityChanges(t *testing.T) {
	for _, backend := range []string{"file", "keyring"} {
		t.Run(backend, func(t *testing.T) {
			v := &vault{values: map[string]string{}}
			s := tofa.Store{Dir: t.TempDir(), Vault: v}
			if err := s.Login("project", "saved-key", backend); err != nil {
				t.Fatal(err)
			}
			before, err := s.Config()
			if err != nil {
				t.Fatal(err)
			}
			v.availability = tofa.ErrVaultAbsent
			c, key, err := s.Credentials()
			if err != nil || c != before || key != "saved-key" {
				t.Fatal("changed saved login based on availability", err)
			}
		})
	}
}

func TestInvalidStorageOverrideFailsBeforeCredentialInput(t *testing.T) {
	for _, arg := range []string{"--storage=", "--storage=auto", "--storage=other"} {
		a := tofa.App{Dir: t.TempDir(), Out: &bytes.Buffer{}, Vault: &vault{}, Prompt: func(string, bool) (string, error) {
			t.Fatal("prompted before validating storage override")
			return "", nil
		}}
		if err := a.Run([]string{"auth", "login", arg}); err == nil {
			t.Fatal("invalid override accepted", arg)
		}
	}
}

func TestHelpExplainsCredentialSelection(t *testing.T) {
	var out bytes.Buffer
	a := tofa.App{Dir: t.TempDir(), Out: &out}
	if err := a.Run([]string{"--help"}); err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"saved storage choice", "absent or unsupported", "--storage keyring", "--storage file", "unencrypted"} {
		if !strings.Contains(out.String(), want) {
			t.Fatalf("help missing %q", want)
		}
	}
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
