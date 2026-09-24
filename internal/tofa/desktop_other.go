//go:build !darwin

package tofa

import (
	"context"
	"errors"
)

func (a *App) launchDesktop(context.Context, Store, []string) error {
	return errors.New("experimental codex-desktop requires the tested macOS arm64 application; use launch codex for the CLI")
}

func runDesktopBridge([]string) error {
	return errors.New("desktop engine bridge requires macOS")
}
