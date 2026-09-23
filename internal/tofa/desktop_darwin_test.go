//go:build darwin

package tofa_test

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"syscall"
	"testing"
	"time"
)

// A real executable fixture exercises the bundle, app-server protocol, environment,
// filesystem and HTTP boundaries without starting Electron or paid inference.
func desktopFixture(t *testing.T, mode string) (string, string) {
	t.Helper()
	version, err := exec.Command("/usr/bin/sw_vers", "-productVersion").Output()
	if err != nil || runtime.GOARCH != "arm64" || strings.TrimSpace(string(version)) != "26.6.2" {
		t.Skip("desktop executable qualification is pinned to macOS 26.6.2 arm64")
	}
	python, err := exec.LookPath("python3")
	if err != nil {
		t.Fatal(err)
	}
	bundle := filepath.Join(t.TempDir(), "Renamed.app")
	for _, dir := range []string{"Contents/MacOS", "Contents/Resources"} {
		if err := os.MkdirAll(filepath.Join(bundle, dir), 0700); err != nil {
			t.Fatal(err)
		}
	}
	plist := `<?xml version="1.0"?><plist version="1.0"><dict>
<key>CFBundleIdentifier</key><string>com.openai.codex</string>
<key>CFBundleName</key><string>ChatGPT</string>
<key>CFBundleExecutable</key><string>ChatGPT</string>
<key>CFBundleShortVersionString</key><string>26.915.31945</string>
<key>CFBundleVersion</key><string>9922</string></dict></plist>`
	if err := os.WriteFile(filepath.Join(bundle, "Contents/Info.plist"), []byte(plist), 0600); err != nil {
		t.Fatal(err)
	}
	capture := filepath.Join(t.TempDir(), "capture.json")
	script := fmt.Sprintf(`#!%s
import json, os, pathlib, re, sys, time, urllib.request, urllib.error, signal, subprocess
MODE = %q
CAPTURE = %q
if '--version' in sys.argv:
    print('codex-cli 0.155.0-alpha.9.2')
    sys.exit(0)
if '--owned-worker' in sys.argv:
    signal.signal(signal.SIGTERM, signal.SIG_IGN)
    if MODE == 'engine-exit': time.sleep(0.7); sys.exit(19)
    while True: time.sleep(1)
home = pathlib.Path(os.environ['CODEX_HOME'])
config_text = (home / 'config.toml').read_text()
def setting(key):
    return json.loads(re.search(r'^' + key + r' = (.+)$', config_text, re.M).group(1))
if 'app-server' in sys.argv:
    for line in sys.stdin:
        req = json.loads(line)
        if 'id' not in req: continue
        result = {}
        if req['method'] == 'config/read':
            result = {'config': {k: setting(k) for k in ['model', 'model_provider', 'model_catalog_json', 'web_search', 'cli_auth_credentials_store', 'sqlite_home', 'log_dir']}}
            result['config']['model_providers'] = {'nebius-tofa': {k: setting(k) for k in ['base_url', 'env_key', 'wire_api', 'requires_openai_auth', 'supports_websockets']}}
        if req['method'] == 'configRequirements/read':
            result = {'requirements': {'modelProvider': 'openai'} if MODE == 'managed' else None}
        print(json.dumps({'id': req['id'], 'result': result}), flush=True)
    sys.exit(0)
if MODE == 'exit': sys.exit(23)
(home / 'sessions').mkdir(exist_ok=True)
(home / 'sessions' / 'conversation.jsonl').write_text('preserve conversation')
pathlib.Path('user-work.txt').write_text('preserve workspace')
pathlib.Path(CAPTURE).write_text(json.dumps({'args': sys.argv[1:], 'home': str(home), 'electron': os.environ['CODEX_ELECTRON_USER_DATA_PATH'], 'cwd': os.getcwd(), 'key': os.environ['TOFA_API_KEY'], 'config': config_text, 'env': dict(os.environ)}))
if MODE == 'http':
    results = []
    for model, key in [('gpt-5.6-luna', os.environ['TOFA_API_KEY']), ('moonshotai/Kimi-K3', 'wrong'), ('moonshotai/Kimi-K3', os.environ['TOFA_API_KEY'])]:
        req = urllib.request.Request(setting('base_url') + '/responses', data=json.dumps({'model': model, 'stream': True, 'input': []}).encode(), headers={'Authorization':'Bearer '+key, 'Content-Type':'application/json'})
        try:
            with urllib.request.urlopen(req) as res: results.append({'status':res.status, 'body':res.read().decode()})
        except urllib.error.HTTPError as err: results.append({'status':err.code, 'body':err.read().decode()})
    pathlib.Path(CAPTURE+'.http').write_text(json.dumps(results))
if MODE != 'exit':
    engine = pathlib.Path(__file__).parent.parent / 'Resources' / 'codex'
    worker_args = [str(engine), '-c', 'features.code_mode_host=true', 'app-server'] + ([] if engine.is_symlink() else ['--owned-worker'])
    worker = subprocess.Popen(worker_args, stdin=subprocess.PIPE, stdout=subprocess.DEVNULL, stderr=subprocess.DEVNULL)
    pathlib.Path(CAPTURE+'.pid').write_text(str(worker.pid))
    if MODE not in ['ignore', 'engine-exit']: time.sleep(0.4); sys.exit(0)
    signal.signal(signal.SIGTERM, signal.SIG_IGN)
    while True: time.sleep(1)
`, python, mode, capture)
	for _, name := range []string{"Contents/MacOS/ChatGPT", "Contents/Resources/codex"} {
		if err := os.WriteFile(filepath.Join(bundle, name), []byte(script), 0700); err != nil {
			t.Fatal(err)
		}
	}
	return bundle, capture
}

