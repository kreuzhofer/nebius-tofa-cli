//go:build !windows

package tofa_test

import (
	"bytes"
	"github.com/kreuzhofer/nebius-tofa-cli/internal/tofa"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestRealChildReceivesScopedEnvironmentAndExitStatus(t *testing.T) {
	dir := t.TempDir()
	bin := t.TempDir()
	home := t.TempDir()
	capture := filepath.Join(home, "capture")
	v := &vault{values: map[string]string{}}
	s := tofa.Store{Dir: dir, Vault: v}
	if err := s.Login("project", "fixture-token", "keyring"); err != nil {
		t.Fatal(err)
	}
	// Fake installed client, launched through the real OS process boundary.
	script := "#!/bin/sh\nprintf '%s\\n' \"$TOFA_API_KEY\" \"$@\" > \"$TOFA_TEST_CAPTURE\"\nexit 23\n"
	if err := os.WriteFile(filepath.Join(bin, "codex"), []byte(script), 0700); err != nil {
		t.Fatal(err)
	}
	codexConfig := filepath.Join(home, "config.toml")
	sentinel := []byte("model_provider = \"original\"\n")
	if err := os.WriteFile(codexConfig, sentinel, 0600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))
	t.Setenv("TOFA_TEST_CAPTURE", capture)
	t.Setenv("CODEX_HOME", home)
	t.Setenv("TOFA_API_KEY", "parent-token")
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.Write([]byte(`{"data":[{"id":"fixture-model"}]}`)) }))
	defer server.Close()
	var out bytes.Buffer
	app := tofa.App{Dir: dir, Vault: v, Out: &out, Endpoint: server.URL, HTTP: server.Client()}
	err := app.Run([]string{"launch", "codex", "--model", "fixture-model", "--allow-unverified", "--", "hello"})
	exit, ok := err.(*exec.ExitError)
	if !ok || exit.ExitCode() != 23 {
		t.Fatalf("lost child exit code: %v", err)
	}
	captured, err := os.ReadFile(capture)
	if err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(string(captured), "\n")
	if lines[0] != "fixture-token" {
		t.Fatal("child did not receive key")
	}
	if strings.Contains(strings.Join(lines[1:], "\n"), "fixture-token") {
		t.Fatal("key leaked into argv")
	}
	if os.Getenv("TOFA_API_KEY") != "parent-token" {
		t.Fatal("parent environment changed")
	}
	after, err := os.ReadFile(codexConfig)
	if err != nil || !bytes.Equal(after, sentinel) {
		t.Fatal("normal client config changed")
	}
}
