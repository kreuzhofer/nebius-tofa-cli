package tofa

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"runtime"
	"slices"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"
)

const desktopModel = "moonshotai/Kimi-K3"

type desktopBundle struct{ executable, engine string }

type desktopOutput struct {
	mu     sync.Mutex
	writer io.Writer
}

func (out *desktopOutput) Write(p []byte) (int, error) {
	out.mu.Lock()
	defer out.mu.Unlock()
	return out.writer.Write(p)
}

func discoverDesktop(ctx context.Context, path string) (desktopBundle, error) {
	var bundle desktopBundle
	version, err := exec.CommandContext(ctx, "/usr/bin/sw_vers", "-productVersion").Output()
	if err != nil || runtime.GOARCH != "arm64" || strings.TrimSpace(string(version)) != "26.6.2" {
		return bundle, errors.New("codex-desktop is experimental: tested only on macOS 26.6.2 arm64; use launch codex on other platforms")
	}
	if path == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return bundle, err
		}
		for _, candidate := range []string{"/Applications/ChatGPT.app", "/Applications/Codex.app", filepath.Join(home, "Applications/ChatGPT.app"), filepath.Join(home, "Applications/Codex.app")} {
			if _, err := os.Stat(candidate); err == nil {
				path = candidate
				break
			}
		}
	}
	if path == "" {
		return bundle, errors.New("ChatGPT desktop with Codex mode is not installed; install the tested app or pass --app-bundle PATH")
	}
	path, err = filepath.Abs(path)
	if err != nil {
		return bundle, err
	}
	plist := filepath.Join(path, "Contents/Info.plist")
	for _, field := range []struct{ key, want string }{
		{"CFBundleIdentifier", "com.openai.codex"}, {"CFBundleName", "ChatGPT"},
		{"CFBundleShortVersionString", "26.915.31945"}, {"CFBundleVersion", "9922"}, {"CFBundleExecutable", "ChatGPT"},
	} {
		value, err := exec.CommandContext(ctx, "/usr/libexec/PlistBuddy", "-c", "Print :"+field.key, plist).Output()
		if err != nil || strings.TrimSpace(string(value)) != field.want {
			return bundle, fmt.Errorf("incompatible desktop bundle: expected %s=%s; tested ChatGPT 26.915.31945 (9922), Codex mode; use --app-bundle PATH or launch codex", field.key, field.want)
		}
	}
	bundle = desktopBundle{filepath.Join(path, "Contents/MacOS/ChatGPT"), filepath.Join(path, "Contents/Resources/codex")}
	for _, executable := range []string{bundle.executable, bundle.engine} {
		info, err := os.Stat(executable)
		if err != nil || !info.Mode().IsRegular() || info.Mode().Perm()&0111 == 0 {
			return bundle, errors.New("desktop bundle is missing its executable or bundled Codex engine; reinstall the tested app")
		}
	}
	probe, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	probeHome, err := os.MkdirTemp("", "tofa-desktop-version-")
	if err != nil {
		return bundle, err
	}
	defer os.RemoveAll(probeHome)
	command := exec.CommandContext(probe, bundle.engine, "--version")
	command.Env = desktopEnv(probeHome, probeHome, "")
	output, err := command.Output()
	if err != nil || strings.TrimSpace(string(output)) != "codex-cli 0.155.0-alpha.9.2" {
		return bundle, errors.New("incompatible bundled engine: expected codex-cli 0.155.0-alpha.9.2; use the tested app or launch codex")
	}
	return bundle, nil
}

// Keep HOME for native managed policy lookup, but never inherit routing, auth,
// Electron control variables, or the real provider key into the owned instance.
func desktopEnv(home, electron, token string) []string {
	env := []string{}
	for _, entry := range os.Environ() {
		name, _, _ := strings.Cut(entry, "=")
		name = strings.ToUpper(name)
		if strings.HasPrefix(name, "CODEX_") || strings.HasPrefix(name, "OPENAI_") || strings.HasPrefix(name, "TOFA_") || strings.HasPrefix(name, "ELECTRON_") || strings.HasPrefix(name, "DYLD_") || name == "NODE_OPTIONS" || name == "ZDOTDIR" {
			continue
		}
		env = append(env, entry)
	}
	return append(env, "CODEX_HOME="+home, "CODEX_ELECTRON_USER_DATA_PATH="+electron, "TOFA_API_KEY="+token)
}

