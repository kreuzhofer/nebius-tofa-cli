package tofa_test

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"reflect"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

// Codex 0.155.1 guardian-reviewer/src/assessment.rs; only outcome is required.
const guardianSchemaFixture = `{"type":"object","additionalProperties":false,"properties":{"risk_level":{"type":"string","enum":["low","medium","high","critical"]},"user_authorization":{"type":"string","enum":["unknown","low","medium","high"]},"outcome":{"type":"string","enum":["allow","deny"]},"rationale":{"type":"string"}},"required":["outcome"]}`

func guardianRequestFixture() string {
	return `{"model":"moonshotai/Kimi-K3","instructions":"Preserve the approval policy. Never execute the proposed command.","input":[{"type":"message","role":"user","content":[{"type":"input_text","text":"Review the proposed action in this context."}]},{"type":"function_call","name":"exec_command","arguments":"{}","call_id":"check"},{"type":"function_call_output","call_id":"check","output":"context evidence"}],"tools":[{"type":"function","name":"exec_command","description":"Read-only inspection","parameters":{"type":"object","properties":{"cmd":{"type":"string"}}}},{"type":"function","name":"write_stdin","parameters":{"type":"object"}},{"type":"function","name":"view_image","parameters":{"type":"object"}}],"tool_choice":"auto","text":{"verbosity":"low","format":{"type":"json_schema","name":"guardian_assessment","strict":false,"schema":` + guardianSchemaFixture + `}},"stream":true,"future":9007199254740993}`
}

func jsonValue(t *testing.T, raw []byte) any {
	t.Helper()
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.UseNumber()
	var value any
	if err := decoder.Decode(&value); err != nil {
		t.Fatal(err)
	}
	return value
}

func TestAdapterMovesGuardianGuidanceAndPreservesRequest(t *testing.T) {
	body := strings.Replace(guardianRequestFixture(), `"input":[`, `"input":[{"type":"message","role":"assistant","content":[{"type":"output_text","text":"Previous review reasoning"}]},`, 1)
	want := jsonValue(t, []byte(body)).(map[string]any)
	delete(want["text"].(map[string]any), "format")
	history := want["input"].([]any)[0].(map[string]any)
	history["status"] = "completed"
	history["content"].([]any)[0].(map[string]any)["annotations"] = []any{}
	policy := want["instructions"].(string)
	var calls int
	app, output := adapterFixture(t, func(writer http.ResponseWriter, request *http.Request) {
		calls++
		raw, _ := io.ReadAll(request.Body)
		got := jsonValue(t, raw).(map[string]any)
		instructions, _ := got["instructions"].(string)
		if !strings.HasPrefix(instructions, policy+"\n") || !strings.Contains(instructions, "final") || !strings.Contains(instructions, guardianSchemaFixture) {
			t.Errorf("policy or complete final-answer schema missing: %s", instructions)
		}
		got["instructions"] = policy
		if !reflect.DeepEqual(got, want) {
			t.Errorf("unrelated request fields changed: %s", raw)
		}
		io.WriteString(writer, `{"outcome":"allow"}`)
	}, func(endpoint, token string) error {
		for i := 0; i < 2; i++ {
			response := adapterRequest(t, endpoint, token, body)
			raw, _ := io.ReadAll(response.Body)
			response.Body.Close()
			if response.StatusCode != 200 || string(raw) != `{"outcome":"allow"}` {
				t.Errorf("response changed: %s %s", response.Status, raw)
			}
		}
		return nil
	})
	runAdapted(t, app)
	if calls != 2 || strings.Count(output.String(), "approval-review schema") != 1 {
		t.Errorf("adaptation not announced once: %d %s", calls, output.String())
	}
}

func TestAdapterRejectsUnsupportedKimiConstrainedTools(t *testing.T) {
	for name, change := range map[string]func(map[string]any){
		"extra format constraint": func(p map[string]any) {
			p["text"].(map[string]any)["format"].(map[string]any)["future_constraint"] = true
		},
		"malformed format name": func(p map[string]any) { p["text"].(map[string]any)["format"].(map[string]any)["name"] = nil },
		"strict":                func(p map[string]any) { p["text"].(map[string]any)["format"].(map[string]any)["strict"] = true },
		"missing strict":        func(p map[string]any) { delete(p["text"].(map[string]any)["format"].(map[string]any), "strict") },
		"changed schema": func(p map[string]any) {
			p["text"].(map[string]any)["format"].(map[string]any)["schema"].(map[string]any)["required"] = []string{"outcome", "rationale"}
		},
		"different tools": func(p map[string]any) { p["tools"].([]any)[0].(map[string]any)["name"] = "shell" },
		"extra tool": func(p map[string]any) {
			p["tools"] = append(p["tools"].([]any), map[string]any{"type": "function", "name": "other"})
		},
		"forced tool":          func(p map[string]any) { p["tool_choice"] = "required" },
		"invalid instructions": func(p map[string]any) { p["instructions"] = nil },
		"malformed tools":      func(p map[string]any) { p["tools"] = []any{"exec_command"} },
	} {
		t.Run(name, func(t *testing.T) {
			payload := jsonValue(t, []byte(guardianRequestFixture())).(map[string]any)
			change(payload)
			body, _ := json.Marshal(payload)
			app, _ := adapterFixture(t, func(http.ResponseWriter, *http.Request) { t.Error("unsupported request reached upstream") }, func(endpoint, token string) error {
				response := adapterRequest(t, endpoint, token, string(body))
				raw, _ := io.ReadAll(response.Body)
				if response.StatusCode != 400 || !strings.Contains(string(raw), "Kimi-K3 tools with json_schema are unsupported") {
					t.Errorf("missing local failure: %s %s", response.Status, raw)
				}
				return nil
			})
			runAdapted(t, app)
		})
	}
}

