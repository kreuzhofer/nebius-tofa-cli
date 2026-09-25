package tofa

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"

	"github.com/kreuzhofer/nebius-tofa-cli/scripts"
)

// The installer and uninstaller use the same lease as desktop launches. It
// remains held until the installed operation and its child script have exited.
func (a *App) desktopLifecycle(ctx context.Context, args []string) error {
	if len(args) == 0 || (args[0] != "install" && args[0] != "uninstall") {
		return errors.New("expected desktop-lifecycle install or uninstall")
	}
	profile, err := ordinaryDesktopProfile()
	if err != nil {
		return err
	}
	lease, err := acquireDesktopLease(profile)
	if err != nil {
		return err
	}
	defer lease.Close()
	if err := refuseDesktopProcesses(ctx, profile); err != nil {
		return err
	}
	if args[0] == "uninstall" && (len(args) > 2 || (len(args) == 2 && args[1] != "--purge")) {
		return errors.New("expected desktop-lifecycle uninstall [--purge]")
	}
	if args[0] == "install" && (len(args) != 2 || !filepath.IsAbs(args[1])) {
		return errors.New("expected desktop-lifecycle install ABSOLUTE_INSTALL_DIRECTORY")
	}
	if info, err := os.Lstat(a.Dir); err == nil {
		if !info.IsDir() {
			return errors.New("configuration directory must be a real directory; desktop lifecycle stopped")
		}
	} else if !errors.Is(err, os.ErrNotExist) {
		return err
	}
	if err := checkDesktopRuntimeOwners(a.Dir, profile); err != nil {
		return err
	}
	bridges, err := ownedDesktopBridges(a.Dir)
	if err != nil {
		return err
	}
	if args[0] == "install" {
		root := args[1]
		for _, path := range []string{root, filepath.Join(root, "bin")} {
			info, err := os.Lstat(path)
			if err != nil || !info.IsDir() {
				return errors.New("installation must have real root and bin directories")
			}
		}
		manifest, err := os.ReadFile(filepath.Join(root, ".tofa-install"))
		if err != nil || strings.TrimSpace(string(manifest)) != "tofa-install-v1" {
			return errors.New("installation has no supported ownership manifest")
		}
		// A client update is not automatically supported. Refuse the whole
		// operation before changing any helper if any recorded bundle differs.
		for _, owner := range bridges {
			if _, err := discoverDesktop(ctx, filepath.Dir(filepath.Dir(filepath.Dir(owner.Engine)))); err != nil {
				return fmt.Errorf("desktop upgrade stopped: %w; restore the qualified client or uninstall to detach its integration", err)
			}
		}
		for path, owner := range bridges {
			if err := replaceDesktopBridge(path, owner, false); err != nil {
				return err
			}
		}
		executable, err := os.Executable()
		if err != nil {
			return err
		}
		binary, err := os.ReadFile(executable)
		if err != nil {
			return err
		}
		return writeDesktopExecutable(filepath.Join(root, "bin/tofa"), binary)
	}
	for path, owner := range bridges {
		if err := replaceDesktopBridge(path, owner, true); err != nil {
			return err
		}
	}
	if _, err := os.Lstat(filepath.Join(a.Dir, "desktop-launches")); err == nil {
		// The ownership check above proved that every recorded runtime owner is
		// dead and belongs to this profile. Reuse normal crash recovery cleanup.
		runtime, err := prepareDesktopRuntime(a.Dir, profile, a.Out)
		if err != nil {
			return err
		}
		if err := os.RemoveAll(runtime); err != nil {
			return err
		}
	} else if !errors.Is(err, os.ErrNotExist) {
		return err
	}
	command := exec.CommandContext(ctx, "/bin/sh", append([]string{"-s", "--"}, args[1:]...)...)
	command.Stdin = strings.NewReader(scripts.UninstallSH)
	command.Stdout, command.Stderr = a.Out, os.Stderr
	command.Env = append(os.Environ(), "TOFA_DESKTOP_LIFECYCLE_PARENT="+strconv.Itoa(os.Getpid()))
	return command.Run()
}