func (a *App) launchDesktop(ctx context.Context, s Store, args []string) (result error) {
	// Startup and HTTP handlers share the caller's output, which need not be
	// concurrency-safe. Copy the app so this per-launch lock stays with handlers.
	launch := *a
	launch.Out = &desktopOutput{writer: a.Out}
	a = &launch
	fs := flags("launch codex-desktop")
	model := fs.String("model", "", "")
	path := fs.String("app-bundle", "", "")
	project := fs.String("project-id", "", "")
	allow := fs.Bool("allow-unverified", false, "")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() != 0 {
		return errors.New("codex-desktop does not accept client arguments or routing overrides")
	}
	if !*allow {
		return errors.New("experimental desktop/model combination requires --allow-unverified")
	}
	if *model != desktopModel {
		return errors.New("codex-desktop requires explicit --model moonshotai/Kimi-K3; other desktop combinations have not been tested")
	}
	ctx, stop := signal.NotifyContext(ctx, os.Interrupt, syscall.SIGTERM)
	defer stop()
	bundle, err := discoverDesktop(ctx, *path)
	if err != nil {
		return err
	}
	bridge, err := installDesktopBridge(a.Dir, bundle.engine)
	if err != nil {
		return err
	}
	c, key, err := s.Credentials()
	if err != nil {
		return err
	}
	if *project != "" {
		c.ProjectID = *project
	}
	if !validText(c.ProjectID, 256) {
		return errors.New("invalid project ID")
	}
	models, err := a.models(c.ProjectID, key)
	if err != nil {
		return err
	}
	found := false
	for _, candidate := range models {
		if candidate.ID == *model {
			found = true
		}
	}
	if !found {
		return errors.New("selected model is not available in this project's catalog")
	}
	catalog, err := prepareModelCatalog(*model, "")
	if err != nil {
		return err
	}
	if catalog == "" {
		return errors.New("desktop launch requires verified bundled model metadata")
	}
	defer func() { result = errors.Join(result, os.Remove(catalog)) }()
	parent := filepath.Join(a.Dir, "desktop-sessions")
	if err := privateDir(parent); err != nil {
		return err
	}
	root, err := os.MkdirTemp(parent, "session-")
	if err != nil {
		return err
	}
	home, electron, workspace := filepath.Join(root, "codex"), filepath.Join(root, "electron"), filepath.Join(root, "workspace")
	for _, dir := range []string{home, electron, workspace} {
		if err := os.Mkdir(dir, 0700); err != nil {
			return err
		}
	}
	fmt.Fprintf(a.Out, "Launching ChatGPT desktop, Codex mode, with %s (unverified).\nConversations and workspace retained at: %s\nExperimental limitations: automatic title generation can fail; auxiliary models and compaction are unsupported. Shared ordinary-desktop history requires #34.\n", *model, root)
	adapter, err := a.startAdapter(ctx, c.ProjectID, key, *model)
	if err != nil {
		return err
	}
	defer func() {
		result = errors.Join(result, adapter.close())
		if err := <-adapter.done; err != nil {
			result = errors.Join(result, errors.New("request adapter stopped unexpectedly; desktop launch cancelled"))
		}
	}()
	configPath := filepath.Join(home, "config.toml")
	config := "model = " + strconv.Quote(*model) + "\nmodel_provider = \"nebius-tofa\"\nmodel_catalog_json = " + strconv.Quote(catalog) + "\nsqlite_home = " + strconv.Quote(home) + "\nlog_dir = " + strconv.Quote(filepath.Join(home, "logs")) + "\ncli_auth_credentials_store = \"file\"\nweb_search = \"disabled\"\n\n[model_providers.nebius-tofa]\nname = \"Nebius Token Factory\"\nbase_url = " + strconv.Quote(adapter.endpoint) + "\nenv_key = \"TOFA_API_KEY\"\nwire_api = \"responses\"\nrequires_openai_auth = false\nsupports_websockets = false\nrequest_max_retries = 0\nstream_max_retries = 0\n"
	if err := os.WriteFile(configPath, []byte(config), 0600); err != nil {
		return err
	}
	defer func() { result = errors.Join(result, os.Remove(configPath)) }()
	// Use one source of routing settings for the isolated config and the bridge.
	// This is launcher-generated TOML, never an external configuration parser.
	var overrides []string
	section := ""
	for _, line := range strings.Split(config, "\n") {
		if strings.HasPrefix(line, "[") {
			section = strings.Trim(line, "[]") + "."
			continue
		}
		if line != "" {
			overrides = append(overrides, section+line)
		}
	}
	adapter.desktop.Store(&desktopRoute{Bridge: bridge, Engine: bundle.engine, Home: home, Overrides: overrides})
	shellDir, err := prepareDesktopShell(ctx, root)
	if err != nil {
		return err
	}
	defer func() {
		result = errors.Join(result, os.Remove(filepath.Join(shellDir, ".zshenv")), os.Remove(shellDir))
	}()
	env := append(desktopEnv(home, electron, adapter.token), "ZDOTDIR="+shellDir, "CODEX_CLI_PATH="+bridge, "TOFA_DESKTOP_CONTEXT="+adapter.endpoint)
	if err := checkDesktopRouting(adapter.context, bridge, workspace, env, *model, adapter.endpoint, catalog); err != nil {
		return err
	}
	fmt.Fprintln(a.Out, "Bundled engine effective routing checked; verified provider metadata loaded. Keep this terminal open; Ctrl-C stops the owned desktop instance.")
	fmt.Fprintln(a.Out, "Desktop environment probe isolated: using launch-time variables; coding commands retain normal shell startup.")
	command := exec.Command(bundle.executable, "--user-data-dir="+electron, "codex://threads/new?mode=codex")
	command.Env, command.Dir = env, workspace
	return runDesktopProcess(adapter.context, command, bundle.engine)
}

