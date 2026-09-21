package tofa

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"strconv"
	"strings"
	"time"
)

func childArgs(model, project, endpoint string, extra []string) ([]string, error) {
	// These flags can replace the selected model, provider or authentication.
	for _, arg := range extra {
		for _, prefix := range []string{"--config", "--profile", "--model", "--oss", "--local-provider", "--remote", "-c", "-p", "-m"} {
			if strings.HasPrefix(arg, prefix) {
				return nil, fmt.Errorf("Codex routing flag %s is not allowed through tofa", prefix)
			}
		}
	}
	query := ""
	if project != "" {
		query = `, query_params = { ai_project_id = ` + strconv.Quote(project) + ` }`
	}
	provider := `{ name = "Nebius Token Factory", base_url = ` + strconv.Quote(endpoint) + `, env_key = "TOFA_API_KEY", wire_api = "responses", requires_openai_auth = false, supports_websockets = false, request_max_retries = 0, stream_max_retries = 0` + query + ` }`
	args := []string{"-c", "model=" + strconv.Quote(model), "-c", `model_provider="nebius-tofa"`, "-c", "model_providers.nebius-tofa=" + provider, "-c", `web_search="disabled"`}
	return append(args, extra...), nil
}
func childEnv(key string) []string {
	env := []string{}
	for _, item := range os.Environ() {
		name, _, _ := strings.Cut(item, "=")
		switch strings.ToUpper(name) {
		case "TOFA_API_KEY", "OPENAI_API_KEY", "OPENAI_BASE_URL", "OPENAI_ORGANIZATION", "OPENAI_PROJECT":
			continue
		}
		env = append(env, item)
	}
	return append(env, "TOFA_API_KEY="+key)
}
func runClient(ctx context.Context, args, env []string) error {
	path, err := exec.LookPath("codex")
	if err != nil {
		return errors.New("Codex CLI is not installed or not on PATH; install it before launching")
	}
	cmd := exec.CommandContext(ctx, path, args...)
	cmd.Env = env
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Cancel = func() error {
		if runtime.GOOS == "windows" {
			return cmd.Process.Kill()
		}
		return cmd.Process.Signal(os.Interrupt)
	}
	cmd.WaitDelay = 2 * time.Second
	if err = cmd.Start(); err != nil {
		return errors.New("could not start Codex CLI")
	}
	return cmd.Wait()
}
