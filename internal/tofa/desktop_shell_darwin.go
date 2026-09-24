package tofa

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// The qualified desktop uses this exact zsh -ilc environment query,
// including its delimiters; see docs/research/desktop-profile-ownership.md.
const desktopShellProbe = `printf '\0%s\0' '_SHELL_ENV_DELIMITER_'; command env -0 || exit; printf '\0%s\0' '_SHELL_ENV_DELIMITER_'; exit`

// Isolate only Electron's environment query. Other zsh invocations restore the
// caller's startup-file location before normal shell initialization continues.
// HOME is untouched so the engine still discovers native enforced policy.
func prepareDesktopShell(ctx context.Context, root string) (string, error) {
	quote := func(value string) string { return "'" + strings.ReplaceAll(value, "'", "'\\''") + "'" }
	restore := "unset ZDOTDIR\n"
	if original, ok := os.LookupEnv("ZDOTDIR"); ok {
		restore = "export ZDOTDIR=" + quote(original) + "\n"
	}
	script := "# Launcher-owned environment probe isolation; no ordinary startup files are modified.\n" +
		"if [[ ${CODEX_SHELL:-} == 1 && ${ZSH_EXECUTION_STRING:-} == " + quote(desktopShellProbe) + " ]]; then\n" +
		"  builtin printf '\\0%s\\0' '_SHELL_ENV_DELIMITER_'\n" +
		"  /usr/bin/env -0 || exit\n" +
		"  builtin printf '\\0%s\\0' '_SHELL_ENV_DELIMITER_'\n" +
		"  exit\n" +
		"fi\n" + restore +
		"if [[ -r ${ZDOTDIR-$HOME}/.zshenv ]]; then source \"${ZDOTDIR-$HOME}/.zshenv\"; fi\n"
	dir := filepath.Join(root, "shell")
	if err := os.Mkdir(dir, 0700); err != nil {
		return "", err
	}
	if err := os.WriteFile(filepath.Join(dir, ".zshenv"), []byte(script), 0600); err != nil {
		return "", errors.Join(err, os.Remove(dir))
	}
	return dir, nil
}

func checkDesktopLoginShell(ctx context.Context) error {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	username, err := exec.CommandContext(ctx, "/usr/bin/id", "-un").Output()
	if err != nil {
		return errors.New("could not determine desktop login shell")
	}
	loginShell, err := exec.CommandContext(ctx, "/usr/bin/dscl", "/Search", "-read", "/Users/"+strings.TrimSpace(string(username)), "UserShell").Output()
	if err != nil || strings.TrimSpace(string(loginShell)) != "UserShell: /bin/zsh" {
		return errors.New("codex-desktop requires the qualified /bin/zsh login shell; use launch codex with other shells")
	}
	return nil
}
