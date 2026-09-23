package tofa_test

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

func TestKimiLaunchUsesScopedProviderMetadata(t *testing.T) {
	for _, clientError := range []error{nil, errors.New("fixture client failure")} {
		t.Run(fmtError(clientError), func(t *testing.T) {
			app, _ := adapterFixture(t, nil, func(string, string) error { return clientError })
			runner := app.RunClient
			var catalogPath string
			app.RunClient = func(args, env []string) error {
				for _, arg := range args {
					if value, found := strings.CutPrefix(arg, "model_catalog_json="); found {
						var err error
						catalogPath, err = strconv.Unquote(value)
						if err != nil {
							t.Fatal(err)
						}
					}
				}
				if catalogPath == "" {
					t.Fatal("Kimi launch has no model catalog")
				}
				data, err := os.ReadFile(catalogPath)
				if err != nil {
					t.Fatal(err)
				}
				var catalog struct {
					Models []struct {
						Slug      string `json:"slug"`
						Context   int    `json:"context_window"`
						Maximum   int    `json:"max_context_window"`
						Reasoning []any  `json:"supported_reasoning_levels"`
						Summary   bool   `json:"supports_reasoning_summary_parameter"`
						Verbosity bool   `json:"support_verbosity"`
						Shell     string `json:"shell_type"`
						Messages  struct {
							Instructions string `json:"instructions_template"`
						} `json:"model_messages"`
					} `json:"models"`
				}
				if err := json.Unmarshal(data, &catalog); err != nil {
					t.Fatal(err)
				}
				if len(catalog.Models) != 1 {
					t.Fatal("catalog must contain selected model only")
				}
				model := catalog.Models[0]
				if model.Slug != "moonshotai/Kimi-K3" || model.Context != 1024000 || model.Maximum != 1024000 {
					t.Fatal("incorrect provider-advertised model/context")
				}
				if len(model.Reasoning) != 0 || model.Summary || model.Verbosity {
					t.Fatal("unverified optional controls advertised")
				}
				if model.Shell != "unified_exec" || !strings.Contains(model.Messages.Instructions, "You are a coding agent running in the Codex CLI") {
					t.Fatal("default coding behavior lost")
				}
				return runner(args, env)
			}
			err := app.Run([]string{"launch", "codex", "--model", "moonshotai/Kimi-K3", "--allow-unverified"})
			if !errors.Is(err, clientError) {
				t.Fatalf("lost client result: %v", err)
			}
			if _, err := os.Stat(catalogPath); !os.IsNotExist(err) {
				t.Fatal("launch model catalog was not removed")
			}
		})
	}
}

func TestUnknownModelDoesNotReceiveInventedMetadata(t *testing.T) {
	app, _ := adapterFixture(t, nil, func(string, string) error { return nil })
	runner := app.RunClient
	app.RunClient = func(args, env []string) error {
		for _, arg := range args {
			if strings.HasPrefix(arg, "model_catalog_json=") {
				t.Fatal("unknown model received a fabricated catalog")
			}
		}
		return runner(args, env)
	}
	runAdapted(t, app)
}

func TestEvaluationPairUsesScopedMetadataAndCleansUp(t *testing.T) {
	for _, guardian := range []string{"moonshotai/Kimi-K3", "nvidia/Nemotron-3-Ultra-550b-a55b"} {
		t.Run(guardian, func(t *testing.T) {
			app, _ := adapterFixture(t, nil, nil)
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				io.WriteString(w, `{"data":[{"id":"moonshotai/Kimi-K3"},{"id":"nvidia/Nemotron-3-Ultra-550b-a55b"}]}`)
			}))
			defer server.Close()
			app.Endpoint = server.URL
			var path string
			clientError := errors.New("fixture exit")
			app.RunClient = func(args, env []string) error {
				for _, arg := range args {
					if value, ok := strings.CutPrefix(arg, "model_catalog_json="); ok {
						path, _ = strconv.Unquote(value)
					}
				}
				data, err := os.ReadFile(path)
				if err != nil {
					t.Fatal(err)
				}
				var catalog struct {
					Models []struct {
						Slug      string `json:"slug"`
						Guardian  string `json:"auto_review_model_override"`
						Context   int    `json:"context_window"`
						Reasoning []any  `json:"supported_reasoning_levels"`
					} `json:"models"`
				}
				if err := json.Unmarshal(data, &catalog); err != nil {
					t.Fatal(err)
				}
				byID := map[string]int{}
				for _, model := range catalog.Models {
					byID[model.Slug] = model.Context
					if len(model.Reasoning) != 0 {
						t.Fatal("unverified reasoning controls")
					}
					if model.Slug == "moonshotai/Kimi-K3" && model.Guardian != guardian {
						t.Fatal("missing explicit Guardian selection")
					}
				}
				if byID["moonshotai/Kimi-K3"] != 1024000 {
					t.Fatal("wrong main metadata")
				}
				if guardian != "moonshotai/Kimi-K3" && (len(byID) != 2 || byID[guardian] != 1048576) {
					t.Fatal("reviewer must use its own metadata")
				}
				if len(byID) != len(catalog.Models) {
					t.Fatal("duplicate catalog entries")
				}
				return clientError
			}
			if err := app.Run([]string{"launch", "codex", "--model", "moonshotai/Kimi-K3", "--allow-unverified", "--evaluation-guardian-model", guardian}); !errors.Is(err, clientError) {
				t.Fatalf("lost client result: %v", err)
			}
			if _, err := os.Stat(path); !os.IsNotExist(err) {
				t.Fatal("temporary pair metadata retained")
			}
		})
	}
}

