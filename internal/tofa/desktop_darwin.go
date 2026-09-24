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
		{"CFBundleShortVersionString", "26.917.71314"}, {"CFBundleVersion", "10954"}, {"CFBundleExecutable", "ChatGPT"},
	} {
		value, err := exec.CommandContext(ctx, "/usr/libexec/PlistBuddy", "-c", "Print :"+field.key, plist).Output()
		if err != nil || strings.TrimSpace(string(value)) != field.want {
			return bundle, fmt.Errorf("incompatible desktop bundle: expected %s=%s; tested ChatGPT 26.917.71314 (10954), Codex mode; use --app-bundle PATH or launch codex", field.key, field.want)
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
	if err != nil || strings.TrimSpace(string(output)) != "codex-cli 0.155.0-alpha.16.4" {
		return bundle, errors.New("incompatible bundled engine: expected codex-cli 0.155.0-alpha.16.4; use the tested app or launch codex")
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
	profile, err := ordinaryDesktopProfile()
	if err != nil {
		return err
	}
	if err := refuseDesktopOwner(profile); err != nil {
		return err
	}
	if err := refuseDesktopProcesses(ctx, profile); err != nil {
		return err
	}
	lease, err := acquireDesktopLease(profile)
	if err != nil {
		return err
	}
	defer lease.Close()
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
	home, err := desktopHome(ctx)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(home, 0700); err != nil {
		return err
	}
	workspace, err := os.Getwd()
	if err != nil {
		return err
	}
	root, err := prepareDesktopRuntime(a.Dir, profile, a.Out)
	if err != nil {
		return err
	}
	// Only preflight/control state lives here. History, profile and workspace
	// remain at the client's ordinary locations and are never cleanup targets.
	defer func() { result = errors.Join(result, os.RemoveAll(root)) }()
	if err := integrateDesktopProvider(ctx, bundle.engine, home, profile, workspace, root, 0); err != nil {
		return err
	}
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
	route := &desktopRoute{Bridge: bridge, Engine: bundle.engine, Home: home, ready: make(chan struct{}), claim: make(chan int, 1)}
	adapter.desktop.Store(route)
	shellDir, err := prepareDesktopShell(ctx, root)
	if err != nil {
		return err
	}
	env := append(desktopEnv(home, profile, adapter.token), "ZDOTDIR="+shellDir, "CODEX_CLI_PATH="+bridge, "TOFA_DESKTOP_CONTEXT="+adapter.endpoint)
	owned := func(ownerContext context.Context, pid int) error {
		catalog, err := prepareDesktopCatalog(ownerContext, bundle.engine, home, profile, workspace, *model, root)
		if err != nil {
			return err
		}
		route.Overrides = []string{
			"model=" + strconv.Quote(*model), `model_provider="nebius-tofa"`, "model_catalog_json=" + strconv.Quote(catalog), `web_search="disabled"`,
			`model_providers.nebius-tofa.name="Nebius Token Factory"`, "model_providers.nebius-tofa.base_url=" + strconv.Quote(adapter.endpoint),
			`model_providers.nebius-tofa.env_key="TOFA_API_KEY"`, `model_providers.nebius-tofa.wire_api="responses"`,
			`model_providers.nebius-tofa.requires_openai_auth=false`, `model_providers.nebius-tofa.supports_websockets=false`,
			`model_providers.nebius-tofa.request_max_retries=0`, `model_providers.nebius-tofa.stream_max_retries=0`,
		}
		// Resolve live overrides through the actual engine before publishing them
		// to the blocked desktop bridge or editing ordinary integration entries.
		checkEnv := desktopEnv(home, profile, adapter.token)
		if err := checkDesktopRoutingOverrides(ownerContext, bundle.engine, workspace, checkEnv, *model, adapter.endpoint, catalog, route.Overrides); err != nil {
			return err
		}
		if !desktopOwnsNativeProfile(profile, pid) {
			return errors.New("desktop native profile ownership lost before integration; owned launch cancelled")
		}
		if err := integrateDesktopProvider(ownerContext, bundle.engine, home, profile, workspace, root, pid); err != nil {
			return err
		}
		close(route.ready)
		fmt.Fprintln(a.Out, "Native model catalog resolved and merged for this launch. Relaunch after account or catalog changes; only the announced automatic-title contract has an auxiliary route through Token Factory.")
		fmt.Fprintf(a.Out, "Launching Codex desktop with %s (unverified), using ordinary history and profile.\nToken Factory history remains readable after exit; relaunch through tofa with --model moonshotai/Kimi-K3 to continue. Choosing GPT does not migrate providers.\nAutomatic title generation can fail; other auxiliary requests and compaction remain unsupported. Keep this terminal open.\n", *model)
		fmt.Fprintln(a.Out, "Desktop environment probe isolated; coding commands retain normal shell startup.")
		return nil
	}
	// No deep link or model-selection argument is forwarded to a possible
	// incumbent. The bridge waits for ownership and configuration qualification.
	command := exec.Command(bundle.executable, "--user-data-dir="+profile)
	command.Env, command.Dir = env, workspace
	return runDesktopProcess(adapter.context, command, bundle.engine, profile, route.claim, owned)
}

// Resolve live routing against ordinary settings without printing private policy.
func checkDesktopRoutingOverrides(ctx context.Context, engine, workspace string, env []string, model, endpoint, catalog string, overrides []string) error {
	ctx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()
	args := []string{"app-server"}
	for _, override := range overrides {
		args = append(args, "-c", override)
	}
	command := exec.CommandContext(ctx, engine, args...)
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
				Config       map[string]any `json:"config"`
				Requirements map[string]any `json:"requirements"`
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
			if err := checkDesktopRequirements(response.Result.Requirements); err != nil {
				return err
			}
			continue
		}
		config := response.Result.Config
		for key, value := range map[string]string{"model": model, "model_provider": "nebius-tofa", "model_catalog_json": catalog, "web_search": "disabled"} {
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
			case "name", "base_url", "env_key", "env_key_instructions", "wire_api", "requires_openai_auth", "supports_websockets", "request_max_retries", "stream_max_retries":
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

func runDesktopProcess(ctx context.Context, command *exec.Cmd, engine, profile string, claim <-chan int, owned func(context.Context, int) error) error {
	if err := refuseDesktopOwner(profile); err != nil {
		return err
	}
	if err := refuseDesktopProcesses(ctx, profile); err != nil {
		return err
	}
	command.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	if err := command.Start(); err != nil {
		return errors.New("could not start owned desktop application")
	}
	// Never signal the ordinary app. Every signal targets this child's new group.
	defer syscall.Kill(-command.Process.Pid, syscall.SIGKILL)
	ownerContext, ownerExited := context.WithCancel(ctx)
	defer ownerExited()
	done := make(chan error, 1)
	go func() {
		err := command.Wait()
		ownerExited()
		done <- err
	}()
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
	// Abort an unclaimed contender promptly. Native singleton collision handling
	// must never be allowed to time out and take over an unresponsive incumbent.
	claimDeadline := time.NewTimer(3 * time.Second)
	defer claimDeadline.Stop()
	select {
	case <-ctx.Done():
		return stop(ctx.Err())
	case err := <-done:
		if err != nil {
			return err
		}
		return errors.New("another desktop won profile ownership; Quit it manually and relaunch through tofa; no process was adopted")
	case <-claimDeadline.C:
		return stop(errors.New("desktop ownership handshake timed out; no existing process was adopted"))
	case pid := <-claim:
		if pid != command.Process.Pid || !desktopOwnsNativeProfile(profile, pid) {
			return stop(errors.New("competing desktop launch or missing native profile ownership; Quit Codex desktop and relaunch"))
		}
	}
	if err := owned(ownerContext, command.Process.Pid); err != nil {
		return stop(err)
	}
	if !desktopOwnsNativeProfile(profile, command.Process.Pid) {
		return stop(errors.New("desktop native profile ownership changed; owned launch cancelled"))
	}
	tick := time.NewTicker(100 * time.Millisecond)
	defer tick.Stop()
	startup := time.NewTimer(15 * time.Second)
	defer startup.Stop()
	engineSeen := false
	var engineExit <-chan time.Time
	var shutdown *time.Timer
	defer func() {
		if shutdown != nil {
			shutdown.Stop()
		}
	}()
	for {
		select {
		case err := <-done:
			if err == nil && !engineSeen {
				return errors.New("desktop exited before starting its owned app-server; no ordinary instance was adopted")
			}
			return err
		case <-ctx.Done():
			return stop(ctx.Err())
		case <-engineExit:
			return stop(errors.New("owned desktop app-server exited; launch cancelled"))
		case <-startup.C:
			if !engineSeen {
				return stop(errors.New("desktop did not start its owned bundled app-server within 15 seconds"))
			}
		case <-tick.C:
			if !desktopOwnsNativeProfile(profile, command.Process.Pid) {
				select {
				case err := <-done:
					return err
				case <-time.After(100 * time.Millisecond):
					return stop(errors.New("desktop native profile ownership lost; owned launch cancelled"))
				}
			}
			if engineExit != nil {
				continue
			}
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
				// Electron can take longer than a polling interval to finish
				// after its engine exits. Keep ownership/cancellation checks
				// active during this bounded wait; never adopt a replacement.
				shutdown = time.NewTimer(2 * time.Second)
				engineExit = shutdown.C
			}
			engineSeen = engineSeen || found
		}
	}
}
