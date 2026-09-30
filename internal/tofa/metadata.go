package tofa

import (
	_ "embed"
	"encoding/json"
	"errors"
	"os"
	"slices"
)

//go:embed assets/codex-prompt.md
var codexPrompt string

//go:embed assets/evaluation-candidates.json
var candidateSnapshot []byte

type modelMetadata struct {
	DisplayName string   `json:"display_name"`
	Context     int      `json:"context_window"`
	Modalities  []string `json:"input_modalities"`
	Responses   bool     `json:"responses_api"`
	Tools       bool     `json:"function_calling"`
}

func metadataFor(identity string) (modelMetadata, error) {
	var snapshot struct {
		Models map[string]modelMetadata `json:"models"`
	}
	if err := json.Unmarshal(candidateSnapshot, &snapshot); err != nil {
		return modelMetadata{}, errors.New("invalid bundled model metadata")
	}
	candidate, known := snapshot.Models[identity]
	if !known {
		return candidate, errors.New("missing bundled model metadata")
	}
	if candidate.DisplayName == "" || candidate.Context <= 4096 || !slices.Contains(candidate.Modalities, "text") || !candidate.Responses || !candidate.Tools {
		return candidate, errors.New("incompatible model metadata: Codex requires text input, Responses, function calling and a context window above 4096")
	}
	for _, modality := range candidate.Modalities {
		if modality != "text" && modality != "image" {
			return candidate, errors.New("incompatible model input modality")
		}
	}
	return candidate, nil
}

func prepareModelCatalog(model, guardian string) (string, error) {
	return prepareModelCatalogInDir(model, guardian, "")
}

func prepareModelCatalogInDir(model, guardian, dir string) (string, error) {
	identities := []string{model}
	if guardian != "" && guardian != model {
		identities = append(identities, guardian)
	}
	entries := []any{}
	for _, identity := range identities {
		reviewer := ""
		if identity == model {
			reviewer = guardian
		}
		entry, err := modelCatalogEntry(identity, reviewer)
		if err != nil {
			return "", err
		}
		entries = append(entries, entry)
	}
	return writeModelCatalog(entries, dir)
}

func modelCatalogEntry(identity, guardian string) (map[string]any, error) {
	candidate, err := metadataFor(identity)
	if err != nil {
		return nil, err
	}
	entry := map[string]any{
		"slug":                                 identity,
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
	}
	if guardian != "" {
		entry["auto_review_model_override"] = guardian
	}
	return entry, nil
}

func writeModelCatalog(entries []any, dir string) (string, error) {
	catalog := map[string]any{"models": entries}
	data, err := json.Marshal(catalog)
	if err != nil {
		return "", errors.New("could not encode launch model catalog")
	}
	file, err := os.CreateTemp(dir, "tofa-model-catalog-*.json")
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
