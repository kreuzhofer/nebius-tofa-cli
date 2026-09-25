package tofa_test

import (
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
)

func TestDesktopBundledEngineAutomaticApproval(t *testing.T) {
	installed := os.Getenv("TOFA_TEST_DESKTOP_ENGINE")
	if installed == "" {
		t.Skip("set TOFA_TEST_DESKTOP_ENGINE")
	}
	for _, scenario := range []struct {
		name, assessment string
		allow, inspect   bool
		status           int
	}{
		{"allow", `{"outcome":"allow"}`, true, false, 200},
		{"allow after inspection", `{"outcome":"allow"}`, true, true, 200},
		{"deny", `{"outcome":"deny"}`, false, false, 200},
		{"invalid assessment", `{"outcome":"maybe"}`, false, false, 200},
		{"provider failure", "", false, false, 503},
	} {
		t.Run(scenario.name, func(t *testing.T) {
			bundle, capture := desktopFixture(t, "ignore")
			engine := filepath.Join(bundle, "Contents/Resources/codex")
			if err := os.Remove(engine); err != nil {
				t.Fatal(err)
			}
			if err := os.Symlink(installed, engine); err != nil {
				t.Fatal(err)
			}
			var mainRequests, reviewRequests atomic.Int32
			var inspected atomic.Bool
			app, output := adapterFixture(t, func(w http.ResponseWriter, r *http.Request) {
				var request struct {
					Model        string
					Tools        []json.RawMessage
					Instructions string
					Text         map[string]json.RawMessage
					Input        []struct {
						Type   string
						Output json.RawMessage
					}
				}
				if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
					t.Error(err)
					return
				}
				if request.Model != "moonshotai/Kimi-K3" {
					t.Error("unexpected model reached Token Factory")
				}
				if len(request.Tools) == 3 {
					count := reviewRequests.Add(1)
					if request.Text["format"] != nil || !strings.Contains(request.Instructions, `"outcome"`) {
						t.Error("review assessment guidance was lost or still uses constrained decoding")
					}
					if scenario.status != 200 {
						w.WriteHeader(scenario.status)
						return
					}
					if scenario.inspect && count == 1 {
						emitFixtureResponse(w, map[string]any{"id": "fc_inspect", "type": "function_call", "call_id": "call_inspect", "name": "exec_command", "status": "completed", "arguments": `{"cmd":"printf tofa-review-inspection","max_output_tokens":100}`})
						return
					}
					for _, item := range request.Input {
						if item.Type == "function_call_output" && strings.Contains(string(item.Output), "tofa-review-inspection") && strings.Contains(string(item.Output), "Process exited with code 0") {
							inspected.Store(true)
						}
					}
					emitFixtureResponse(w, fixtureMessage(scenario.assessment))
				} else if mainRequests.Add(1) == 1 {
					emitFixtureResponse(w, map[string]any{
						"id": "fc_desktop_review", "type": "function_call", "call_id": "call_desktop_review",
						"name": "exec_command", "status": "completed",
						"arguments": `{"cmd":"printf tofa-desktop-approved","sandbox_permissions":"require_escalated","justification":"Run the harmless desktop qualification fixture.","max_output_tokens":100}`,
					})
				} else {
					emitFixtureResponse(w, fixtureMessage("Fixture complete."))
				}
			}, nil)
			child, stop := liveDesktopFixture(t, app, bundle, capture)
			e := openDesktopEngine(t, child)
			started := e.call("thread/start", map[string]any{
				"cwd": child.Cwd, "approvalPolicy": "on-request", "approvalsReviewer": "auto_review", "sandbox": "read-only",
			})
			if started["approvalPolicy"] != "on-request" || started["approvalsReviewer"] != "auto_review" {
				t.Fatalf("desktop approval settings changed: %v", started)
			}
			id := started["thread"].(map[string]any)["id"].(string)
			e.call("turn/start", map[string]any{"threadId": id, "input": []any{map[string]string{"type": "text", "text": "Run the harmless fixture once. Stop if review fails."}}})
			executed := false
			for {
				message := e.read()
				if message["method"] == "item/completed" {
					item := message["params"].(map[string]any)["item"].(map[string]any)
					if item["type"] == "commandExecution" && item["exitCode"] == float64(0) && strings.Contains(item["aggregatedOutput"].(string), "tofa-desktop-approved") {
						executed = true
					}
				}
				if message["method"] == "turn/completed" {
					params := message["params"].(map[string]any)
					if params["threadId"] != id || params["turn"].(map[string]any)["status"] != "completed" {
						t.Fatalf("unexpected main turn result: %v", params)
					}
					break
				}
			}
			e.close()
			stop()
			if reviewRequests.Load() == 0 || executed != scenario.allow || inspected.Load() != scenario.inspect {
				t.Fatalf("desktop automatic approval did not reach a reviewer and execute its approved action: reviews=%d executed=%v\n%s", reviewRequests.Load(), executed, output.String())
			}
			if !strings.Contains(output.String(), "Automatic review for moonshotai/Kimi-K3 conversations uses moonshotai/Kimi-K3") {
				t.Fatal("review model selection was not announced")
			}
		})
	}
}
