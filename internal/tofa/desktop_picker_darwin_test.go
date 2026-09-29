package tofa_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

// Use the existing executable desktop boundary with real terminal input. The
// Python driver isolates HOME, launcher credentials and the loopback provider.
func TestDesktopPicker(t *testing.T) {
	bundle, capture := desktopFixture(t, "controlled")
	if installed := os.Getenv("TOFA_TEST_DESKTOP_ENGINE"); installed != "" {
		engine := filepath.Join(bundle, "Contents/Resources/codex")
		if err := os.Remove(engine); err != nil {
			t.Fatal(err)
		}
		if err := os.Symlink(installed, engine); err != nil {
			t.Fatal(err)
		}
	}
	command := exec.Command("python3", "scripts/desktop_picker_test.py", "-v")
	command.Dir = "../.."
	command.Env = append(os.Environ(), "FIXTURE_DESKTOP_BUNDLE="+bundle, "FIXTURE_DESKTOP_CAPTURE="+capture)
	if output, err := command.CombinedOutput(); err != nil {
		t.Fatalf("desktop picker: %v\n%s", err, output)
	} else {
		t.Log(string(output))
	}
}
