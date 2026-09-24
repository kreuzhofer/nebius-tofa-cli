package tofa_test

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"sync/atomic"
	"testing"
)

func TestDesktopHistoryRoundTripRequiresFreshLaunch(t *testing.T) {
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
	var nativeCalls, tofaCalls atomic.Int32
	respond := func(w http.ResponseWriter, item map[string]any) {
		w.Header().Set("Content-Type", "text/event-stream")
		for _, event := range []struct {
			kind  string
			value map[string]any
		}{
			{"response.created", map[string]any{"response": map[string]any{"id": "response_fixture", "status": "in_progress", "output": []any{}}}},
			{"response.output_item.added", map[string]any{"output_index": 0, "item": item}},
			{"response.output_item.done", map[string]any{"output_index": 0, "item": item}},
			{"response.completed", map[string]any{"response": map[string]any{"id": "response_fixture", "status": "completed", "output": []any{item}, "usage": map[string]int{"input_tokens": 10, "output_tokens": 5, "total_tokens": 15}}}},
		} {
			event.value["type"] = event.kind
			data, _ := json.Marshal(event.value)
			fmt.Fprintf(w, "event: %s\ndata: %s\n\n", event.kind, data)
		}
	}
	answer := func(text string) map[string]any {
		return map[string]any{"id": "message_fixture", "type": "message", "role": "assistant", "status": "completed", "content": []any{map[string]any{"type": "output_text", "text": text, "annotations": []any{}}}}
	}
	native := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			w.WriteHeader(http.StatusUpgradeRequired)
			return
		}
		var request struct{ Model string }
		if json.NewDecoder(r.Body).Decode(&request) != nil || request.Model != "gpt-6-astra" || r.Header.Get("Authorization") != "Bearer synthetic-native" {
			t.Error("native conversation routing changed")
		}
		nativeCalls.Add(1)
		respond(w, answer("Native history answer"))
	}))
	defer native.Close()
	app, _ := adapterFixture(t, func(w http.ResponseWriter, r *http.Request) {
		var request struct {
			Model string
			Input []map[string]any
		}
		if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
			t.Error(err)
			w.WriteHeader(400)
			return
		}
		if request.Model != "moonshotai/Kimi-K3" {
			t.Error("Token Factory model changed")
		}
		tofaCalls.Add(1)
		for _, item := range request.Input {
			if item["type"] == "function_call_output" {
				if !strings.Contains(fmt.Sprint(item["output"]), "history-tool-result") {
					t.Error("coding tool result missing from inference history")
				}
				respond(w, answer("Token Factory history answer"))
				return
			}
		}
		respond(w, map[string]any{"id": "function_fixture", "type": "function_call", "call_id": "history_call", "name": "exec_command", "arguments": `{"cmd":"printf history-tool-result","max_output_tokens":100}`, "status": "completed"})
	}, nil)
	home := filepath.Join(os.Getenv("HOME"), ".codex")
	workspace := filepath.Join(filepath.Dir(capture), "workspace")
	for _, path := range []string{home, workspace} {
		if err := os.MkdirAll(path, 0700); err != nil {
			t.Fatal(err)
		}
	}
	workspace, err := filepath.EvalSymlinks(workspace)
	if err != nil {
		t.Fatal(err)
	}
	config := "model = \"gpt-6-astra\"\ncli_auth_credentials_store = \"file\"\nopenai_base_url = " + fmt.Sprintf("%q", native.URL) + "\n"
	if err := os.WriteFile(filepath.Join(home, "config.toml"), []byte(config), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(home, "auth.json"), []byte(`{"OPENAI_API_KEY":"synthetic-native"}`), 0600); err != nil {
		t.Fatal(err)
	}
	ordinary := capturedDesktop{Cwd: workspace, Env: map[string]string{"PATH": os.Getenv("PATH"), "HOME": os.Getenv("HOME"), "CODEX_HOME": home, "CODEX_CLI_PATH": installed}}
	e := openDesktopEngine(t, ordinary)
	started := e.call("thread/start", map[string]any{"cwd": workspace, "historyMode": "paginated", "model": "gpt-6-astra", "modelProvider": "openai", "approvalPolicy": "never", "sandbox": "read-only"})
	nativeID := started["thread"].(map[string]any)["id"].(string)
	e.turnText(nativeID, "completed", "Native first message")
	e.call("thread/name/set", map[string]string{"threadId": nativeID, "name": "Native title"})
	e.close()
	child, stop := liveDesktopFixture(t, app, bundle, capture)
	e = openDesktopEngine(t, child)
	resumed := e.call("thread/resume", map[string]any{"threadId": nativeID, "model": nil, "modelProvider": nil})
	if resumed["modelProvider"] != "openai" || resumed["model"] != "gpt-6-astra" {
		t.Fatal("native identity changed")
	}
	e.turnText(nativeID, "completed", "Native second message during tofa")
	started = e.call("thread/start", map[string]any{"cwd": workspace, "historyMode": "paginated", "approvalPolicy": "never", "sandbox": "read-only"})
	tofaID := started["thread"].(map[string]any)["id"].(string)
	e.turnText(tofaID, "completed", "Token Factory first message")
	e.call("thread/name/set", map[string]string{"threadId": tofaID, "name": "Token Factory title"})
	e.close()
	stop()
	ordinary.Env["CODEX_CLI_PATH"] = child.Env["CODEX_CLI_PATH"]
	ordinary.Env["TOFA_API_KEY"] = child.Env["TOFA_API_KEY"] // Stale credentials must not revive ordinary inference.
	ordinary.Env["TOFA_DESKTOP_INACTIVE"] = "synthetic-inherited-credential"
	want := []string{"user:Token Factory first message", "tool:printf history-tool-result:history-tool-result", "assistant:Token Factory history answer"}
	check := func(e *desktopEngine) {
		t.Helper()
		listed := e.call("thread/list", map[string]any{"modelProviders": []string{}, "useStateDbOnly": true})["data"].([]any)
		ids := map[string]int{}
		for _, raw := range listed {
			ids[raw.(map[string]any)["id"].(string)]++
		}
		if len(ids) != 2 || ids[nativeID] != 1 || ids[tofaID] != 1 {
			t.Fatalf("history identities changed: %v", ids)
		}
		for _, tc := range []struct {
			id, title, provider string
			messages            []string
		}{
			{nativeID, "Native title", "openai", []string{"user:Native first message", "assistant:Native history answer", "user:Native second message during tofa", "assistant:Native history answer"}},
			{tofaID, "Token Factory title", "nebius-tofa", want},
		} {
			thread := e.call("thread/read", map[string]any{"threadId": tc.id, "includeTurns": true})["thread"].(map[string]any)
			if thread["name"] != tc.title || thread["cwd"] != workspace || thread["modelProvider"] != tc.provider {
				t.Fatalf("history metadata changed: %v", thread)
			}
			var messages []string
			for _, rawTurn := range thread["turns"].([]any) {
				for _, rawItem := range rawTurn.(map[string]any)["items"].([]any) {
					item := rawItem.(map[string]any)
					switch item["type"] {
					case "userMessage":
						for _, raw := range item["content"].([]any) {
							part := raw.(map[string]any)
							if part["type"] == "text" {
								messages = append(messages, "user:"+part["text"].(string))
							}
						}
					case "agentMessage":
						messages = append(messages, "assistant:"+item["text"].(string))
					case "commandExecution":
						data, _ := json.Marshal(item)
						if !strings.Contains(string(data), "printf history-tool-result") || !strings.Contains(fmt.Sprint(item["aggregatedOutput"]), "history-tool-result") || item["exitCode"] != float64(0) {
							t.Fatalf("tool call/result lost: %s", data)
						}
						messages = append(messages, "tool:printf history-tool-result:history-tool-result")
					}
				}
			}
			if !reflect.DeepEqual(messages, tc.messages) {
				data, _ := json.Marshal(thread["turns"])
				t.Fatalf("ordered history changed: got %v want %v; turns: %s", messages, tc.messages, data)
			}
		}
	}
	for launch := 0; launch < 2; launch++ {
		e = openDesktopEngine(t, ordinary)
		check(e)
		resumed = e.call("thread/resume", map[string]any{"threadId": tofaID, "model": nil, "modelProvider": nil})
		if resumed["modelProvider"] != "nebius-tofa" || resumed["model"] != "moonshotai/Kimi-K3" {
			t.Fatal("ordinary hydration changed provider/model")
		}
		before := tofaCalls.Load()
		e.turnText(tofaID, "failed", "Inactive send")
		if tofaCalls.Load() != before || nativeCalls.Load() != 2 {
			t.Fatal("inactive conversation reached inference")
		}
		want = append(want, "user:Inactive send")
		e.close()
		if err := os.Remove(capture); err != nil {
			t.Fatal(err)
		}
		child, stop = liveDesktopFixture(t, app, bundle, capture)
		e = openDesktopEngine(t, child)
		check(e)
		e.call("thread/resume", map[string]any{"threadId": tofaID, "model": nil, "modelProvider": nil})
		e.turnText(tofaID, "completed", "Continue same thread after relaunch")
		want = append(want, "user:Continue same thread after relaunch", "assistant:Token Factory history answer")
		check(e)
		e.close()
		stop()
	}
	e = openDesktopEngine(t, ordinary)
	check(e)
	e.close()
	if nativeCalls.Load() != 2 || tofaCalls.Load() != 4 {
		t.Fatalf("unexpected routing counts: native=%d tofa=%d", nativeCalls.Load(), tofaCalls.Load())
	}
}
