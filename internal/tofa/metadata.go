package tofa

import (
	_ "embed"
	"encoding/json"
	"errors"
	"os"
)

//go:embed assets/codex-prompt.md
var codexPrompt string

//go:embed assets/evaluation-candidates.json
var candidateSnapshot []byte

func prepareModelCatalog(model string) (string, error) {
	var snapshot struct {
		Models map[string]struct {
			DisplayName string   `json:"display_name"`
			Context     int      `json:"context_window"`
			Modalities  []string `json:"input_modalities"`
			Responses   bool     `json:"responses_api"`
			Tools       bool     `json:"function_calling"`
		} `json:"models"`
	}
	if err := json.Unmarshal(candidateSnapshot, &snapshot); err != nil {
		return "", errors.New("invalid bundled model metadata")
	}
	candidate, known := snapshot.Models[model]
	if !known {
		return "", nil
	}
	if candidate.Context <= 4096 || len(candidate.Modalities) == 0 || !candidate.Responses || !candidate.Tools {
		return "", errors.New("unresolved candidate model metadata")
	}
	catalog := map[string]any{"models": []any{map[string]any{
		"slug":                                 model,
		"display_name":                         candidate.DisplayName + " (Token Factory)",
		"description":                          "Provider catalog snapshot 2026-09-23; experimental Codex integration",
		"supported_reasoning_levels":           []any{},
		"shell_type":                           "unified_exec",
		"visibility":                           "list",
		"supported_in_api":                     true,
		"priority":                             0,
		"include_apps_usage_instructions":      false,
		"supports_reasoning_summary_parameter": false,
		"support_verbosity":                    false,
		"truncation_policy":                    map[string]any{"mode": "bytes", "limit": 10000},
		"context_window":                       candidate.Context,
		"max_context_window":                   candidate.Context,
		"effective_context_window_percent":     95,
		"experimental_supported_tools":         []any{},
		"input_modalities":                     candidate.Modalities,
		"model_messages":                       map[string]any{"instructions_template": codexPrompt},
	}}}
	data, err := json.Marshal(catalog)
	if err != nil {
		return "", errors.New("could not encode launch model catalog")
	}
	file, err := os.CreateTemp("", "tofa-model-catalog-*.json")
	if err != nil {
		return "", errors.New("could not create launch model catalog")
	}
	_, writeErr := file.Write(data)
	closeErr := file.Close()
	if writeErr != nil || closeErr != nil {
		if err := os.Remove(file.Name()); err != nil {
			return "", errors.New("could not write or clean up launch model catalog at " + file.Name())
		}
		return "", errors.New("could not write launch model catalog")
	}
	return file.Name(), nil
}