func TestDesktopRejectsAuxiliaryModelsAndStreamsSelectedModel(t *testing.T) {
	bundle, capture := desktopFixture(t, "http")
	requests := 0
	app, output := adapterFixture(t, func(w http.ResponseWriter, r *http.Request) {
		requests++
		var body struct{ Model string }
		if json.NewDecoder(r.Body).Decode(&body) != nil || body.Model != "moonshotai/Kimi-K3" {
			t.Error("unsupported model reached upstream")
		}
		w.Header().Set("Content-Type", "text/event-stream")
		fmt.Fprint(w, "event: response.output_text.delta\ndata: {\"delta\":\"fixture streamed\"}\n\n")
	}, nil)
	if err := app.Run([]string{"launch", "codex-desktop", "--app-bundle", bundle, "--model", "moonshotai/Kimi-K3", "--allow-unverified"}); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(capture + ".http")
	if err != nil {
		t.Fatal(err)
	}
	var results []struct {
		Status int
		Body   string
	}
	if err := json.Unmarshal(data, &results); err != nil {
		t.Fatal(err)
	}
	if requests != 1 || len(results) != 3 || results[0].Status != 400 || results[1].Status != 401 || results[2].Status != 200 || !strings.Contains(results[2].Body, "fixture streamed") {
		t.Fatalf("request gate failed: %d upstream requests; %s", requests, data)
	}
	if !strings.Contains(output.String(), "unsupported model") {
		t.Fatal("auxiliary failure not surfaced in terminal")
	}
}

func TestDesktopLaunchOwnsIsolatedStateAndPreservesConversation(t *testing.T) {
	bundle, capture := desktopFixture(t, "normal")
	ordinary := t.TempDir()
	t.Setenv("CODEX_HOME", ordinary)
	t.Setenv("CODEX_APP_SERVER_OPENAI_BASE_URL", "https://wrong.invalid")
	t.Setenv("OPENAI_API_KEY", "ordinary-secret")
	for _, name := range []string{"config.toml", "auth.json"} {
		if err := os.WriteFile(filepath.Join(ordinary, name), []byte("ordinary unchanged"), 0600); err != nil {
			t.Fatal(err)
		}
	}
	app, output := adapterFixture(t, func(http.ResponseWriter, *http.Request) { t.Error("unexpected inference") }, nil)
	var address string
	app.Listen = func(network, bind string) (net.Listener, error) {
		listener, err := net.Listen(network, bind)
		if err == nil {
			address = listener.Addr().String()
		}
		return listener, err
	}
	err := app.Run([]string{"launch", "codex-desktop", "--app-bundle", bundle, "--model", "moonshotai/Kimi-K3", "--allow-unverified"})
	if err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(capture)
	if err != nil {
		t.Fatal(err)
	}
	var child struct {
		Args                             []string
		Home, Electron, Cwd, Key, Config string
		Env                              map[string]string
	}
	if err := json.Unmarshal(data, &child); err != nil {
		t.Fatal(err)
	}
	if child.Home == ordinary || child.Electron == "" || len(child.Key) != 64 || strings.Contains(string(data), "fixture-secret") || strings.Contains(string(data), "ordinary-secret") || child.Env["CODEX_APP_SERVER_OPENAI_BASE_URL"] != "" {
		t.Fatal("desktop routing or credential isolation failed")
	}
	if !strings.Contains(strings.Join(child.Args, " "), "codex://threads/new?mode=codex") {
		t.Fatal("Codex mode not explicit")
	}
	for _, path := range []string{filepath.Join(child.Home, "sessions/conversation.jsonl"), filepath.Join(child.Cwd, "user-work.txt")} {
		if _, err := os.Stat(path); err != nil {
			t.Fatalf("user data lost: %v", err)
		}
	}
	if _, err := os.Stat(filepath.Join(child.Home, "config.toml")); !os.IsNotExist(err) {
		t.Fatal("temporary routing survived exit")
	}
	for _, name := range []string{"config.toml", "auth.json"} {
		data, err := os.ReadFile(filepath.Join(ordinary, name))
		if err != nil || string(data) != "ordinary unchanged" {
			t.Fatal("ordinary state changed")
		}
	}
	connection, err := net.DialTimeout("tcp", address, time.Second)
	if err == nil {
		connection.Close()
		t.Fatal("adapter survived desktop exit")
	}
	if !strings.Contains(output.String(), "title") || !strings.Contains(output.String(), "unverified") {
		t.Fatal("experimental limitations not announced")
	}
	assertDesktopWorkerStopped(t, capture)
}

