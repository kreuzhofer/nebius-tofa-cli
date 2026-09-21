package tofa

import (
	"bytes"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"

	"github.com/zalando/go-keyring"
	"gopkg.in/yaml.v3"
)

const Service = "io.nebius.tofa.prototype"

var ErrMissing = errors.New("credential not found")
var refPattern = regexp.MustCompile(`^[a-f0-9]{32}$`)

type Vault interface {
	Set(reference, key string) error
	Get(reference string) (string, error)
	Delete(reference string) error
}

type NativeVault struct{}

func (NativeVault) Set(ref, key string) error {
	if err := keyring.Set(Service, ref, key); err != nil {
		return errors.New("credential store write failed")
	}
	// The macOS security interactive command can exit successfully after an error.
	stored, err := keyring.Get(Service, ref)
	if err != nil || stored != key {
		return errors.New("credential store did not confirm the saved key")
	}
	return nil
}
func (NativeVault) Get(ref string) (string, error) {
	key, err := keyring.Get(Service, ref)
	if errors.Is(err, keyring.ErrNotFound) {
		return "", ErrMissing
	}
	if err != nil {
		return "", errors.New("credential store unavailable, locked or access denied")
	}
	return key, nil
}
func (NativeVault) Delete(ref string) error {
	err := keyring.Delete(Service, ref)
	if errors.Is(err, keyring.ErrNotFound) {
		return nil
	}
	if err != nil {
		return errors.New("credential store deletion failed; unlock the store and retry")
	}
	return nil
}

type Config struct {
	Version   int    `yaml:"version"`
	ProjectID string `yaml:"project_id"`
	Model     string `yaml:"model,omitempty"`
	Backend   string `yaml:"credential_backend,omitempty"`
	Reference string `yaml:"credential_ref,omitempty"`
}

type Store struct {
	Dir   string
	Vault Vault
}

func ConfigDir() (string, error) {
	if runtime.GOOS == "windows" {
		if p := os.Getenv("LOCALAPPDATA"); filepath.IsAbs(p) {
			return filepath.Join(p, "tofa"), nil
		}
		return "", errors.New("LOCALAPPDATA must be an absolute path")
	}
	if p := os.Getenv("XDG_CONFIG_HOME"); p != "" {
		if !filepath.IsAbs(p) {
			return "", errors.New("XDG_CONFIG_HOME must be absolute")
		}
		return filepath.Join(p, "tofa"), nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".config", "tofa"), nil
}

func validText(s string, limit int) bool {
	if len(s) == 0 || len(s) > limit {
		return false
	}
	for _, r := range s {
		if r < 33 || r > 126 {
			return false
		}
	}
	return true
}

func (s Store) Config() (Config, error) {
	c := Config{Version: 1}
	data, err := readPrivate(filepath.Join(s.Dir, "config.yml"))
	if errors.Is(err, os.ErrNotExist) {
		return c, nil
	}
	if err != nil {
		return c, err
	}
	dec := yaml.NewDecoder(bytes.NewReader(data))
	dec.KnownFields(true)
	if err = dec.Decode(&c); err != nil {
		return c, errors.New("invalid config.yml; expected the documented configuration schema")
	}
	var extra any
	if err = dec.Decode(&extra); err != io.EOF {
		return c, errors.New("config.yml must contain one YAML document")
	}
	if c.Version != 1 {
		return c, errors.New("unsupported configuration version")
	}
	if c.ProjectID != "" && !validText(c.ProjectID, 256) {
		return c, errors.New("invalid project_id")
	}
	if c.Model != "" && !validText(c.Model, 512) {
		return c, errors.New("invalid model")
	}
	if (c.Backend == "") != (c.Reference == "") {
		return c, errors.New("incomplete credential reference")
	}
	if c.Backend != "" && c.Backend != "keyring" && c.Backend != "file" {
		return c, errors.New("unknown credential backend")
	}
	if c.Reference != "" && !refPattern.MatchString(c.Reference) {
		return c, errors.New("invalid credential reference")
	}
	return c, nil
}

func (s Store) save(c Config) error {
	b, err := yaml.Marshal(c)
	if err != nil {
		return err
	}
	return writePrivate(filepath.Join(s.Dir, "config.yml"), b)
}
func (s Store) files() (map[string]string, error) {
	values := map[string]string{}
	b, err := readPrivate(filepath.Join(s.Dir, "credentials.yml"))
	if errors.Is(err, os.ErrNotExist) {
		return values, nil
	}
	if err != nil {
		return nil, err
	}
	if yaml.Unmarshal(b, &values) != nil {
		return nil, errors.New("invalid credentials.yml")
	}
	return values, nil
}
func (s Store) saveFiles(values map[string]string) error {
	b, err := yaml.Marshal(values)
	if err != nil {
		return err
	}
	return writePrivate(filepath.Join(s.Dir, "credentials.yml"), b)
}
func (s Store) Credentials() (Config, string, error) {
	c, err := s.Config()
	if err != nil {
		return c, "", err
	}
	if c.Reference == "" {
		return c, "", errors.New("not logged in; run tofa auth login")
	}
	var key string
	if c.Backend == "keyring" {
		key, err = s.Vault.Get(c.Reference)
	} else {
		var values map[string]string
		values, err = s.files()
		key = values[c.Reference]
	}
	if err != nil {
		return c, "", err
	}
	if !validText(key, 2048) {
		return c, "", errors.New("saved key missing or invalid; run tofa auth login")
	}
	return c, key, nil
}

