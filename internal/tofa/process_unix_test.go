//go:build !windows

package tofa_test

import (
	"bytes"
	"context"
	"github.com/kreuzhofer/nebius-tofa-cli/internal/tofa"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"
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
	err := app.Run([]string{"launch", "codex", "--model", "fixture-model", "--allow-unverified", "--direct", "--", "hello"})
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

func TestUnixLaunchSignalsStopChildAndAdapter(t *testing.T) {
	if os.Getenv("TOFA_TEST_SIGNAL_LAUNCH") == "1" {
		capture := fakeInstalledClient(t, "ignore")
		app, _ := adapterFixture(t, nil, nil)
		app.RunClient = nil
		var address string
		app.Listen = func(network, bind string) (net.Listener, error) {
			listener, err := net.Listen(network, bind)
			if err == nil {
				address = listener.Addr().String()
				go func() {
					for {
						if _, err := os.Stat(capture); err == nil {
							os.WriteFile(os.Getenv("TOFA_TEST_SIGNAL_READY"), []byte(address), 0600)
							return
						}
						time.Sleep(10 * time.Millisecond)
					}
				}()
			}
			return listener, err
		}
		if err := app.Run([]string{"launch", "codex", "--model", "fixture-model", "--allow-unverified"}); err == nil {
			t.Fatal("signal cancellation returned success")
		}
		connection, err := net.DialTimeout("tcp", address, time.Second)
		if err == nil {
			connection.Close()
			t.Fatal("adapter survived signal cleanup")
		}
		return
	}
	for _, interrupt := range []os.Signal{os.Interrupt, syscall.SIGTERM} {
		t.Run(interrupt.String(), func(t *testing.T) {
			executable, err := os.Executable()
			if err != nil {
				t.Fatal(err)
			}
			ready := filepath.Join(t.TempDir(), "ready")
			ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
			defer cancel()
			command := exec.CommandContext(ctx, executable, "-test.run=^TestUnixLaunchSignalsStopChildAndAdapter$")
			command.Env = append(os.Environ(), "TOFA_TEST_SIGNAL_LAUNCH=1", "TOFA_TEST_SIGNAL_READY="+ready)
			var output bytes.Buffer
			command.Stdout, command.Stderr = &output, &output
			if err := command.Start(); err != nil {
				t.Fatal(err)
			}
			finished := make(chan error, 1)
			go func() { finished <- command.Wait() }()
			for {
				if _, err := os.Stat(ready); err == nil {
					break
				}
				select {
				case err := <-finished:
					t.Fatalf("signal fixture exited before ready: %v %s", err, output.String())
				case <-ctx.Done():
					t.Fatal("signal fixture did not become ready")
				case <-time.After(10 * time.Millisecond):
				}
			}
			if err := command.Process.Signal(interrupt); err != nil {
				t.Fatal(err)
			}
			select {
			case err := <-finished:
				if err != nil {
					t.Fatalf("signal fixture failed: %v %s", err, output.String())
				}
			case <-time.After(6 * time.Second):
				t.Fatal("signal cleanup exceeded deadline")
			}
		})
	}
}
