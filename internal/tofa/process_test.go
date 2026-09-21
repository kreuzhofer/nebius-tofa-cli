package tofa_test

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"runtime"
	"strings"
	"syscall"
	"testing"
	"time"
)

func TestMain(tests *testing.M) {
	if mode := os.Getenv("TOFA_TEST_CLIENT_MODE"); mode != "" {
		if mode == "ignore" {
			signal.Ignore(os.Interrupt, syscall.SIGTERM)
		}
		capture, _ := json.Marshal(struct {
			Args []string
			Key  string
		}{os.Args[1:], os.Getenv("TOFA_API_KEY")})
		if os.WriteFile(os.Getenv("TOFA_TEST_CAPTURE"), capture, 0600) != nil {
			os.Exit(99)
		}
		if mode == "exit" {
			os.Exit(23)
		}
		for {
			time.Sleep(time.Hour)
		}
	}
	os.Exit(tests.Run())
}

func fakeInstalledClient(t *testing.T, mode string) string {
	t.Helper()
	executable, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	source, err := os.Open(executable)
	if err != nil {
		t.Fatal(err)
	}
	defer source.Close()
	name := "codex"
	if runtime.GOOS == "windows" {
		name += ".exe"
	}
	directory := t.TempDir()
	target, err := os.OpenFile(filepath.Join(directory, name), os.O_CREATE|os.O_WRONLY|os.O_EXCL, 0700)
	if err != nil {
		t.Fatal(err)
	}
	_, copyErr := io.Copy(target, source)
	closeErr := target.Close()
	if copyErr != nil || closeErr != nil {
		t.Fatalf("copy test client: %v %v", copyErr, closeErr)
	}
	capture := filepath.Join(t.TempDir(), "client.json")
	t.Setenv("PATH", directory+string(os.PathListSeparator)+os.Getenv("PATH"))
	t.Setenv("TOFA_TEST_CLIENT_MODE", mode)
	t.Setenv("TOFA_TEST_CAPTURE", capture)
	return capture
}

func TestAdaptedRealChildKeepsExitCodeAndLocalCredential(t *testing.T) {
	capture := fakeInstalledClient(t, "exit")
	app, _ := adapterFixture(t, nil, nil)
	app.RunClient = nil
	err := app.Run([]string{"launch", "codex", "--model", "fixture-model", "--allow-unverified"})
	var exit *exec.ExitError
	if !errors.As(err, &exit) || exit.ExitCode() != 23 {
		t.Fatalf("lost child status: %v", err)
	}
	data, err := os.ReadFile(capture)
	if err != nil {
		t.Fatal(err)
	}
	var child struct {
		Args []string
		Key  string
	}
	if err := json.Unmarshal(data, &child); err != nil {
		t.Fatal(err)
	}
	if len(child.Key) != 64 || strings.Contains(string(data), "fixture-secret") || strings.Contains(strings.Join(child.Args, " "), child.Key) {
		t.Fatal("child credential isolation failed")
	}
}

func TestLaunchCancellationBoundsUnresponsiveChildCleanup(t *testing.T) {
	for _, cause := range []string{"cancellation", "adapter failure"} {
		t.Run(cause, func(t *testing.T) {
			capture := fakeInstalledClient(t, "ignore")
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			app, _ := adapterFixture(t, nil, nil)
			app.RunClient = nil
			listening := make(chan net.Listener, 1)
			app.Listen = func(network, address string) (net.Listener, error) {
				listener, err := net.Listen(network, address)
				if err == nil {
					listening <- listener
				}
				return listener, err
			}
			result := make(chan error, 1)
			go func() {
				result <- app.RunContext(ctx, []string{"launch", "codex", "--model", "fixture-model", "--allow-unverified"})
			}()
			deadline := time.After(5 * time.Second)
			tick := time.NewTicker(10 * time.Millisecond)
			defer tick.Stop()
			waiting := true
			for waiting {
				select {
				case err := <-result:
					t.Fatalf("client exited before cancellation: %v", err)
				case <-deadline:
					t.Fatal("client never became ready")
				case <-tick.C:
					if _, err := os.Stat(capture); err == nil {
						waiting = false
					}
				}
			}
			if cause == "cancellation" {
				cancel()
			} else {
				(<-listening).Close()
			}
			select {
			case err := <-result:
				if err == nil {
					t.Fatal("cancelled launch reported success")
				}
				if cause == "adapter failure" && !strings.Contains(err.Error(), "adapter stopped unexpectedly") {
					t.Fatalf("adapter failure hidden by child exit: %v", err)
				}
			case <-time.After(5 * time.Second):
				t.Fatal("unresponsive client exceeded cleanup deadline")
			}
		})
	}
}

func TestChildStartFailureClosesAdapter(t *testing.T) {
	fakeInstalledClient(t, "exit")
	path, err := exec.LookPath("codex")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("invalid executable fixture"), 0700); err != nil {
		t.Fatal(err)
	}
	app, _ := adapterFixture(t, nil, nil)
	app.RunClient = nil
	var address string
	app.Listen = func(network, bind string) (net.Listener, error) {
		listener, err := net.Listen(network, bind)
		if err == nil {
			address = listener.Addr().String()
		}
		return listener, err
	}
	err = app.Run([]string{"launch", "codex", "--model", "fixture-model", "--allow-unverified"})
	if err == nil || !strings.Contains(err.Error(), "could not start") {
		t.Fatalf("missing child startup error: %v", err)
	}
	connection, err := net.DialTimeout("tcp", address, time.Second)
	if err == nil {
		connection.Close()
		t.Fatal("adapter survived child startup failure")
	}
}
