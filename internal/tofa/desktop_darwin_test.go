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
	"regexp"
	"runtime"
	"strings"
	"sync/atomic"
	"syscall"
	"testing"
	"time"

	"github.com/kreuzhofer/nebius-tofa-cli/internal/tofa"
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
	// Never read the operator's startup files during offline desktop qualification.
	t.Setenv("HOME", t.TempDir())
	t.Setenv("ZDOTDIR", t.TempDir())
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
if '--ordinary-probe' in sys.argv:
    print(json.dumps({'args':sys.argv[1:], 'home':os.environ.get('CODEX_HOME'), 'native_key':os.environ.get('OPENAI_API_KEY'), 'tofa_key':os.environ.get('TOFA_API_KEY')}))
    sys.exit(23)
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
# Desktop 26.915.31945 QDe/pq/gq: reload an interactive login shell,
# merge its environment, then restore only the explicitly supplied CODEX_HOME.
probe = "printf '\\0%%s\\0' '_SHELL_ENV_DELIMITER_'; command env -0 || exit; printf '\\0%%s\\0' '_SHELL_ENV_DELIMITER_'; exit"
shell_env = dict(os.environ, CODEX_SHELL='1', DISABLE_AUTO_UPDATE='true', ZSH_TMUX_AUTOSTARTED='true', ZSH_TMUX_AUTOSTART='false')
loaded = subprocess.run(['/bin/zsh', '-ilc', probe], env=shell_env, stdout=subprocess.PIPE, stderr=subprocess.PIPE, check=True).stdout.split(b'\0_SHELL_ENV_DELIMITER_\0')[1]
os.environ.update(dict(item.decode().split('=', 1) for item in loaded.split(b'\0') if b'=' in item))
os.environ['CODEX_HOME'] = str(home)
if MODE == 'exit': sys.exit(23)
(home / 'sessions').mkdir(exist_ok=True)
(home / 'sessions' / 'conversation.jsonl').write_text('preserve conversation')
pathlib.Path('user-work.txt').write_text('preserve workspace')
pathlib.Path(CAPTURE).write_text(json.dumps({'args': sys.argv[1:], 'home': str(home), 'electron': os.environ['CODEX_ELECTRON_USER_DATA_PATH'], 'cwd': os.getcwd(), 'key': os.environ['TOFA_API_KEY'], 'config': config_text, 'env': dict(os.environ)}))
(pathlib.Path(os.environ['CODEX_ELECTRON_USER_DATA_PATH']) / 'tool-settings.json').write_text(json.dumps({'engine':os.environ.get('CODEX_CLI_PATH')}))
if MODE == 'shell-commands':
    result = subprocess.run(['/bin/zsh', '-ilc', 'printf "%%s|%%s" "$STARTUP_ENV" "$STARTUP_RC"'], stdout=subprocess.PIPE, stderr=subprocess.PIPE, check=True)
    pathlib.Path(CAPTURE+'.commands').write_bytes(result.stdout)
if MODE == 'http':
    results = []
    for model, key in [('gpt-5.6-luna', os.environ['TOFA_API_KEY']), ('moonshotai/Kimi-K3', 'wrong'), ('moonshotai/Kimi-K3', os.environ['TOFA_API_KEY'])]:
        req = urllib.request.Request(setting('base_url') + '/responses', data=json.dumps({'model': model, 'stream': True, 'input': []}).encode(), headers={'Authorization':'Bearer '+key, 'Content-Type':'application/json'})
        try:
            with urllib.request.urlopen(req) as res: results.append({'status':res.status, 'body':res.read().decode()})
        except urllib.error.HTTPError as err: results.append({'status':err.code, 'body':err.read().decode()})
    pathlib.Path(CAPTURE+'.http').write_text(json.dumps(results))
if MODE != 'exit':
    engine = pathlib.Path(os.environ.get('CODEX_CLI_PATH') or pathlib.Path(__file__).parent.parent / 'Resources' / 'codex')
    bundled_engine = pathlib.Path(__file__).parent.parent / 'Resources' / 'codex'
    worker_args = [str(engine), '-c', 'features.code_mode_host=true', 'app-server'] + ([] if bundled_engine.is_symlink() else ['--owned-worker'])
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

