package tofa_test

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func TestInstalledCodexToolAndContinuationThroughAdapter(t *testing.T) {
	clientPath := os.Getenv("TOFA_TEST_CODEX")
	if clientPath == "" {
		t.Skip("set TOFA_TEST_CODEX to run installed Codex against synthetic local responses")
	}
	if !filepath.IsAbs(clientPath) {
		t.Fatal("TOFA_TEST_CODEX must be absolute")
	}
	version, err := exec.Command(clientPath, "--version").Output()
	if err != nil {
		t.Fatal(err)
	}
	t.Log(strings.TrimSpace(string(version)))
	var requests, histories, toolResults atomic.Int32
	upstream := func(writer http.ResponseWriter, request *http.Request) {
		var payload struct {
			Input []struct {
				Type    string `json:"type"`
				Role    string `json:"role"`
				Status  string `json:"status"`
				Content []struct {
					Type        string          `json:"type"`
					Annotations json.RawMessage `json:"annotations"`
				} `json:"content"`
				Output json.RawMessage `json:"output"`
			} `json:"input"`
			Tools []struct {
				Name string `json:"name"`
			} `json:"tools"`
		}
		if err := json.NewDecoder(request.Body).Decode(&payload); err != nil {
			t.Error(err)
			writer.WriteHeader(400)
			return
		}
		count := requests.Add(1)
		if count > 3 {
			t.Error("unexpected extra client request")
			writer.WriteHeader(500)
			return
		}
		for _, item := range payload.Input {
			if item.Role == "assistant" && item.Type == "message" {
				histories.Add(1)
				if item.Status == "" {
					t.Error("missing assistant status")
					writer.WriteHeader(422)
					return
				}
				for _, part := range item.Content {
					if part.Type == "output_text" && part.Annotations == nil {
						t.Error("missing annotations")
						writer.WriteHeader(422)
						return
					}
				}
			}
			if item.Type == "function_call_output" && strings.Contains(string(item.Output), "tofa-fixture-tool") {
				if !strings.Contains(string(item.Output), "Process exited with code 0") {
					t.Error("synthetic command did not report successful execution")
					writer.WriteHeader(400)
					return
				}
				toolResults.Add(1)
			}
		}
		var item map[string]any
		if count == 1 {
			var name, arguments string
			for _, tool := range payload.Tools {
				switch tool.Name {
				case "exec_command":
					name, arguments = tool.Name, `{"cmd":"printf tofa-fixture-tool","max_output_tokens":100}`
				case "shell_command":
					name, arguments = tool.Name, `{"command":"printf tofa-fixture-tool"}`
				case "shell":
					name, arguments = tool.Name, `{"command":["sh","-c","printf tofa-fixture-tool"]}`
				}
				if name != "" {
					break
				}
			}
			if name == "" {
				t.Error("no known shell tool advertised")
				writer.WriteHeader(400)
				return
			}
			item = map[string]any{"id": "fc_fixture", "type": "function_call", "call_id": "call_fixture", "name": name, "arguments": arguments, "status": "completed"}
		} else {
			item = map[string]any{"id": fmt.Sprintf("msg_%d", count), "type": "message", "role": "assistant", "status": "completed", "content": []any{map[string]any{"type": "output_text", "text": "Fixture complete.", "annotations": []any{}}}}
		}
		response := map[string]any{"id": fmt.Sprintf("resp_%d", count), "object": "response", "status": "completed", "model": "moonshotai/Kimi-K3", "output": []any{item}, "usage": map[string]int{"input_tokens": 10, "output_tokens": 2, "total_tokens": 12}}
		writer.Header().Set("Content-Type", "text/event-stream")
		emit := func(name string, body map[string]any) {
			body["type"] = name
			encoded, _ := json.Marshal(body)
			fmt.Fprintf(writer, "event: %s\ndata: %s\n\n", name, encoded)
			writer.(http.Flusher).Flush()
		}
		emit("response.created", map[string]any{"response": map[string]any{"id": response["id"], "status": "in_progress", "output": []any{}}})
		emit("response.output_item.added", map[string]any{"output_index": 0, "item": item})
		emit("response.output_item.done", map[string]any{"output_index": 0, "item": item})
		emit("response.completed", map[string]any{"response": response})
	}
	app, _ := adapterFixture(t, upstream, nil)
	root := t.TempDir()
	clientHome := filepath.Join(root, "codex")
	if err := os.Mkdir(clientHome, 0700); err != nil {
		t.Fatal(err)
	}
	sentinel := []byte("model_provider = \"original-provider\"\n")
	configPath := filepath.Join(clientHome, "config.toml")
	if err := os.WriteFile(configPath, sentinel, 0600); err != nil {
		t.Fatal(err)
	}
	app.RunClient = func(args, env []string) error {
		isolated := []string{"HOME=" + root, "CODEX_HOME=" + clientHome, "OTEL_SDK_DISABLED=true"}
		for _, entry := range env {
			name, _, _ := strings.Cut(entry, "=")
			switch name {
			case "PATH", "TMPDIR", "SYSTEMROOT", "WINDIR", "TOFA_API_KEY":
				isolated = append(isolated, entry)
			}
		}
		ctx, cancel := context.WithTimeout(context.Background(), 40*time.Second)
		defer cancel()
		command := exec.CommandContext(ctx, clientPath, args...)
		command.Dir, command.Env = root, isolated
		output, err := command.CombinedOutput()
		if err != nil {
			t.Log(string(output))
		}
		return err
	}
	for _, extra := range [][]string{
		{"--ask-for-approval", "never", "exec", "--sandbox", "read-only", "--skip-git-repo-check", "--json", "Run the synthetic fixture tool and reply."},
		{"exec", "resume", "--last", "--skip-git-repo-check", "--json", "Continue the fixture conversation."},
	} {
		args := append([]string{"launch", "codex", "--model", "moonshotai/Kimi-K3", "--allow-unverified", "--"}, extra...)
		if err := app.Run(args); err != nil {
			t.Fatal(err)
		}
	}
	if requests.Load() != 3 || histories.Load() == 0 || toolResults.Load() == 0 {
		t.Fatalf("missing tool/continuation evidence: requests=%d histories=%d toolResults=%d", requests.Load(), histories.Load(), toolResults.Load())
	}
	after, err := os.ReadFile(configPath)
	if err != nil || string(after) != string(sentinel) {
		t.Fatal("Codex config changed")
	}
	if _, err := os.Stat(filepath.Join(clientHome, "auth.json")); !os.IsNotExist(err) {
		t.Fatal("unexpected client auth file")
	}
}
