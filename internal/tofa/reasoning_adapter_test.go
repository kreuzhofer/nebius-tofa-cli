package tofa_test

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"
)

func TestInstalledCodexKeepsProviderReasoningSeparate(t *testing.T) {
	client := os.Getenv("TOFA_TEST_CODEX")
	if client == "" {
		t.Skip("set TOFA_TEST_CODEX")
	}
	requests := make(chan json.RawMessage, 4)
	app, _ := adapterFixture(t, func(w http.ResponseWriter, r *http.Request) {
		var payload map[string]json.RawMessage
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Error(err)
			return
		}
		select {
		case requests <- payload["reasoning"]:
		default:
			t.Error("unexpected extra provider request")
			http.Error(w, "request budget exceeded", http.StatusTooManyRequests)
			return
		}
		emitReasoningFixture(w)
	}, nil)
	root := t.TempDir()
	home := filepath.Join(root, "codex")
	if err := os.Mkdir(home, 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(home, "config.toml"), []byte("model_reasoning_effort=\"none\"\n"), 0600); err != nil {
		t.Fatal(err)
	}
	var stdout []byte
	app.RunClient = func(args, env []string) error {
		isolated := []string{"HOME=" + root, "CODEX_HOME=" + home, "OTEL_SDK_DISABLED=true"}
		for _, entry := range env {
			name, _, _ := strings.Cut(entry, "=")
			switch name {
			case "PATH", "TMPDIR", "SYSTEMROOT", "WINDIR", "TOFA_API_KEY":
				isolated = append(isolated, entry)
			}
		}
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		cmd := exec.CommandContext(ctx, client, args...)
		cmd.Dir, cmd.Env = root, isolated
		var err error
		stdout, err = cmd.Output()
		return err
	}
	if err := app.Run([]string{"launch", "codex", "--model", "zai-org/GLM-5.3", "--allow-unverified", "--", "exec", "--sandbox", "read-only", "--skip-git-repo-check", "--json", "Synthetic reasoning channel check"}); err != nil {
		t.Fatal(err)
	}
	var answer, summary string
	var reasoningTokens int
	for _, line := range strings.Split(string(stdout), "\n") {
		var event struct {
			Type  string `json:"type"`
			Usage struct {
				ReasoningTokens int `json:"reasoning_output_tokens"`
			} `json:"usage"`
			Item struct {
				Type string `json:"type"`
				Text string `json:"text"`
			} `json:"item"`
		}
		if json.Unmarshal([]byte(line), &event) != nil {
			continue
		}
		if event.Type == "turn.completed" {
			reasoningTokens = event.Usage.ReasoningTokens
		}
		if event.Type == "item.completed" && event.Item.Type == "agent_message" {
			answer = event.Item.Text
		}
		if event.Type == "item.completed" && event.Item.Type == "reasoning" {
			summary = event.Item.Text
		}
	}
	// exec --json omits raw reasoning from displayed items, but keeps its usage
	// and structured history. TUI raw-reasoning visibility is a native setting.
	if reasoningTokens != 4 || summary != "Synthetic summary." || answer != "Literal </think> example." {
		t.Fatalf("CLI channel separation: reasoning tokens=%d summary=%q answer=%q; events=%s", reasoningTokens, summary, answer, stdout)
	}
	retainedReasoning := false
	err := filepath.WalkDir(filepath.Join(home, "sessions"), func(path string, entry fs.DirEntry, err error) error {
		if err != nil || entry.IsDir() || !strings.HasSuffix(path, ".jsonl") {
			return err
		}
		raw, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		for _, line := range strings.Split(string(raw), "\n") {
			var record struct {
				Type    string `json:"type"`
				Payload struct {
					Type    string `json:"type"`
					Content []struct {
						Type string `json:"type"`
						Text string `json:"text"`
					} `json:"content"`
				} `json:"payload"`
			}
			if json.Unmarshal([]byte(line), &record) == nil && record.Type == "response_item" && record.Payload.Type == "reasoning" {
				for _, part := range record.Payload.Content {
					retainedReasoning = retainedReasoning || (part.Type == "reasoning_text" && part.Text == "Synthetic reasoning.")
				}
			}
		}
		return nil
	})
	if err != nil || !retainedReasoning {
		t.Fatalf("CLI did not retain separate reasoning history: %v", err)
	}
	var sent map[string]any
	if raw := <-requests; len(raw) != 0 {
		if err := json.Unmarshal(raw, &sent); err != nil {
			t.Fatal(err)
		}
	}
	if _, present := sent["effort"]; present {
		t.Errorf("unsupported CLI None reached provider: %v", sent)
	}
}