// Ask the bundled engine to resolve config/policy in the exact isolated home and
// workspace. Never print its response: inherited policy can contain private data.
func checkDesktopRouting(ctx context.Context, engine, workspace string, env []string, model, endpoint, catalog string) error {
	ctx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()
	command := exec.CommandContext(ctx, engine, "app-server")
	command.Env, command.Dir = env, workspace
	command.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	stdin, err := command.StdinPipe()
	if err != nil {
		return err
	}
	stdout, err := command.StdoutPipe()
	if err != nil {
		return err
	}
	if err := command.Start(); err != nil {
		return errors.New("could not start bundled engine routing check")
	}
	defer func() { stdin.Close(); syscall.Kill(-command.Process.Pid, syscall.SIGKILL); command.Wait() }()
	encoder, decoder := json.NewEncoder(stdin), json.NewDecoder(io.LimitReader(stdout, 4<<20))
	for _, request := range []struct {
		method string
		params any
	}{
		{"initialize", map[string]any{"clientInfo": map[string]string{"name": "tofa", "version": "experimental"}}},
		{"config/read", map[string]any{"includeLayers": true, "cwd": workspace}},
		{"configRequirements/read", map[string]any{}},
	} {
		if encoder.Encode(map[string]any{"id": 1, "method": request.method, "params": request.params}) != nil {
			return errors.New("desktop routing check failed to send request")
		}
		var response struct {
			ID     *int            `json:"id"`
			Error  json.RawMessage `json:"error"`
			Result struct {
				Config       map[string]any             `json:"config"`
				Requirements map[string]json.RawMessage `json:"requirements"`
			} `json:"result"`
		}
		for {
			if err := decoder.Decode(&response); err != nil {
				return errors.New("desktop effective routing check failed; inspect installed client and managed policy")
			}
			if response.ID != nil {
				break
			}
		}
		if len(response.Error) != 0 && string(response.Error) != "null" {
			return errors.New("bundled engine rejected configuration; inspect managed policy without disabling it")
		}
		if request.method == "initialize" {
			if encoder.Encode(map[string]any{"method": "initialized"}) != nil {
				return errors.New("desktop initialization failed")
			}
			continue
		}
		if request.method == "configRequirements/read" {
			// Dynamic loopback routing and isolated state cannot safely satisfy
			// managed routing/state overrides. Fail closed; never edit that policy.
			for _, key := range []string{"modelProvider", "modelProviders", "modelCatalogJson", "models", "sqliteHome", "logDir", "cliAuthCredentialsStore", "allowedLoginMethods", "enforceResidency"} {
				if value := response.Result.Requirements[key]; len(value) != 0 && string(value) != "null" {
					return fmt.Errorf("managed %s requirement is not qualified for isolated desktop routing; contact your administrator or use an approved client", key)
				}
			}
			continue
		}
		config := response.Result.Config
		for key, value := range map[string]string{"model": model, "model_provider": "nebius-tofa", "model_catalog_json": catalog, "web_search": "disabled", "cli_auth_credentials_store": "file", "sqlite_home": filepath.Dir(workspace) + "/codex", "log_dir": filepath.Dir(workspace) + "/codex/logs"} {
			if config[key] != value {
				return fmt.Errorf("desktop effective routing conflict for %s; inspect policy and use launch codex if necessary", key)
			}
		}
		providers, _ := config["model_providers"].(map[string]any)
		provider, _ := providers["nebius-tofa"].(map[string]any)
		for key, value := range provider {
			if value == nil {
				continue
			} // The engine serializes absent optional settings as null.
			if key == "supports_standalone_web_search" && value == false {
				continue
			}
			switch key {
			case "name", "base_url", "env_key", "wire_api", "requires_openai_auth", "supports_websockets", "request_max_retries", "stream_max_retries":
			default:
				return fmt.Errorf("desktop effective provider has unexpected %s setting; launch cancelled", key)
			}
		}
		for key, value := range map[string]any{"base_url": endpoint, "env_key": "TOFA_API_KEY", "wire_api": "responses", "requires_openai_auth": false, "supports_websockets": false} {
			if provider[key] != value {
				return fmt.Errorf("desktop effective provider conflict for %s; launch cancelled", key)
			}
		}
	}
	return nil
}

