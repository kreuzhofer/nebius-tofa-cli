// Test-only launcher: real terminal and child process, isolated synthetic store
// and loopback provider. Production exposes neither override.
package main

import (
	"bytes"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"os"

	"github.com/kreuzhofer/tofa-launcher/internal/tofa"
)

type failingTerminal struct{}

// This fixture never touches the user's native credential vault.
type absentVault struct{}

func (absentVault) Availability() error {
	switch os.Getenv("FIXTURE_VAULT") {
	case "write-failed":
		return nil
	case "locked", "denied", "failed":
		return errors.New("fixture vault unavailable")
	default:
		return tofa.ErrVaultAbsent
	}
}
func (absentVault) Get(string) (string, error) { return "", tofa.ErrMissing }
func (absentVault) Set(string, string) error   { return errors.New("fixture vault absent") }
func (absentVault) Delete(string) error        { return nil }

func (failingTerminal) Write(p []byte) (int, error) {
	if bytes.Contains(p, []byte("\r\x1b[2K")) {
		return 0, errors.New("synthetic terminal output failure")
	}
	return os.Stdout.Write(p)
}

func main() {
	endpoint := os.Getenv("FIXTURE_ENDPOINT")
	u, err := url.Parse(endpoint)
	if err != nil || u.Scheme != "http" || u.Hostname() != "127.0.0.1" || u.Port() == "" || u.User != nil {
		panic("fixture requires a loopback provider")
	}
	app := tofa.App{Dir: os.Getenv("FIXTURE_DIR"), Vault: absentVault{}, Endpoint: endpoint, HTTP: &http.Client{Transport: &http.Transport{Proxy: nil}}}
	if os.Getenv("FIXTURE_VAULT") == "native" {
		app.Vault = tofa.NativeVault{}
	}
	if os.Getenv("FIXTURE_OUTPUT_FAILURE") == "1" {
		app.Out = failingTerminal{}
	}
	if err := app.Run(os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