func checkDesktopRuntimeOwners(dir, profile string) error {
	parent := filepath.Join(dir, "desktop-launches")
	info, err := os.Lstat(parent)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	if !info.IsDir() || checkPrivate(parent, info) != nil {
		return errors.New("unrecognized desktop runtime ownership; inspect desktop-launches before retrying")
	}
	entries, err := os.ReadDir(parent)
	if err != nil {
		return err
	}
	for _, entry := range entries {
		path := filepath.Join(parent, entry.Name())
		info, err := os.Lstat(path)
		if err != nil || !info.IsDir() || checkPrivate(path, info) != nil || !strings.HasPrefix(entry.Name(), "launch-") {
			return errors.New("unrecognized desktop runtime owner; inspect desktop-launches before retrying")
		}
		var owner desktopRuntimeOwner
		data, err := readPrivate(filepath.Join(path, "owner.json"))
		if err != nil || json.Unmarshal(data, &owner) != nil || owner.Version != 1 || owner.PID <= 0 || owner.Profile != profile {
			return errors.New("unrecognized or different desktop runtime owner; inspect desktop-launches before retrying")
		}
		if !errors.Is(syscall.Kill(owner.PID, 0), syscall.ESRCH) {
			return errors.New("desktop runtime owner may still be alive; wait for that launch to exit before changing its integration")
		}
	}
	return nil
}

// Preflight every record before changing any bridge; user-managed or partial
// artifacts must never be interpreted as permission to overwrite an executable.
func ownedDesktopBridges(dir string) (map[string]desktopBridgeOwner, error) {
	bridges := map[string]desktopBridgeOwner{}
	for _, version := range []string{"desktop-bridge-v1", "desktop-bridge-v2"} {
		parent := filepath.Join(dir, version)
		info, err := os.Lstat(parent)
		if errors.Is(err, os.ErrNotExist) {
			continue
		}
		if err != nil {
			return nil, err
		}
		if !info.IsDir() || checkPrivate(parent, info) != nil {
			return nil, fmt.Errorf("refusing unowned desktop bridge directory: %s", parent)
		}
		entries, err := os.ReadDir(parent)
		if err != nil {
			return nil, err
		}
		for _, entry := range entries {
			root := filepath.Join(parent, entry.Name())
			info, err := os.Lstat(root)
			if err != nil || !info.IsDir() || checkPrivate(root, info) != nil {
				return nil, fmt.Errorf("refusing unowned desktop bridge entry: %s", root)
			}
			path := filepath.Join(root, desktopBridgeName)
			owner, err := readDesktopBridge(path)
			if err != nil {
				return nil, fmt.Errorf("%s: %w", root, err)
			}
			id := sha256.Sum256([]byte(owner.Engine))
			if entry.Name() != hex.EncodeToString(id[:]) || version != fmt.Sprintf("desktop-bridge-v%d", owner.Version) {
				return nil, errors.New("desktop bridge path disagrees with ownership; restore matching artifacts before retrying")
			}
			bridges[path] = owner
		}
	}
	return bridges, nil
}

func replaceDesktopBridge(path string, owner desktopBridgeOwner, detached bool) error {
	executable, err := os.Executable()
	if err != nil {
		return err
	}
	binary, err := os.ReadFile(executable)
	if err != nil {
		return err
	}
	digest := sha256.Sum256(binary)
	next := hex.EncodeToString(digest[:])
	// Publish the new digest before the atomic rename. Until then an old helper
	// validates its original digest; afterwards the new helper accepts Pending.
	owner.PendingSHA256 = next
	data, err := json.Marshal(owner)
	if err != nil {
		return err
	}
	record := filepath.Join(filepath.Dir(path), "owner.json")
	if err := writePrivate(record, data); err != nil {
		return err
	}
	if err := writeDesktopExecutable(path, binary); err != nil {
		return err
	}
	owner.SHA256, owner.PendingSHA256, owner.Detached = next, "", detached
	owner.LifecycleVersion = 1
	data, err = json.Marshal(owner)
	if err != nil {
		return err
	}
	return writePrivate(record, data)
}

func writeDesktopExecutable(path string, binary []byte) error {
	file, err := os.CreateTemp(filepath.Dir(path), ".tofa-executable-*")
	if err != nil {
		return err
	}
	defer os.Remove(file.Name())
	if _, err := file.Write(binary); err != nil {
		file.Close()
		return err
	}
	if err := file.Chmod(0700); err != nil {
		file.Close()
		return err
	}
	if err := file.Sync(); err != nil {
		file.Close()
		return err
	}
	if err := file.Close(); err != nil {
		return err
	}
	return os.Rename(file.Name(), path)
}
