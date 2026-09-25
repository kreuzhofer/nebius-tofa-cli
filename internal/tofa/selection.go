package tofa

import (
	_ "embed"
	"encoding/json"
	"errors"
	"fmt"
)

//go:embed assets/model-verification.json
var verificationSnapshot []byte

func validateAvailableRole(models []Model, role, identity string) error {
	found := false
	for _, model := range models {
		if model.ID == identity {
			found = true
			break
		}
	}
	if !found {
		return fmt.Errorf("%s model %s: not available in this project's catalog", role, identity)
	}
	if _, err := metadataFor(identity); err != nil {
		return fmt.Errorf("%s model %s: %w", role, identity, err)
	}
	return nil
}

// A record approves a complete role combination; evidence for a main or a
// Guardian in another launch does not transfer to this selection.
func selectionStatus(target, route, main, guardian string, allow bool) (string, error) {
	var snapshot struct {
		Records []struct{ Target, Route, Main, Guardian, Status, Evidence string }
	}
	if err := json.Unmarshal(verificationSnapshot, &snapshot); err != nil || snapshot.Records == nil {
		return "", errors.New("invalid bundled model verification records")
	}
	for _, record := range snapshot.Records {
		if record.Evidence == "" || (record.Status != "supported" && record.Status != "experimental") {
			return "", errors.New("invalid bundled model verification record")
		}
		if record.Target == target && record.Route == route && record.Main == main && record.Guardian == guardian && record.Status == "supported" {
			return "supported", nil
		}
	}
	if !allow {
		return "", fmt.Errorf("unverified %s %s combination (main %s, Guardian %s); experimental selection requires --allow-unverified", target, route, main, guardian)
	}
	return "experimental (unverified)", nil
}