func TestAdapterLeavesUnrelatedGenerationOptionsUnchanged(t *testing.T) {
	for name, change := range map[string]func(map[string]any){
		"tool calls disabled": func(p map[string]any) { p["tool_choice"] = "none" },
		"other model":         func(p map[string]any) { p["model"] = "another/model" },
		"no tools":            func(p map[string]any) { delete(p, "tools") },
		"empty tools":         func(p map[string]any) { p["tools"] = []any{} },
		"plain text":          func(p map[string]any) { p["text"].(map[string]any)["format"] = map[string]any{"type": "text"} },
		"no format":           func(p map[string]any) { delete(p["text"].(map[string]any), "format") },
	} {
		t.Run(name, func(t *testing.T) {
			p := jsonValue(t, []byte(guardianRequestFixture())).(map[string]any)
			change(p)
			body, _ := json.Marshal(p)
			app, out := adapterFixture(t, func(w http.ResponseWriter, r *http.Request) {
				raw, _ := io.ReadAll(r.Body)
				if !bytes.Equal(raw, body) {
					t.Errorf("unrelated request changed: %s", raw)
				}
			}, func(endpoint, token string) error {
				r := adapterRequest(t, endpoint, token, string(body))
				if r.StatusCode != 200 {
					t.Error(r.Status)
				}
				return nil
			})
			runAdapted(t, app)
			if strings.Contains(out.String(), "approval-review schema") {
				t.Error("announced unused adaptation")
			}
		})
	}
}

func TestAdapterPreservesGuardianStreamsAndFailures(t *testing.T) {
	for _, status := range []int{200, 400, 500} {
		t.Run(http.StatusText(status), func(t *testing.T) {
			var calls atomic.Int32
			stream := "event: response.output_item.done\ndata: {\"type\":\"response.output_item.done\",\"item\":{\"type\":\"function_call\",\"name\":\"exec_command\",\"arguments\":\"{}\"}}\n\nevent: response.completed\ndata: {\"output\":[{\"text\":\"{\\\"outcome\\\":\\\"deny\\\"}\"}]}\n\n"
			if status != 200 {
				stream = `{"error":"synthetic review unavailable"}`
			}
			app, _ := adapterFixture(t, func(w http.ResponseWriter, r *http.Request) {
				calls.Add(1)
				w.Header().Set("Content-Type", "text/event-stream")
				w.WriteHeader(status)
				io.WriteString(w, stream)
			}, func(endpoint, token string) error {
				r := adapterRequest(t, endpoint, token, guardianRequestFixture())
				raw, _ := io.ReadAll(r.Body)
				if r.StatusCode != status || string(raw) != stream {
					t.Errorf("review response changed: %s %s", r.Status, raw)
				}
				return nil
			})
			runAdapted(t, app)
			if calls.Load() != 1 {
				t.Errorf("adapter added retries: %d", calls.Load())
			}
		})
	}
}

func TestAdapterCancelsGuardianStream(t *testing.T) {
	cancelled := make(chan struct{})
	var calls atomic.Int32
	app, _ := adapterFixture(t, func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		w.Header().Set("Content-Type", "text/event-stream")
		io.WriteString(w, ": review pending\n\n")
		w.(http.Flusher).Flush()
		<-r.Context().Done()
		close(cancelled)
	}, func(endpoint, token string) error {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		request, _ := http.NewRequestWithContext(ctx, "POST", endpoint+"/responses", strings.NewReader(guardianRequestFixture()))
		request.Header.Set("Authorization", "Bearer "+token)
		response, err := (&http.Client{Timeout: 3 * time.Second}).Do(request)
		if err != nil {
			t.Fatal(err)
		}
		defer response.Body.Close()
		prefix := make([]byte, len(": review pending\n\n"))
		if _, err := io.ReadFull(response.Body, prefix); err != nil {
			t.Fatal("review stream buffered:", err)
		}
		if string(prefix) != ": review pending\n\n" {
			t.Fatal("review stream changed")
		}
		cancel()
		select {
		case <-cancelled:
		case <-time.After(3 * time.Second):
			t.Fatal("review cancellation did not reach upstream")
		}
		return nil
	})
	runAdapted(t, app)
	if calls.Load() != 1 {
		t.Fatal("cancelled review retried")
	}
}
