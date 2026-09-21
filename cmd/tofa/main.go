package main

import (
	"errors"
	"fmt"
	"github.com/kreuzhofer/nebius-tofa-cli/internal/tofa"
	"github.com/kreuzhofer/nebius-tofa-cli/scripts"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
)

var version = "dev-prototype"

func main() {
	app := tofa.App{Version: version, Uninstall: uninstall}
	if err := app.Run(os.Args[1:]); err != nil {
		var exit *exec.ExitError
		if errors.As(err, &exit) {
			code := exit.ExitCode()
			if code < 0 {
				code = 1
			}
			os.Exit(code)
		}
		fmt.Fprintln(os.Stderr, "tofa:", err)
		os.Exit(1)
	}
}
func uninstall(purge bool) error {
	executable, err := os.Executable()
	if err != nil {
		return err
	}
	root := filepath.Dir(filepath.Dir(executable))
	manifest, err := os.ReadFile(filepath.Join(root, ".tofa-install"))
	if err != nil || strings.TrimSpace(string(manifest)) != "tofa-install-v1" {
		return errors.New("this binary has no installer ownership manifest; use the standalone uninstall script for an installed copy")
	}
	env := []string{}
	for _, item := range os.Environ() {
		if !strings.HasPrefix(item, "TOFA_INSTALL_DIR=") {
			env = append(env, item)
		}
	}
	env = append(env, "TOFA_INSTALL_DIR="+root)
	if runtime.GOOS == "windows" {
		f, err := os.CreateTemp("", "tofa-uninstall-*.ps1")
		if err != nil {
			return err
		}
		path := f.Name()
		// The helper waits for this process to release the executable, then removes itself.
		content := scripts.UninstallPS + "\nRemove-Item -LiteralPath $PSCommandPath\n"
		if _, err = f.WriteString(content); err != nil {
			f.Close()
			os.Remove(path)
			return err
		}
		if err = f.Close(); err != nil {
			return err
		}
		args := []string{"-NoProfile", "-File", path, "-WaitPid", strconv.Itoa(os.Getpid())}
		if purge {
			args = append(args, "-Purge")
		}
		cmd := exec.Command("powershell.exe", args...)
		cmd.Env = env
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		if err = cmd.Start(); err != nil {
			os.Remove(path)
			return err
		}
		fmt.Println("Uninstaller started; it will report completion after tofa exits.")
		return nil
	}
	args := []string{"-s", "--"}
	if purge {
		args = append(args, "--purge")
	}
	cmd := exec.Command("sh", args...)
	cmd.Env = env
	cmd.Stdin = strings.NewReader(scripts.UninstallSH)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}