func TestDesktopShellReloadPreservesAdapterCredential(t *testing.T) {
	bundle, capture := desktopFixture(t, "http")
	startup := "export TOFA_API_KEY=shell-credential-must-not-replace-adapter-token\n"
	if err := os.WriteFile(filepath.Join(os.Getenv("ZDOTDIR"), ".zshrc"), []byte(startup), 0600); err != nil {
		t.Fatal(err)
	}
	requests := 0
	app, _ := adapterFixture(t, func(w http.ResponseWriter, r *http.Request) {
		requests++
		fmt.Fprint(w, "streamed through authenticated adapter")
	}, nil)
	if err := app.Run([]string{"launch", "codex-desktop", "--app-bundle", bundle, "--model", "moonshotai/Kimi-K3", "--allow-unverified"}); err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(capture + ".http")
	if err != nil {
		t.Fatal(err)
	}
	var responses []struct {
		Status int
		Body   string
	}
	if err := json.Unmarshal(raw, &responses); err != nil {
		t.Fatal(err)
	}
	if requests != 1 || len(responses) != 3 || responses[2].Status != 200 || responses[2].Body != "streamed through authenticated adapter" {
		t.Fatalf("shell initialization broke authenticated inference: upstream=%d responses=%s", requests, raw)
	}
}

func TestDesktopShellReloadPreservesEngineAndCodingStartup(t *testing.T) {
	for _, custom := range []bool{false, true} {
		t.Run(fmt.Sprintf("custom startup directory %v", custom), func(t *testing.T) {
			bundle, capture := desktopFixture(t, "shell-commands")
			startupDir := os.Getenv("HOME")
			if custom {
				startupDir = filepath.Join(t.TempDir(), "user's shell files")
				if err := os.Mkdir(startupDir, 0700); err != nil {
					t.Fatal(err)
				}
				t.Setenv("ZDOTDIR", startupDir)
			} else {
				if err := os.Unsetenv("ZDOTDIR"); err != nil {
					t.Fatal(err)
				}
			}
			files := map[string]string{
				".zshenv": "export STARTUP_ENV=normal-env\nexport CODEX_CLI_PATH=/unverified/engine\nexport OPENAI_API_KEY=synthetic-shell-secret\n",
				".zshrc":  "export STARTUP_RC=normal-rc\nexport CODEX_APP_SERVER_OPENAI_BASE_URL=https://wrong.invalid\n",
			}
			for name, content := range files {
				if err := os.WriteFile(filepath.Join(startupDir, name), []byte(content), 0600); err != nil {
					t.Fatal(err)
				}
			}
			app, output := adapterFixture(t, nil, nil)
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			if err := app.RunContext(ctx, []string{"launch", "codex-desktop", "--app-bundle", bundle, "--model", "moonshotai/Kimi-K3", "--allow-unverified"}); err != nil {
				t.Fatal(err)
			}
			commands, err := os.ReadFile(capture + ".commands")
			if err != nil || string(commands) != "normal-env|normal-rc" {
				t.Fatalf("coding shell startup lost: %s %v", commands, err)
			}
			var child struct{ Env map[string]string }
			raw, err := os.ReadFile(capture)
			if err != nil {
				t.Fatal(err)
			}
			if err := json.Unmarshal(raw, &child); err != nil {
				t.Fatal(err)
			}
			if path := child.Env["CODEX_CLI_PATH"]; !strings.HasPrefix(path, app.Dir+"/") || filepath.Base(path) != "tofa-desktop-engine" {
				t.Fatal("shell replaced the launcher-owned engine bridge")
			}
			for _, key := range []string{"OPENAI_API_KEY", "CODEX_APP_SERVER_OPENAI_BASE_URL"} {
				if child.Env[key] != "" {
					t.Errorf("shell reintroduced %s into desktop environment", key)
				}
			}
			if _, err := os.Stat(child.Env["ZDOTDIR"]); !os.IsNotExist(err) {
				t.Fatal("launcher shell files survived desktop exit")
			}
			for name, content := range files {
				raw, err := os.ReadFile(filepath.Join(startupDir, name))
				if err != nil || string(raw) != content {
					t.Fatalf("ordinary %s modified", name)
				}
			}
			if !strings.Contains(output.String(), "Desktop environment probe isolated") {
				t.Fatal("environment isolation was not announced")
			}
			assertDesktopWorkerStopped(t, capture)
		})
	}
}

