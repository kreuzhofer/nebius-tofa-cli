package tofa

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
)

func childArgs(model, project string, extra []string) ([]string, error) {
	// These flags can replace the selected model, provider or authentication.
	for _, arg := range extra {
		for _, prefix := range []string{"--config", "--profile", "--model", "--oss", "--local-provider", "--remote", "-c", "-p", "-m"} {
			if strings.HasPrefix(arg, prefix) {
				return nil, fmt.Errorf("Codex routing flag %s is not allowed through tofa", prefix)
			}
		}
	}
	provider := `{ name = "Nebius Token Factory", base_url = "` + Endpoint + `", env_key = "TOFA_API_KEY", wire_api = "responses", requires_openai_auth = false, supports_websockets = false, query_params = { ai_project_id = ` + strconv.Quote(project) + ` } }`
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
func runClient(args, env []string) error {
	path, err := exec.LookPath("codex")
	if err != nil {
		return errors.New("Codex CLI is not installed or not on PATH; install it before launching")
	}
	cmd := exec.Command(path, args...)
	cmd.Env = env
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err = cmd.Start(); err != nil {
		return errors.New("could not start Codex CLI")
	}
	signals := make(chan os.Signal, 1)
	done := make(chan struct{})
	signal.Notify(signals, os.Interrupt, syscall.SIGTERM)
	defer signal.Stop(signals)
	go func() {
		for {
			select {
			case sig := <-signals:
				_ = cmd.Process.Signal(sig)
			case <-done:
				return
			}
		}
	}()
	err = cmd.Wait()
	close(done)
	return err
}
