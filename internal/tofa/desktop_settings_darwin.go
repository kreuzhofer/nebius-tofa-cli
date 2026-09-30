package tofa

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
)

// The desktop's default picker writes model/effort into the ordinary user
// config before handling the engine's okOverridden result. Those writes leak
// into concurrent CLI clients even though launch flags override the desktop.
// Keep this exact write contract session-scoped: ask the engine for the current
// config version without writing, then return an explicit override result. The
// pinned renderer retains its requested selection locally on okOverridden and
// supplies it when starting a thread. All other protocol traffic stays native.
func runDesktopSettingsBridge(engine string, args, env []string, home string) error {
	configFile, err := filepath.EvalSymlinks(filepath.Join(home, "config.toml"))
	if err != nil {
		return errors.New("desktop model isolation could not resolve the ordinary configuration file")
	}
	command := exec.Command(engine, args...)
	command.Env, command.Stderr = env, os.Stderr
	input, err := command.StdinPipe()
	if err != nil {
		return err
	}
	output, err := command.StdoutPipe()
	if err != nil {
		input.Close()
		return err
	}
	if err := command.Start(); err != nil {
		return err
	}
	defer func() { command.Process.Kill(); command.Wait() }()
	var writesMu, pendingMu sync.Mutex
	pending := map[string]json.RawMessage{}
	write := func(value any) error {
		writesMu.Lock()
		defer writesMu.Unlock()
		return json.NewEncoder(os.Stdout).Encode(value)
	}
	failure := func(id json.RawMessage, message string) error {
		return write(map[string]any{"id": id, "error": map[string]any{"code": -32600, "message": message}})
	}
	inputDone := make(chan error, 1)
	go func() {
		scanner := bufio.NewScanner(os.Stdin)
		scanner.Buffer(make([]byte, 64<<10), 64<<20)
		for scanner.Scan() {
			line := append([]byte(nil), scanner.Bytes()...)
			var request struct {
				ID     json.RawMessage
				Method string
				Params json.RawMessage
			}
			if json.Unmarshal(line, &request) != nil {
				inputDone <- errors.New("invalid desktop protocol request")
				command.Process.Kill()
				return
			}
			intercept, valid := desktopModelWrite(request.Method, request.Params)
			if intercept {
				if !valid || len(request.ID) == 0 || string(request.ID) == "null" {
					if err := failure(request.ID, "tofa: unsupported desktop model-default write; use the model picker for this session; unrelated settings were not changed"); err != nil {
						inputDone <- err
						command.Process.Kill()
						return
					}
					continue
				}
				pendingMu.Lock()
				pending[string(request.ID)] = request.Params
				pendingMu.Unlock()
				line, _ = json.Marshal(map[string]any{"id": request.ID, "method": "config/read", "params": map[string]any{"includeLayers": true}})
			}
			if _, err := input.Write(append(line, '\n')); err != nil {
				inputDone <- err
				return
			}
		}
		input.Close()
		inputDone <- scanner.Err()
		if scanner.Err() != nil {
			command.Process.Kill()
		}
	}()
	scanner := bufio.NewScanner(output)
	scanner.Buffer(make([]byte, 64<<10), 64<<20)
	for scanner.Scan() {
		line := append(json.RawMessage(nil), scanner.Bytes()...)
		var response struct {
			ID     json.RawMessage
			Method string
			Result json.RawMessage
			Error  json.RawMessage
		}
		if json.Unmarshal(line, &response) != nil {
			return errors.New("invalid bundled engine protocol response")
		}
		var params json.RawMessage
		intercepted := false
		// Server requests (including approvals) have their own ID namespace.
		// They must never consume a pending client configuration response.
		if response.Method == "" {
			pendingMu.Lock()
			params, intercepted = pending[string(response.ID)]
			delete(pending, string(response.ID))
			pendingMu.Unlock()
		}
		if intercepted && (len(response.Error) == 0 || string(response.Error) == "null") {
			var result struct {
				Config map[string]any
				Layers []struct {
					Name    map[string]any
					Version string
				}
			}
			var request struct{ ExpectedVersion *string }
			json.Unmarshal(params, &request)
			if json.Unmarshal(response.Result, &result) != nil {
				return errors.New("invalid engine configuration response")
			}
			version := ""
			for _, layer := range result.Layers {
				if layer.Name["type"] == "user" && layer.Name["file"] == configFile {
					version = layer.Version
				}
			}
			if version == "" || (request.ExpectedVersion != nil && *request.ExpectedVersion != version) {
				if err := failure(response.ID, "tofa: desktop model selection could not verify the current settings version; refresh and retry; CLI defaults were not changed"); err != nil {
					return err
				}
				continue
			}
			if err := write(map[string]any{"id": response.ID, "result": map[string]any{
				"status": "okOverridden", "version": version, "filePath": configFile,
				"overriddenMetadata": map[string]any{"message": "tofa: model selection applies only to this desktop session; CLI defaults were not written.", "effectiveValue": result.Config["model"], "overridingLayer": map[string]any{"name": map[string]any{"type": "sessionFlags"}, "version": version}},
			}}); err != nil {
				return err
			}
			continue
		}
		if err := write(line); err != nil {
			return err
		}
	}
	if err := scanner.Err(); err != nil {
		return fmt.Errorf("desktop engine protocol: %w", err)
	}
	if err := command.Wait(); err != nil {
		return err
	}
	select {
	case err := <-inputDone:
		return err
	default:
		return nil
	}
}

// Accept only the pinned model picker writes. Never partially apply a mixed
// model/settings batch, and never intercept unrelated engine settings.
func desktopModelWrite(method string, params json.RawMessage) (intercept, valid bool) {
	if method != "config/batchWrite" && method != "config/value/write" {
		return false, false
	}
	type edit struct {
		KeyPath       string
		Value         json.RawMessage
		MergeStrategy string
	}
	var request struct {
		Edits         []edit
		KeyPath       string
		Value         json.RawMessage
		MergeStrategy string
		FilePath      *string
	}
	if json.Unmarshal(params, &request) != nil {
		return true, false
	}
	edits := request.Edits
	if method == "config/value/write" {
		edits = []edit{{request.KeyPath, request.Value, request.MergeStrategy}}
	}
	valid = request.FilePath == nil && len(edits) > 0
	for _, change := range edits {
		key := change.KeyPath
		modelKey := key == "model" || key == "model_reasoning_effort" || key == "service_tier" || key == "plan_mode_reasoning_effort"
		if modelKey || key == "" || strings.HasPrefix(key, "model.") || strings.HasPrefix(key, "profiles") {
			intercept = true
		}
		if !modelKey || (change.MergeStrategy != "replace" && change.MergeStrategy != "upsert") {
			valid = false
		}
		var value *string
		if len(change.Value) == 0 || json.Unmarshal(change.Value, &value) != nil {
			valid = false
		}
	}
	return intercept, valid
}