func TestAdapterUsesGLMProviderReasoningDefaultWithoutChangingText(t *testing.T) {
	for _, tc := range []struct {
		name, model, reasoning, want string
		changed                      bool
	}{
		{"native None", "zai-org/GLM-5.3", `{"effort":"none"}`, ``, true},
		{"preserve other reasoning fields", "zai-org/GLM-5.3", `{"effort":"none","summary":"auto","future":9007199254740993}`, `{"summary":"auto","future":9007199254740993}`, true},
		{"explicit high", "zai-org/GLM-5.3", `{"effort":"high"}`, `{"effort":"high"}`, false},
		{"provider default", "zai-org/GLM-5.3", ``, ``, false},
		{"explicit null", "zai-org/GLM-5.3", `null`, `null`, false},
		{"other model", "zai-org/GLM-5.3-Flash", `{"effort":"none"}`, `{"effort":"none"}`, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			body := `{"model":"` + tc.model + `","input":[{"role":"user","content":"Explain literal </think> text"}],"future":9007199254740993}`
			want := body
			if tc.reasoning != "" {
				body = strings.TrimSuffix(body, `}`) + `,"reasoning":` + tc.reasoning + `}`
			}
			if tc.want != "" {
				want = strings.TrimSuffix(want, `}`) + `,"reasoning":` + tc.want + `}`
			}
			const responseBody = "event: response.output_text.delta\ndata: {\"type\":\"response.output_text.delta\",\"delta\":\"Literal </think> text\"}\n\n"
			app, output := adapterFixture(t, func(w http.ResponseWriter, r *http.Request) {
				raw, err := io.ReadAll(r.Body)
				if err != nil {
					t.Error(err)
					return
				}
				if !reflect.DeepEqual(jsonValue(t, raw), jsonValue(t, []byte(want))) {
					t.Errorf("request contract changed: %s", raw)
				}
				w.Header().Set("Content-Type", "text/event-stream")
				io.WriteString(w, responseBody)
			}, func(endpoint, token string) error {
				for i := 0; i < 2; i++ {
					response := adapterRequest(t, endpoint, token, body)
					raw, err := io.ReadAll(response.Body)
					response.Body.Close()
					if err != nil || string(raw) != responseBody {
						t.Error("native response stream was changed")
					}
				}
				return nil
			})
			runAdapted(t, app)
			notices := strings.Count(output.String(), "GLM 5.3 thinking:")
			if (tc.changed && notices != 1) || (!tc.changed && notices != 0) {
				t.Errorf("unexpected reasoning notices: %d", notices)
			}
		})
	}
}

func emitReasoningFixture(w http.ResponseWriter) {
	w.Header().Set("Content-Type", "text/event-stream")
	emit := func(kind string, body map[string]any) {
		body["type"] = kind
		raw, _ := json.Marshal(body)
		fmt.Fprintf(w, "event: %s\ndata: %s\n\n", kind, raw)
		w.(http.Flusher).Flush()
	}
	reasoning := map[string]any{"id": "rs_fixture", "type": "reasoning", "summary": []any{}, "content": []any{}}
	answer := map[string]any{"id": "msg_fixture", "type": "message", "role": "assistant", "status": "in_progress", "content": []any{}}
	emit("response.created", map[string]any{"response": map[string]any{"id": "resp_fixture", "status": "in_progress", "output": []any{}}})
	emit("response.output_item.added", map[string]any{"output_index": 0, "item": reasoning})
	emit("response.reasoning_text.delta", map[string]any{"output_index": 0, "item_id": "rs_fixture", "content_index": 0, "delta": "Synthetic reasoning."})
	emit("response.reasoning_summary_part.added", map[string]any{"output_index": 0, "item_id": "rs_fixture", "summary_index": 0, "part": map[string]string{"type": "summary_text", "text": ""}})
	emit("response.reasoning_summary_text.delta", map[string]any{"output_index": 0, "item_id": "rs_fixture", "summary_index": 0, "delta": "Synthetic summary."})
	reasoning["content"] = []any{map[string]string{"type": "reasoning_text", "text": "Synthetic reasoning."}}
	reasoning["summary"] = []any{map[string]string{"type": "summary_text", "text": "Synthetic summary."}}
	emit("response.output_item.done", map[string]any{"output_index": 0, "item": reasoning})
	emit("response.output_item.added", map[string]any{"output_index": 1, "item": answer})
	emit("response.content_part.added", map[string]any{"output_index": 1, "content_index": 0, "item_id": "msg_fixture", "part": map[string]any{"type": "output_text", "text": "", "annotations": []any{}}})
	emit("response.output_text.delta", map[string]any{"output_index": 1, "content_index": 0, "item_id": "msg_fixture", "delta": "Literal </think> example."})
	answer["status"] = "completed"
	answer["content"] = []any{map[string]any{"type": "output_text", "text": "Literal </think> example.", "annotations": []any{}}}
	emit("response.output_item.done", map[string]any{"output_index": 1, "item": answer})
	emit("response.completed", map[string]any{"response": map[string]any{"id": "resp_fixture", "status": "completed", "model": "zai-org/GLM-5.3", "output": []any{reasoning, answer}, "usage": map[string]any{"input_tokens": 10, "output_tokens": 8, "total_tokens": 18, "output_tokens_details": map[string]int{"reasoning_tokens": 4}}}})
}
