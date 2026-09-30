package tofa_test

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestDesktopMinimumVersions(t *testing.T) {
	for _, tc := range []struct {
		name, appVersion, engineVersion string
		accepted                        bool
	}{
		{"minimum", "26.917.71314", "0.155.0-alpha.16.4", true},
		{"updated desktop", "26.928.21956", "0.159.2", true},
		{"later numeric prerelease", "26.917.71314", "0.155.0-alpha.16.10", true},
		{"stable engine", "26.917.71314", "0.155.0", true},
		{"future compatible version", "27.0.0", "1.0.0", true},
		{"old desktop", "26.917.71313", "0.159.2", false},
		{"old engine", "26.928.21956", "0.155.0-alpha.16.3", false},
		{"invalid desktop version", "26.unknown", "0.159.2", false},
		{"invalid engine version", "26.928.21956", "0.159.2junk", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			bundle, capture := desktopFixture(t, "normal")
			plist := filepath.Join(bundle, "Contents/Info.plist")
			data, err := os.ReadFile(plist)
			if err != nil {
				t.Fatal(err)
			}
			text := strings.ReplaceAll(string(data), "26.917.71314", tc.appVersion)
			text = strings.ReplaceAll(text, "10954", "12404")
			if err := os.WriteFile(plist, []byte(text), 0600); err != nil {
				t.Fatal(err)
			}
			engine := filepath.Join(bundle, "Contents/Resources/codex")
			data, err = os.ReadFile(engine)
			if err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(engine, []byte(strings.ReplaceAll(string(data), "0.155.0-alpha.16.4", tc.engineVersion)), 0700); err != nil {
				t.Fatal(err)
			}
			app, _ := adapterFixture(t, nil, nil)
			err = app.Run([]string{"launch", "codex-desktop", "--app-bundle", bundle, "--model", "deepseek-ai/DeepSeek-V4.1-Flash"})
			if tc.accepted {
				if err != nil {
					t.Fatalf("compatible version rejected: %v", err)
				}
				if _, err := os.Stat(capture); err != nil {
					t.Fatal("accepted client never launched")
				}
			} else {
				if err == nil || !strings.Contains(err.Error(), "version") {
					t.Fatalf("expected explicit version refusal, got %v", err)
				}
				if _, err := os.Stat(capture); !os.IsNotExist(err) {
					t.Fatal("incompatible version launched")
				}
			}
		})
	}
}

func TestDesktopSavedReferenceFollowsBundleEngineMove(t *testing.T) {
	bundle, capture := desktopFixture(t, "ignore")
	app, _ := adapterFixture(t, nil, nil)
	child, stop := liveDesktopFixture(t, app, bundle, capture, "--model", "deepseek-ai/DeepSeek-V4.1-Flash")
	stop()
	legacy := filepath.Join(bundle, "Contents/Resources/codex")
	packaged := filepath.Join(bundle, "Contents/Resources/codex-cli/CodexCLI.app/Contents/MacOS/codex")
	if err := os.MkdirAll(filepath.Dir(packaged), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.Rename(legacy, packaged); err != nil {
		t.Fatal(err)
	}
	command := exec.Command(child.Env["CODEX_CLI_PATH"], "--version")
	command.Env = []string{"PATH=" + os.Getenv("PATH"), "HOME=" + os.Getenv("HOME")}
	var diagnostics bytes.Buffer
	command.Stderr = &diagnostics
	output, err := command.Output()
	if err != nil || strings.TrimSpace(string(output)) != "codex-cli 0.155.0-alpha.16.4" {
		t.Fatalf("saved desktop reference stopped working after an engine move: %v %s", err, diagnostics.String())
	}
	if !strings.Contains(diagnostics.String(), "engine moved") {
		t.Fatal("engine relocation was not announced")
	}
}

func TestDesktopPackagedEngineLayout(t *testing.T) {
	for _, legacyPresent := range []bool{false, true} {
		t.Run(map[bool]string{false: "packaged only", true: "packaged takes precedence"}[legacyPresent], func(t *testing.T) {
			bundle, capture := desktopFixture(t, "normal")
			legacy := filepath.Join(bundle, "Contents/Resources/codex")
			packaged := filepath.Join(bundle, "Contents/Resources/codex-cli/CodexCLI.app/Contents/MacOS/codex")
			if err := os.MkdirAll(filepath.Dir(packaged), 0700); err != nil {
				t.Fatal(err)
			}
			if err := os.Rename(legacy, packaged); err != nil {
				t.Fatal(err)
			}
			if legacyPresent {
				if err := os.WriteFile(legacy, []byte("#!/bin/sh\necho obsolete-flat-engine >&2\nexit 1\n"), 0700); err != nil {
					t.Fatal(err)
				}
			}
			app, _ := adapterFixture(t, nil, nil)
			if err := app.Run([]string{"launch", "codex-desktop", "--app-bundle", bundle, "--model", "deepseek-ai/DeepSeek-V4.1-Flash"}); err != nil {
				t.Fatalf("packaged native engine did not launch: %v", err)
			}
			if _, err := os.Stat(capture); err != nil {
				t.Fatal("packaged desktop never started")
			}
		})
	}
}

func TestDesktopIncompleteEnginePackageIsRefused(t *testing.T) {
	bundle, capture := desktopFixture(t, "normal")
	if err := os.MkdirAll(filepath.Join(bundle, "Contents/Resources/codex-cli"), 0700); err != nil {
		t.Fatal(err)
	}
	app, _ := adapterFixture(t, nil, nil)
	err := app.Run([]string{"launch", "codex-desktop", "--app-bundle", bundle, "--model", "deepseek-ai/DeepSeek-V4.1-Flash"})
	if err == nil || !strings.Contains(err.Error(), "bundled Codex engine") {
		t.Fatalf("incomplete package silently used the obsolete flat engine: %v", err)
	}
	if _, err := os.Stat(capture); !os.IsNotExist(err) {
		t.Fatal("incomplete app bundle launched")
	}
}