// Optional real Electron qualification: no inference or ordinary desktop state.
// Inspect only the owned engine and authenticate to the local HTTP fixture using
// its temporary credential; never log a process environment or credential value.
func TestDesktopInstalledAppShellIsolation(t *testing.T) {
	bundle := os.Getenv("TOFA_TEST_DESKTOP_APP")
	if bundle == "" {
		t.Skip("set TOFA_TEST_DESKTOP_APP to qualify installed Electron with synthetic shell exports")
	}
	if !filepath.IsAbs(bundle) {
		t.Fatal("TOFA_TEST_DESKTOP_APP must be absolute")
	}
	t.Setenv("HOME", t.TempDir())
	startupDir := t.TempDir()
	t.Setenv("ZDOTDIR", startupDir)
	if err := os.WriteFile(filepath.Join(startupDir, ".zshenv"), []byte("export TOFA_API_KEY=synthetic-wrong-key\nexport CODEX_CLI_PATH=/unverified/engine\nexport OPENAI_API_KEY=synthetic-shell-key\n"), 0600); err != nil {
		t.Fatal(err)
	}
	app, _ := adapterFixture(t, func(w http.ResponseWriter, r *http.Request) { fmt.Fprint(w, "authenticated owned engine") }, nil)
	listening := make(chan net.Listener, 1)
	app.Listen = func(network, address string) (net.Listener, error) {
		listener, err := net.Listen(network, address)
		if err == nil {
			listening <- listener
		}
		return listener, err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	done := make(chan error, 1)
	go func() {
		done <- app.RunContext(ctx, []string{"launch", "codex-desktop", "--app-bundle", bundle, "--model", "moonshotai/Kimi-K3", "--allow-unverified"})
	}()
	defer func() {
		cancel()
		select {
		case <-done:
		case <-time.After(5 * time.Second):
			t.Error("owned desktop cleanup exceeded deadline")
		}
	}()
	var enginePID string
	for enginePID == "" {
		select {
		case <-ctx.Done():
			t.Fatal("owned bundled engine did not start with isolated shell environment")
		default:
		}
		raw, err := exec.Command("/bin/ps", "-axo", "pid=,pgid=,args=").Output()
		if err != nil {
			t.Fatal("could not inspect owned processes")
		}
		lines := strings.Split(string(raw), "\n")
		var group string
		for _, line := range lines {
			fields := strings.Fields(line)
			if len(fields) > 2 && strings.Contains(line, filepath.Join(bundle, "Contents/MacOS/ChatGPT")+" ") && strings.Contains(line, "--user-data-dir="+app.Dir+"/desktop-sessions/") {
				group = fields[1]
			}
		}
		for _, line := range lines {
			fields := strings.Fields(line)
			if len(fields) > 2 && group != "" && fields[1] == group && strings.Contains(line, filepath.Join(bundle, "Contents/Resources/codex")+" ") && strings.Contains(line, "app-server") {
				enginePID = fields[0]
			}
		}
		if enginePID == "" {
			time.Sleep(100 * time.Millisecond)
		}
	}
	raw, err := exec.Command("/bin/ps", "eww", "-p", enginePID, "-o", "command=").Output()
	if err != nil {
		t.Fatal("could not inspect owned engine environment")
	}
	credential := regexp.MustCompile(`(?:^| )TOFA_API_KEY=([a-f0-9]{64})(?: |$)`).FindSubmatch(raw)
	if len(credential) != 2 || strings.Contains(string(raw), "synthetic-wrong-key") || strings.Contains(string(raw), "synthetic-shell-key") || strings.Contains(string(raw), "/unverified/engine") {
		t.Fatal("shell overwrote owned engine settings")
	}
	var listener net.Listener
	select {
	case listener = <-listening:
	case <-ctx.Done():
		t.Fatal("missing adapter listener")
	}
	response := adapterRequest(t, "http://"+listener.Addr().String(), string(credential[1]), `{"model":"moonshotai/Kimi-K3","input":[]}`)
	if response.StatusCode != 200 {
		t.Fatalf("owned engine credential rejected: %s", response.Status)
	}
	t.Log("Installed desktop loaded the bundled engine and retained its working adapter credential despite conflicting shell exports.")
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

func TestDesktopSavedEngineReferenceSurvivesCleanup(t *testing.T) {
	bundle, capture := desktopFixture(t, "normal")
	app, _ := adapterFixture(t, nil, nil)
	if err := app.Run([]string{"launch", "codex-desktop", "--app-bundle", bundle, "--model", "moonshotai/Kimi-K3", "--allow-unverified"}); err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(capture)
	if err != nil {
		t.Fatal(err)
	}
	var child struct{ Env map[string]string }
	if err := json.Unmarshal(raw, &child); err != nil {
		t.Fatal(err)
	}
	bridge := child.Env["CODEX_CLI_PATH"]
	if bridge == "" {
		t.Fatal("desktop has no durable engine reference to save")
	}
	settings, err := os.ReadFile(filepath.Join(child.Env["CODEX_ELECTRON_USER_DATA_PATH"], "tool-settings.json"))
	if err != nil {
		t.Fatal(err)
	}
	var saved struct{ Engine string }
	if json.Unmarshal(settings, &saved) != nil || saved.Engine != bridge {
		t.Fatal("tool settings did not preserve the bridge reference")
	}
	for _, path := range []string{filepath.Dir(bridge), child.Env["CODEX_ELECTRON_USER_DATA_PATH"], filepath.Join(child.Env["CODEX_HOME"], "sessions")} {
		if err := filepath.WalkDir(path, func(path string, entry os.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if entry.IsDir() {
				return nil
			}
			data, err := os.ReadFile(path)
			if err != nil {
				return err
			}
			for _, secret := range []string{child.Env["TOFA_API_KEY"], child.Env["TOFA_DESKTOP_CONTEXT"]} {
				if secret != "" && strings.Contains(string(data), secret) {
					return errors.New("live launch capability persisted after cleanup")
				}
			}
			return nil
		}); err != nil {
			t.Fatal(err)
		}
	}
	command := exec.Command(bridge, "--version")
	command.Env = []string{"PATH=" + os.Getenv("PATH"), "HOME=" + os.Getenv("HOME")}
	output, err := command.CombinedOutput()
	if err != nil || strings.TrimSpace(string(output)) != "codex-cli 0.155.0-alpha.9.2" {
		t.Fatalf("saved reference is not executable after launch cleanup: %v %s", err, output)
	}
	command = exec.Command(bridge, "--ordinary-probe", "argument with spaces")
	command.Env = []string{"PATH=" + os.Getenv("PATH"), "CODEX_HOME=/ordinary/fixture", "OPENAI_API_KEY=synthetic-native", "TOFA_API_KEY=stale-bearer"}
	output, err = command.CombinedOutput()
	var exit *exec.ExitError
	var ordinary struct {
		Args      []string
		Home      string
		NativeKey string  `json:"native_key"`
		TofaKey   *string `json:"tofa_key"`
	}
	if !errors.As(err, &exit) || exit.ExitCode() != 23 || json.Unmarshal(output, &ordinary) != nil || ordinary.Home != "/ordinary/fixture" || ordinary.NativeKey != "synthetic-native" || ordinary.TofaKey != nil || strings.Join(ordinary.Args, "|") != "--ordinary-probe|argument with spaces" {
		t.Fatal("ordinary bridge invocation lost settings, arguments, exit status, or retained a stale bearer")
	}
	for _, context := range []string{child.Env["TOFA_DESKTOP_CONTEXT"], "", "malformed", "https://127.0.0.1:1", "http://example.com:80", "http://127.0.0.1:0"} {
		command = exec.Command(bridge, "--version")
		command.Env = []string{"PATH=" + os.Getenv("PATH"), "TOFA_DESKTOP_CONTEXT=" + context, "TOFA_API_KEY=" + child.Env["TOFA_API_KEY"]}
		output, err = command.CombinedOutput()
		if err == nil || !strings.Contains(string(output), "context is invalid or expired") {
			t.Fatal("expired or malformed context fell back to the bundled engine")
		}
	}
}

func TestDesktopBundledEngineOverridesAtAppServerBoundary(t *testing.T) {
	installed := os.Getenv("TOFA_TEST_DESKTOP_ENGINE")
	if installed == "" {
		t.Skip("set TOFA_TEST_DESKTOP_ENGINE")
	}
	bundle, capture := desktopFixture(t, "ignore")
	engine := filepath.Join(bundle, "Contents/Resources/codex")
	if err := os.Remove(engine); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(installed, engine); err != nil {
		t.Fatal(err)
	}
	var requests atomic.Int32
	app, _ := adapterFixture(t, func(w http.ResponseWriter, r *http.Request) {
		var request struct{ Model string }
		if json.NewDecoder(r.Body).Decode(&request) != nil || request.Model != "moonshotai/Kimi-K3" {
			t.Error("wrong model reached inference fixture")
		}
		requests.Add(1)
		w.Header().Set("Content-Type", "text/event-stream")
		fmt.Fprint(w, "event: response.created\ndata: {\"type\":\"response.created\",\"response\":{\"id\":\"resp_fixture\",\"status\":\"in_progress\",\"output\":[]}}\n\nevent: response.completed\ndata: {\"type\":\"response.completed\",\"response\":{\"id\":\"resp_fixture\",\"status\":\"completed\",\"output\":[],\"usage\":{\"input_tokens\":1,\"output_tokens\":0,\"total_tokens\":1}}}\n\n")
	}, nil)
	child, _ := liveDesktopFixture(t, app, bundle, capture)
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	engine = child.Env["CODEX_CLI_PATH"]
	command := exec.CommandContext(ctx, engine, "-c", "features.code_mode_host=true", "app-server", "-c", `model_provider="openai"`, "-c", "developer_instructions=\"preserve unrelated override\"", "-c", "features.analytics=false", "-c", "features.shell_snapshot=false", "-c", `model_providers.nebius-tofa.base_url="http://127.0.0.1:1"`, "-c", "plugins.fixture.enabled=false")
	command.Dir = child.Cwd
	for key, value := range child.Env {
		command.Env = append(command.Env, key+"="+value)
	}
	stdin, err := command.StdinPipe()
	if err != nil {
		t.Fatal(err)
	}
	stdout, err := command.StdoutPipe()
	if err != nil {
		t.Fatal(err)
	}
	if err := command.Start(); err != nil {
		t.Fatal(err)
	}
	defer func() { stdin.Close(); command.Process.Kill(); command.Wait() }()
	encoder, decoder := json.NewEncoder(stdin), json.NewDecoder(stdout)
	for id, method := range []string{"initialize", "config/read"} {
		params := map[string]any{"clientInfo": map[string]string{"name": "tofa_fixture", "version": "1"}, "cwd": child.Cwd}
		if err := encoder.Encode(map[string]any{"id": id, "method": method, "params": params}); err != nil {
			t.Fatal(err)
		}
		var response struct {
			ID     *int
			Result struct{ Config map[string]any }
			Error  json.RawMessage
		}
		for {
			if err := decoder.Decode(&response); err != nil {
				t.Fatal(err)
			}
			if response.ID != nil {
				break
			}
		}
		if len(response.Error) != 0 {
			t.Fatalf("engine rejected request: %s", response.Error)
		}
		if method == "config/read" && (response.Result.Config["model_provider"] != "nebius-tofa" || response.Result.Config["developer_instructions"] != "preserve unrelated override") {
			t.Fatal("app-server overrides displaced live routing or lost unrelated settings")
		}
	}
	if err := encoder.Encode(map[string]any{"method": "initialized"}); err != nil {
		t.Fatal(err)
	}
	if err := encoder.Encode(map[string]any{"id": 2, "method": "thread/start", "params": map[string]any{"cwd": child.Cwd, "approvalPolicy": "never", "sandbox": "read-only"}}); err != nil {
		t.Fatal(err)
	}
	var threadID string
	for threadID == "" {
		var response struct {
			ID     *int
			Result struct{ Thread struct{ ID string } }
			Error  json.RawMessage
		}
		if err := decoder.Decode(&response); err != nil {
			t.Fatal(err)
		}
		if response.ID != nil && *response.ID == 2 {
			if len(response.Error) != 0 {
				t.Fatalf("thread/start failed: %s", response.Error)
			}
			threadID = response.Result.Thread.ID
		}
	}
	if err := encoder.Encode(map[string]any{"id": 3, "method": "turn/start", "params": map[string]any{"threadId": threadID, "input": []any{map[string]string{"type": "text", "text": "Synthetic routing check"}}}}); err != nil {
		t.Fatal(err)
	}
	for {
		var response struct {
			Method string
			Error  json.RawMessage
			Params struct{ Turn struct{ Status string } }
		}
		if err := decoder.Decode(&response); err != nil {
			t.Fatal(err)
		}
		if len(response.Error) != 0 {
			t.Fatalf("turn/start failed: %s", response.Error)
		}
		if response.Method == "turn/completed" {
			if response.Params.Turn.Status != "completed" || requests.Load() != 1 {
				t.Fatal("bundled engine did not complete inference through the authenticated fixture route")
			}
			break
		}
	}
	if err := filepath.WalkDir(filepath.Join(child.Env["CODEX_HOME"], "sessions"), func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() {
			return nil
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		if strings.Contains(string(data), child.Env["TOFA_API_KEY"]) || strings.Contains(string(data), child.Env["TOFA_DESKTOP_CONTEXT"]) {
			return errors.New("bundled engine persisted a live launch capability in history")
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
}

type capturedDesktop struct {
	Env map[string]string
	Cwd string
}

func liveDesktopFixture(t *testing.T, app *tofa.App, bundle, capture string) (capturedDesktop, func()) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	done := make(chan struct{})
	var result error
	go func() {
		defer close(done)
		result = app.RunContext(ctx, []string{"launch", "codex-desktop", "--app-bundle", bundle, "--model", "moonshotai/Kimi-K3", "--allow-unverified"})
	}()
	stop := func() { cancel(); <-done }
	t.Cleanup(stop)
	for {
		if raw, err := os.ReadFile(capture); err == nil {
			var child capturedDesktop
			if json.Unmarshal(raw, &child) == nil {
				return child, stop
			}
		}
		select {
		case <-done:
			t.Fatalf("desktop failed before capture: %v", result)
		case <-ctx.Done():
			t.Fatal("desktop capture timed out")
		case <-time.After(20 * time.Millisecond):
		}
	}
}

func TestDesktopBridgeRelaunchAndMismatchedContext(t *testing.T) {
	bundle, capture := desktopFixture(t, "ignore")
	app, _ := adapterFixture(t, nil, nil)
	first, stop := liveDesktopFixture(t, app, bundle, capture)
	bridge := first.Env["CODEX_CLI_PATH"]
	invoke := func(env map[string]string, valid bool) {
		t.Helper()
		command := exec.Command(bridge, "--version")
		for key, value := range env {
			command.Env = append(command.Env, key+"="+value)
		}
		output, err := command.CombinedOutput()
		if valid && (err != nil || !strings.Contains(string(output), "codex-cli")) {
			t.Fatal("valid launch context rejected")
		}
		if !valid && (err == nil || !strings.Contains(string(output), "context")) {
			t.Fatal("invalid launch context reached engine")
		}
	}
	invoke(first.Env, true)
	for _, args := range [][]string{
		{"exec", "--", "app-server", "--ordinary-probe"},
		{"--profile", "app-server", "--ordinary-probe"},
		{"--", "app-server", "--ordinary-probe"},
	} {
		command := exec.Command(bridge, args...)
		for key, value := range first.Env {
			command.Env = append(command.Env, key+"="+value)
		}
		raw, err := command.Output()
		var exit *exec.ExitError
		var probe struct{ Args []string }
		if !errors.As(err, &exit) || exit.ExitCode() != 23 || json.Unmarshal(raw, &probe) != nil || strings.Join(probe.Args, "|") != strings.Join(args, "|") {
			t.Fatal("live bridge treated an argument value as an app-server subcommand")
		}
	}
	originalHome := first.Env["CODEX_HOME"]
	first.Env["CODEX_HOME"] = t.TempDir()
	invoke(first.Env, false)
	first.Env["CODEX_HOME"] = originalHome
	stop()
	invoke(first.Env, false)
	if err := os.Remove(capture); err != nil {
		t.Fatal(err)
	}
	second, stopSecond := liveDesktopFixture(t, app, bundle, capture)
	defer stopSecond()
	if second.Env["CODEX_CLI_PATH"] != bridge || second.Env["TOFA_API_KEY"] == first.Env["TOFA_API_KEY"] || second.Env["TOFA_DESKTOP_CONTEXT"] == first.Env["TOFA_DESKTOP_CONTEXT"] {
		t.Fatal("relaunch did not preserve the bridge and rotate its live route")
	}
	invoke(second.Env, true)
	invoke(first.Env, false)
	second.Env["TOFA_API_KEY"] = first.Env["TOFA_API_KEY"]
	invoke(second.Env, false)
}

func TestDesktopBridgeRefusesUserExecutableOverride(t *testing.T) {
	bundle, capture := desktopFixture(t, "normal")
	userEngine := filepath.Join(t.TempDir(), "my-engine")
	content := []byte("#!/bin/sh\nexit 0\n")
	if err := os.WriteFile(userEngine, content, 0700); err != nil {
		t.Fatal(err)
	}
	t.Setenv("CODEX_CLI_PATH", userEngine)
	app, _ := adapterFixture(t, nil, nil)
	err := app.Run([]string{"launch", "codex-desktop", "--app-bundle", bundle, "--model", "moonshotai/Kimi-K3", "--allow-unverified"})
	if err == nil || !strings.Contains(err.Error(), "user-managed executable override") {
		t.Fatal("user executable override was silently displaced")
	}
	if _, err := os.Stat(capture); !os.IsNotExist(err) {
		t.Fatal("conflicting desktop was started")
	}
	if raw, err := os.ReadFile(userEngine); err != nil || string(raw) != string(content) {
		t.Fatal("user executable was modified")
	}
}

func TestDesktopBridgeRejectsModifiedOwnership(t *testing.T) {
	for _, kind := range []string{"executable", "record", "directory symlink"} {
		t.Run(kind, func(t *testing.T) {
			bundle, capture := desktopFixture(t, "normal")
			app, _ := adapterFixture(t, nil, nil)
			args := []string{"launch", "codex-desktop", "--app-bundle", bundle, "--model", "moonshotai/Kimi-K3", "--allow-unverified"}
			if err := app.Run(args); err != nil {
				t.Fatal(err)
			}
			raw, err := os.ReadFile(capture)
			if err != nil {
				t.Fatal(err)
			}
			var child capturedDesktop
			if err := json.Unmarshal(raw, &child); err != nil {
				t.Fatal(err)
			}
			bridge := child.Env["CODEX_CLI_PATH"]
			path := bridge
			if kind == "record" {
				path = filepath.Join(filepath.Dir(bridge), "owner.json")
			}
			if kind == "directory symlink" {
				root := filepath.Dir(bridge)
				if err := os.Rename(root, root+"-saved"); err != nil {
					t.Fatal(err)
				}
				if err := os.Symlink(root+"-saved", root); err != nil {
					t.Fatal(err)
				}
			} else if err := os.WriteFile(path, []byte("user-managed content"), 0700); err != nil {
				t.Fatal(err)
			}
			if err := os.Remove(capture); err != nil {
				t.Fatal(err)
			}
			if err := app.Run(args); err == nil {
				t.Fatal("modified bridge ownership was accepted")
			}
			if _, err := os.Stat(capture); !os.IsNotExist(err) {
				t.Fatal("app started with conflicting bridge ownership")
			}
			if kind != "directory symlink" {
				if raw, err := os.ReadFile(path); err != nil || string(raw) != "user-managed content" {
					t.Fatal("unowned content was replaced")
				}
			}
		})
	}
}

func TestDesktopBridgePartialInstallationCleansUp(t *testing.T) {
	bundle, _ := desktopFixture(t, "normal")
	executable, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	// A real filesystem write failure while copying the bridge. The limit is
	// scoped to this child process; neither the test runner nor user files change.
	command := exec.Command("/bin/sh", "-c", `ulimit -f 1; trap '' XFSZ; exec "$@"`, "fixture", executable, "--test-desktop-launch", dir, "launch", "codex-desktop", "--app-bundle", bundle, "--model", "moonshotai/Kimi-K3", "--allow-unverified")
	output, err := command.CombinedOutput()
	if err == nil || !strings.Contains(string(output), "file too large") {
		t.Fatalf("copy failure was not exercised: %v %s", err, output)
	}
	entries, err := os.ReadDir(filepath.Join(dir, "desktop-bridge-v1"))
	if err != nil || len(entries) != 0 {
		t.Fatal("partial bridge installation survived a copy failure")
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
