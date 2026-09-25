package tofa

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"syscall"
)

// This record proves cleanup scope, never grants routing access. Recovery runs
// only while holding the durable ordinary-profile lease and after refusing any
// surviving desktop. Unknown artifacts require manual inspection.
type desktopRuntimeOwner struct {
	Version int
	PID     int
	Profile string
}

func prepareDesktopRuntime(dir, profile string, out io.Writer) (string, error) {
	parent := filepath.Join(dir, "desktop-launches")
	if err := privateDir(parent); err != nil {
		return "", err
	}
	entries, err := os.ReadDir(parent)
	if err != nil {
		return "", err
	}
	for _, entry := range entries {
		path := filepath.Join(parent, entry.Name())
		info, err := os.Lstat(path)
		if err != nil {
			return "", err
		}
		var owner desktopRuntimeOwner
		if !strings.HasPrefix(entry.Name(), "launch-") || !info.IsDir() || checkPrivate(path, info) != nil {
			fmt.Fprintln(out, "Desktop recovery: retained an unrecognized runtime artifact; inspect desktop-launches manually.")
			continue
		}
		data, err := readPrivate(filepath.Join(path, "owner.json"))
		if err != nil || json.Unmarshal(data, &owner) != nil || owner.Version != 1 || owner.PID <= 0 || !filepath.IsAbs(owner.Profile) {
			fmt.Fprintln(out, "Desktop recovery: retained runtime artifacts with missing or invalid ownership; inspect desktop-launches manually.")
			continue
		}
		if owner.Profile != profile {
			continue
		}
		if !errors.Is(syscall.Kill(owner.PID, 0), syscall.ESRCH) {
			fmt.Fprintln(out, "Desktop recovery: retained runtime artifacts whose launcher may still be alive.")
			continue
		}
		// RemoveAll does not follow symlinks inside this private launch directory.
		// Never use a path supplied by the ownership record as a deletion target.
		if err := os.RemoveAll(path); err != nil {
			return "", fmt.Errorf("could not recover abandoned desktop runtime: %w", err)
		}
		fmt.Fprintln(out, "Desktop recovery: removed abandoned launch runtime after verifying its owner has exited.")
	}
	root, err := os.MkdirTemp(parent, "launch-")
	if err != nil {
		return "", err
	}
	data, err := json.Marshal(desktopRuntimeOwner{Version: 1, PID: os.Getpid(), Profile: profile})
	if err == nil {
		err = os.WriteFile(filepath.Join(root, "owner.json"), data, 0600)
	}
	if err != nil {
		return "", errors.Join(err, os.RemoveAll(root))
	}
	return root, nil
}