func runDesktopProcess(ctx context.Context, command *exec.Cmd, engine string) error {
	command.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	if err := command.Start(); err != nil {
		return errors.New("could not start owned desktop application")
	}
	// Never signal the ordinary app. Every signal targets this child's new group.
	defer syscall.Kill(-command.Process.Pid, syscall.SIGKILL)
	done := make(chan error, 1)
	go func() { done <- command.Wait() }()
	stop := func(cause error) error {
		syscall.Kill(-command.Process.Pid, syscall.SIGTERM)
		select {
		case <-done:
		case <-time.After(2 * time.Second):
			syscall.Kill(-command.Process.Pid, syscall.SIGKILL)
			<-done
		}
		return cause
	}
	tick := time.NewTicker(100 * time.Millisecond)
	defer tick.Stop()
	startup := time.NewTimer(15 * time.Second)
	defer startup.Stop()
	engineSeen := false
	for {
		select {
		case err := <-done:
			if err == nil && !engineSeen {
				return errors.New("desktop exited before starting its owned app-server; no ordinary instance was adopted")
			}
			return err
		case <-ctx.Done():
			return stop(ctx.Err())
		case <-startup.C:
			if !engineSeen {
				return stop(errors.New("desktop did not start its owned bundled app-server within 15 seconds"))
			}
		case <-tick.C:
			probe, cancel := context.WithTimeout(ctx, time.Second)
			output, err := exec.CommandContext(probe, "/bin/ps", "-axo", "pid=,pgid=,stat=,args=").Output()
			cancel()
			if ctx.Err() != nil {
				return stop(ctx.Err())
			}
			if err != nil {
				return stop(errors.New("could not inspect owned desktop app-server; launch cancelled"))
			}
			found := false
			for _, line := range strings.Split(string(output), "\n") {
				fields := strings.Fields(line)
				if len(fields) < 4 || fields[1] != strconv.Itoa(command.Process.Pid) || strings.HasPrefix(fields[2], "Z") {
					continue
				}
				if strings.Contains(line, engine+" ") && slices.Contains(fields[3:], "app-server") {
					found = true
					break
				}
			}
			if engineSeen && !found {
				// During normal shutdown the engine can exit just before Electron.
				select {
				case err := <-done:
					return err
				case <-time.After(200 * time.Millisecond):
					return stop(errors.New("owned desktop app-server exited; launch cancelled"))
				}
			}
			engineSeen = engineSeen || found
		}
	}
}