// Every native key has a nonsecret recovery marker. A crash between storing a key
// and updating config therefore leaves enough information for logout or purge.
func (s Store) Login(project, key, backend string) error {
	if !validText(project, 256) {
		return errors.New("project ID must be nonempty printable text without spaces")
	}
	if !validText(key, 2048) {
		return errors.New("API key must be 1–2048 printable ASCII characters without spaces")
	}
	if backend != "keyring" && backend != "file" {
		return errors.New("choose keyring or explicitly opt into file storage")
	}
	unlock, err := s.lock()
	if err != nil {
		return err
	}
	defer unlock()
	old, err := s.Config()
	if err != nil {
		return err
	}
	id := make([]byte, 16)
	if _, err = rand.Read(id); err != nil {
		return err
	}
	ref := hex.EncodeToString(id)
	if backend == "keyring" {
		if err = privateDir(filepath.Join(s.Dir, "keyring-refs")); err != nil {
			return err
		}
		if err = writePrivate(filepath.Join(s.Dir, "keyring-refs", ref), []byte("tofa credential reference\n")); err != nil {
			return err
		}
		if err = s.Vault.Set(ref, key); err != nil {
			return fmt.Errorf("%w; no plaintext fallback was used (explicit option: tofa auth login --storage file)", err)
		}
	} else {
		values, e := s.files()
		if e != nil {
			return e
		}
		values[ref] = key
		if err = s.saveFiles(values); err != nil {
			return err
		}
	}
	c := old
	c.ProjectID = project
	c.Backend = backend
	c.Reference = ref
	if err = s.save(c); err != nil {
		cleanup := s.remove(backend, ref)
		if cleanup != nil {
			return errors.New("config save failed; previous login retained; new credential cleanup pending—run auth logout when storage is available")
		}
		return fmt.Errorf("config save failed; previous login retained: %w", err)
	}
	if old.Reference != "" {
		if err = s.remove(old.Backend, old.Reference); err != nil {
			return fmt.Errorf("new login saved; old credential cleanup pending: %w", err)
		}
	}
	return nil
}
func (s Store) remove(backend, ref string) error {
	if !refPattern.MatchString(ref) {
		return errors.New("invalid recovery reference")
	}
	if backend == "keyring" {
		if err := s.Vault.Delete(ref); err != nil && !errors.Is(err, ErrMissing) {
			return err
		}
		err := os.Remove(filepath.Join(s.Dir, "keyring-refs", ref))
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}
		return err
	}
	values, err := s.files()
	if err != nil {
		return err
	}
	delete(values, ref)
	if len(values) == 0 {
		err = os.Remove(filepath.Join(s.Dir, "credentials.yml"))
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}
		return err
	}
	return s.saveFiles(values)
}
func (s Store) Logout() error {
	unlock, err := s.lock()
	if err != nil {
		return err
	}
	defer unlock()
	c, err := s.Config()
	if err != nil {
		return err
	}
	recovery := filepath.Join(s.Dir, "keyring-refs")
	if info, e := os.Lstat(recovery); e == nil {
		if !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
			return errors.New("refusing non-directory credential recovery path")
		}
	} else if !errors.Is(e, os.ErrNotExist) {
		return e
	}
	entries, err := os.ReadDir(recovery)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	for _, entry := range entries {
		if entry.IsDir() || !refPattern.MatchString(entry.Name()) {
			return errors.New("unexpected credential recovery entry; cleanup stopped")
		}
		if err = s.remove("keyring", entry.Name()); err != nil {
			return err
		}
	}
	if c.Backend == "keyring" {
		if err = s.remove("keyring", c.Reference); err != nil {
			return err
		}
	}
	if err = os.Remove(filepath.Join(s.Dir, "credentials.yml")); err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	c.Backend = ""
	c.Reference = ""
	return s.save(c)
}
func (s Store) lock() (func(), error) {
	if err := privateDir(s.Dir); err != nil {
		return nil, err
	}
	p := filepath.Join(s.Dir, ".auth-lock")
	if err := os.Mkdir(p, 0700); err != nil {
		return nil, errors.New("authentication operation already active; if it crashed, remove the empty .auth-lock directory and retry")
	}
	return func() { _ = os.Remove(p) }, nil
}

func privateDir(path string) error {
	if err := os.MkdirAll(path, 0700); err != nil {
		return err
	}
	info, err := os.Lstat(path)
	if err != nil {
		return err
	}
	if !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
		return errors.New("configuration directory must be a real directory")
	}
	return protect(path, true)
}
func readPrivate(path string) ([]byte, error) {
	info, err := os.Lstat(path)
	if err != nil {
		return nil, err
	}
	if !info.Mode().IsRegular() {
		return nil, fmt.Errorf("refusing non-regular file: %s", filepath.Base(path))
	}
	if err = checkPrivate(path, info); err != nil {
		return nil, err
	}
	return os.ReadFile(path)
}
func writePrivate(path string, data []byte) error {
	if info, err := os.Lstat(path); err == nil {
		if !info.Mode().IsRegular() {
			return errors.New("refusing to replace a non-regular config file")
		}
	} else if !errors.Is(err, os.ErrNotExist) {
		return err
	}
	f, err := os.CreateTemp(filepath.Dir(path), ".tofa-*")
	if err != nil {
		return err
	}
	tmp := f.Name()
	defer os.Remove(tmp)
	if err = protect(tmp, false); err != nil {
		f.Close()
		return err
	}
	if _, err = f.Write(data); err != nil {
		f.Close()
		return err
	}
	if err = f.Sync(); err != nil {
		f.Close()
		return err
	}
	if err = f.Close(); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

func safeMessage(s string) string {
	return strings.Map(func(r rune) rune {
		if r < 32 || r == 127 {
			return -1
		}
		return r
	}, s)
}