func TestKimiCatalogCreationFailurePreventsLaunch(t *testing.T) {
	app, _ := adapterFixture(t, nil, func(string, string) error {
		t.Error("client started without its model catalog")
		return nil
	})
	missing := filepath.Join(t.TempDir(), "missing")
	for _, name := range []string{"TMPDIR", "TMP", "TEMP"} {
		t.Setenv(name, missing)
	}
	err := app.Run([]string{"launch", "codex", "--model", "moonshotai/Kimi-K3", "--allow-unverified"})
	if err == nil || !strings.Contains(err.Error(), "could not create launch model catalog") {
		t.Fatalf("missing explicit catalog failure: %v", err)
	}
}

func TestInvalidEvaluationRolesPreventClientLaunch(t *testing.T) {
	for _, test := range []struct {
		main, guardian string
		direct         bool
		want           string
	}{
		{"moonshotai/Kimi-K3", "", false, "valid model ID"},
		{"moonshotai/Kimi-K3", "unlisted/model", false, "bundled model metadata"},
		{"unlisted/model", "moonshotai/Kimi-K3", false, "bundled model metadata"},
		{"moonshotai/Kimi-K3", "zai-org/GLM-5.3", false, "not available"},
		{"moonshotai/Kimi-K3", "moonshotai/Kimi-K3", true, "adapted connection"},
	} {
		t.Run(test.main+"/"+test.guardian+"/"+test.want, func(t *testing.T) {
			app, _ := adapterFixture(t, nil, nil)
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				io.WriteString(w, `{"data":[{"id":"moonshotai/Kimi-K3"},{"id":"unlisted/model"}]}`)
			}))
			defer server.Close()
			app.Endpoint = server.URL
			app.RunClient = func(args, env []string) error { t.Fatal("invalid pair launched client"); return nil }
			args := []string{"launch", "codex", "--model", test.main, "--evaluation-guardian-model", test.guardian, "--allow-unverified"}
			if test.direct {
				args = append(args, "--direct")
			}
			if err := app.Run(args); err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("missing explicit role failure: %v", err)
			}
		})
	}
}

func TestCandidateLaunchUsesOwnScopedMetadata(t *testing.T) {
	for _, candidate := range []struct {
		id      string
		context int
		images  bool
	}{
		{"zai-org/GLM-5.3-Flash", 1024000, true},
		{"deepseek-ai/DeepSeek-V4.1-Flash", 1048000, true},
		{"zai-org/GLM-5.3", 1024000, false},
		{"nvidia/Nemotron-3-Ultra-550b-a55b", 1048576, false},
	} {
		t.Run(candidate.id, func(t *testing.T) {
			app, _ := adapterFixture(t, nil, func(string, string) error { return nil })
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				io.WriteString(w, `{"data":[{"id":"`+candidate.id+`"}]}`)
			}))
			defer server.Close()
			app.Endpoint = server.URL
			var path string
			app.RunClient = func(args, env []string) error {
				for _, arg := range args {
					if value, ok := strings.CutPrefix(arg, "model_catalog_json="); ok {
						path, _ = strconv.Unquote(value)
					}
				}
				if path == "" {
					t.Fatal("missing candidate metadata")
				}
				data, err := os.ReadFile(path)
				if err != nil {
					t.Fatal(err)
				}
				var catalog struct {
					Models []struct {
						Slug       string   `json:"slug"`
						Context    int      `json:"context_window"`
						Modalities []string `json:"input_modalities"`
					} `json:"models"`
				}
				if err := json.Unmarshal(data, &catalog); err != nil {
					t.Fatal(err)
				}
				if len(catalog.Models) != 1 || catalog.Models[0].Slug != candidate.id || catalog.Models[0].Context != candidate.context {
					t.Fatalf("wrong candidate metadata: %s", data)
				}
				images := false
				for _, modality := range catalog.Models[0].Modalities {
					images = images || modality == "image"
				}
				if images != candidate.images {
					t.Fatal("wrong candidate modalities")
				}
				return nil
			}
			if err := app.Run([]string{"launch", "codex", "--model", candidate.id, "--allow-unverified"}); err != nil {
				t.Fatal(err)
			}
			if _, err := os.Stat(path); !os.IsNotExist(err) {
				t.Fatal("candidate catalog retained")
			}
		})
	}
}
