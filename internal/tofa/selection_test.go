package tofa_test

import (
	"encoding/json"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"strconv"
	"strings"
	"testing"
)

func TestExplicitLaunchDefaultsToGLMGuardian(t *testing.T) {
	app, out := adapterFixture(t, nil, nil)
	app.RunClient = func(args, env []string) error {
		var path string
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
				Slug     string `json:"slug"`
				Guardian string `json:"auto_review_model_override"`
			} `json:"models"`
		}
		if err := json.Unmarshal(data, &catalog); err != nil {
			t.Fatal(err)
		}
		if len(catalog.Models) != 2 || catalog.Models[0].Slug != "moonshotai/Kimi-K3" || catalog.Models[0].Guardian != "zai-org/GLM-5.3-Flash" || catalog.Models[1].Slug != "zai-org/GLM-5.3-Flash" {
			t.Fatalf("wrong effective pair: %s", data)
		}
		return nil
	}
	app.Prompt = func(string, bool) (string, error) { t.Fatal("explicit launch must not prompt"); return "", nil }
	if err := app.Run([]string{"launch", "codex", "--model", "moonshotai/Kimi-K3", "--allow-unverified"}); err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"Main: moonshotai/Kimi-K3", "Guardian: zai-org/GLM-5.3-Flash", "Route: adapted", "Status: experimental"} {
		if !strings.Contains(out.String(), want) {
			t.Errorf("missing %q in %s", want, out)
		}
	}
}

func TestExplicitSelectionRejectsInvalidModelsBeforeLaunch(t *testing.T) {
	for _, test := range []struct {
		name, main, catalog, want string
		flags                     []string
	}{
		{name: "unknown main metadata", main: "fixture-model", want: "main model fixture-model: missing bundled model metadata"},
		{name: "unknown direct metadata", main: "fixture-model", flags: []string{"--direct"}, want: "missing bundled model metadata"},
		{name: "unavailable main", main: "absent/model", want: "main model absent/model: not available"},
		{name: "missing default Guardian", main: "moonshotai/Kimi-K3", catalog: `{"data":[{"id":"moonshotai/Kimi-K3"}]}`, want: "Guardian model zai-org/GLM-5.3-Flash: not available"},
		{name: "unknown Guardian metadata", main: "moonshotai/Kimi-K3", flags: []string{"--guardian-model", "fixture-model"}, want: "Guardian model fixture-model: missing bundled model metadata"},
		{name: "unavailable Guardian", main: "moonshotai/Kimi-K3", flags: []string{"--guardian-model", "absent/model"}, want: "Guardian model absent/model: not available"},
		{name: "empty Guardian", main: "moonshotai/Kimi-K3", flags: []string{"--guardian-model", ""}, want: "valid model ID"},
		{name: "direct override", main: "moonshotai/Kimi-K3", flags: []string{"--direct", "--guardian-model", "moonshotai/Kimi-K3"}, want: "adapted connection"},
		{name: "ambiguous aliases", main: "moonshotai/Kimi-K3", flags: []string{"--guardian-model", "moonshotai/Kimi-K3", "--evaluation-guardian-model", "zai-org/GLM-5.3-Flash"}, want: "use only one"},
	} {
		t.Run(test.name, func(t *testing.T) {
			app, _ := adapterFixture(t, nil, nil)
			if test.catalog != "" {
				server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { io.WriteString(w, test.catalog) }))
				defer server.Close()
				app.Endpoint = server.URL
			}
			app.RunClient = func([]string, []string) error { t.Fatal("invalid selection launched"); return nil }
			args := append([]string{"launch", "codex", "--model", test.main, "--allow-unverified"}, test.flags...)
			if err := app.Run(args); err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("want %q, got %v", test.want, err)
			}
		})
	}
}

func TestDirectLaunchPreservesNativeReviewerAndApprovalPolicy(t *testing.T) {
	app, out := adapterFixture(t, nil, nil)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		io.WriteString(w, `{"data":[{"id":"moonshotai/Kimi-K3"}]}`)
	}))
	defer server.Close()
	app.Endpoint = server.URL
	app.Listen = func(string, string) (net.Listener, error) { t.Fatal("direct launch opened adapter"); return nil, nil }
	launched := false
	app.RunClient = func(args, env []string) error {
		launched = true
		if !strings.Contains(strings.Join(args, " "), "--ask-for-approval on-request") {
			t.Fatal("approval policy was lost")
		}
		for _, arg := range args {
			if value, ok := strings.CutPrefix(arg, "model_catalog_json="); ok {
				path, _ := strconv.Unquote(value)
				data, err := os.ReadFile(path)
				if err != nil {
					t.Fatal(err)
				}
				if strings.Contains(string(data), "auto_review_model_override") || strings.Contains(string(data), "GLM-5.3-Flash") {
					t.Fatal("direct route changed native reviewer selection")
				}
			}
		}
		return nil
	}
	if err := app.Run([]string{"launch", "codex", "--model", "moonshotai/Kimi-K3", "--direct", "--allow-unverified", "--", "--ask-for-approval", "on-request"}); err != nil {
		t.Fatal(err)
	}
	if !launched || !strings.Contains(out.String(), "Guardian: native reviewer selection") || !strings.Contains(out.String(), "Route: direct") || !strings.Contains(out.String(), "Status: experimental") {
		t.Fatalf("missing direct selection: %s", out)
	}
}

func TestHistoricalPairEvidenceDoesNotPromoteSupport(t *testing.T) {
	app, _ := adapterFixture(t, nil, nil)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		io.WriteString(w, `{"data":[{"id":"deepseek-ai/DeepSeek-V4.1-Flash"},{"id":"zai-org/GLM-5.3-Flash"}]}`)
	}))
	defer server.Close()
	app.Endpoint = server.URL
	app.RunClient = func([]string, []string) error { t.Fatal("historical evidence promoted support"); return nil }
	if err := app.Run([]string{"launch", "codex", "--model", "deepseek-ai/DeepSeek-V4.1-Flash"}); err == nil || !strings.Contains(err.Error(), "--allow-unverified") {
		t.Fatalf("expected experimental opt-in: %v", err)
	}
}

func TestHelpExplainsGuardianSelectionContract(t *testing.T) {
	app, out := adapterFixture(t, nil, nil)
	if err := app.Run([]string{"--help"}); err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"--guardian-model ID", "zai-org/GLM-5.3-Flash", "native reviewer", "metadata", "--allow-unverified"} {
		if !strings.Contains(out.String(), want) {
			t.Errorf("help missing %q", want)
		}
	}
}