func TestDesktopRejectsManagedRoutingBeforeStartingApp(t *testing.T) {
	bundle, capture := desktopFixture(t, "managed")
	app, _ := adapterFixture(t, nil, nil)
	err := app.Run([]string{"launch", "codex-desktop", "--app-bundle", bundle, "--model", "moonshotai/Kimi-K3", "--allow-unverified"})
	if err == nil || !strings.Contains(err.Error(), "managed") {
		t.Fatalf("managed routing not enforced: %v", err)
	}
	if _, err := os.Stat(capture); !os.IsNotExist(err) {
		t.Fatal("desktop launched despite conflicting policy")
	}
}

func TestDesktopInstalledEngineOfflineRouting(t *testing.T) {
	if os.Getenv("TOFA_TEST_DESKTOP_ENGINE") == "" {
		t.Skip("set TOFA_TEST_DESKTOP_ENGINE to test the installed bundled engine without launching Electron")
	}
	bundle, _ := desktopFixture(t, "normal")
	engine := filepath.Join(bundle, "Contents/Resources/codex")
	if err := os.Remove(engine); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(os.Getenv("TOFA_TEST_DESKTOP_ENGINE"), engine); err != nil {
		t.Fatal(err)
	}
	app, _ := adapterFixture(t, nil, nil)
	if err := app.Run([]string{"launch", "codex-desktop", "--app-bundle", bundle, "--model", "moonshotai/Kimi-K3", "--allow-unverified"}); err != nil {
		t.Fatal(err)
	}
}

func TestDesktopEngineExitStopsOwnedApp(t *testing.T) {
	bundle, capture := desktopFixture(t, "engine-exit")
	app, _ := adapterFixture(t, nil, nil)
	ctx, cancel := context.WithTimeout(context.Background(), 6*time.Second)
	defer cancel()
	err := app.RunContext(ctx, []string{"launch", "codex-desktop", "--app-bundle", bundle, "--model", "moonshotai/Kimi-K3", "--allow-unverified"})
	if err == nil || !strings.Contains(err.Error(), "app-server exited") {
		t.Fatalf("engine loss not detected: %v", err)
	}
	assertDesktopWorkerStopped(t, capture)
}

func assertDesktopWorkerStopped(t *testing.T, capture string) {
	t.Helper()
	data, err := os.ReadFile(capture + ".pid")
	if err != nil {
		t.Fatal(err)
	}
	var pid int
	if _, err := fmt.Sscan(string(data), &pid); err != nil {
		t.Fatal(err)
	}
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		if errors.Is(syscall.Kill(pid, 0), syscall.ESRCH) {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("owned app-server %d survived cleanup", pid)
}

func TestDesktopCancellationClosesOwnedProcessesAndAdapter(t *testing.T) {
	for _, cause := range []string{"interrupt", "adapter failure"} {
		t.Run(cause, func(t *testing.T) {
			bundle, capture := desktopFixture(t, "ignore")
			app, _ := adapterFixture(t, nil, nil)
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			listening := make(chan net.Listener, 1)
			app.Listen = func(network, address string) (net.Listener, error) {
				listener, err := net.Listen(network, address)
				if err == nil {
					listening <- listener
				}
				return listener, err
			}
			done := make(chan error, 1)
			go func() {
				done <- app.RunContext(ctx, []string{"launch", "codex-desktop", "--app-bundle", bundle, "--model", "moonshotai/Kimi-K3", "--allow-unverified"})
			}()
			deadline := time.Now().Add(5 * time.Second)
			for {
				if _, err := os.Stat(capture + ".pid"); err == nil {
					break
				}
				if time.Now().After(deadline) {
					cancel()
					t.Fatal("desktop never started")
				}
				time.Sleep(10 * time.Millisecond)
			}
			listener := <-listening
			if cause == "interrupt" {
				cancel()
			} else {
				listener.Close()
			}
			select {
			case err := <-done:
				if err == nil {
					t.Fatal("cancelled desktop reported success")
				}
				if cause == "adapter failure" && !strings.Contains(err.Error(), "adapter stopped unexpectedly") {
					t.Fatal(err)
				}
			case <-time.After(5 * time.Second):
				t.Fatal("desktop cleanup exceeded deadline")
			}
			assertDesktopWorkerStopped(t, capture)
			connection, err := net.DialTimeout("tcp", listener.Addr().String(), time.Second)
			if err == nil {
				connection.Close()
				t.Fatal("desktop listener survived cancellation")
			}
		})
	}
}

func TestDesktopStartupAndRoutingFailures(t *testing.T) {
	for _, failure := range []string{"missing bundle", "wrong version", "wrong engine", "invalid executable", "exit", "effective routing", "extra auth", "state redirect"} {
		t.Run(failure, func(t *testing.T) {
			bundle, capture := desktopFixture(t, "exit")
			switch failure {
			case "missing bundle":
				bundle = filepath.Join(t.TempDir(), "Missing.app")
			case "wrong version":
				path := filepath.Join(bundle, "Contents/Info.plist")
				data, _ := os.ReadFile(path)
				if err := os.WriteFile(path, []byte(strings.ReplaceAll(string(data), "26.915.31945", "99.0.0")), 0600); err != nil {
					t.Fatal(err)
				}
			case "wrong engine", "effective routing", "extra auth", "state redirect":
				path := filepath.Join(bundle, "Contents/Resources/codex")
				data, _ := os.ReadFile(path)
				text := string(data)
				if failure == "wrong engine" {
					text = strings.ReplaceAll(text, "0.155.0-alpha.9.2", "9.9.9")
				}
				if failure == "effective routing" {
					text = strings.ReplaceAll(text, "result['config']['model_providers'] =", "result['config']['model_provider'] = 'openai'\n            result['config']['model_providers'] =")
				}
				if failure == "extra auth" {
					text = strings.ReplaceAll(text, "print(json.dumps({'id':", "if req['method'] == 'config/read': result['config']['model_providers']['nebius-tofa']['http_headers'] = {'Authorization':'Bearer unwanted'}\n        print(json.dumps({'id':")
				}
				if failure == "state redirect" {
					text = strings.ReplaceAll(text, "print(json.dumps({'id':", "if req['method'] == 'config/read': result['config']['sqlite_home'] = '/ordinary/state'\n        print(json.dumps({'id':")
				}
				if err := os.WriteFile(path, []byte(text), 0700); err != nil {
					t.Fatal(err)
				}
			case "invalid executable":
				if err := os.WriteFile(filepath.Join(bundle, "Contents/MacOS/ChatGPT"), []byte("invalid executable"), 0700); err != nil {
					t.Fatal(err)
				}
			}
			app, _ := adapterFixture(t, nil, nil)
			var address string
			app.Listen = func(network, bind string) (net.Listener, error) {
				listener, err := net.Listen(network, bind)
				if err == nil {
					address = listener.Addr().String()
				}
				return listener, err
			}
			err := app.Run([]string{"launch", "codex-desktop", "--app-bundle", bundle, "--model", "moonshotai/Kimi-K3", "--allow-unverified"})
			if err == nil {
				t.Fatal("invalid desktop launch succeeded")
			}
			if failure == "exit" {
				var exit *exec.ExitError
				if !errors.As(err, &exit) || exit.ExitCode() != 23 {
					t.Fatalf("lost desktop exit status: %v", err)
				}
			}
			if failure == "extra auth" && !strings.Contains(err.Error(), "provider") {
				t.Fatalf("provider override was not rejected: %v", err)
			}
			if failure == "state redirect" && !strings.Contains(err.Error(), "sqlite_home") {
				t.Fatalf("state redirect was not rejected: %v", err)
			}
			if _, err := os.Stat(capture); !os.IsNotExist(err) {
				t.Fatal("failed launch created a conversation")
			}
			if address != "" {
				connection, err := net.DialTimeout("tcp", address, time.Second)
				if err == nil {
					connection.Close()
					t.Fatal("listener survived failed launch")
				}
			}
		})
	}
}
