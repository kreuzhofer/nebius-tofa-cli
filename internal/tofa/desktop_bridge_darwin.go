package tofa

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"syscall"
	"time"
)

const desktopBridgeName = "tofa-desktop-engine"

// This versioned ownership record contains installation data only, never routing.
type desktopBridgeOwner struct {
	Version int
	Engine  string
	SHA256  string
}

func desktopBridgePath(dir, engine string) string {
	id := sha256.Sum256([]byte(engine))
	return filepath.Join(dir, "desktop-bridge-v1", hex.EncodeToString(id[:]), desktopBridgeName)
}

func readDesktopBridge(path string) (desktopBridgeOwner, error) {
	var owner desktopBridgeOwner
	data, err := readPrivate(filepath.Join(filepath.Dir(path), "owner.json"))
	if err != nil || json.Unmarshal(data, &owner) != nil || owner.Version != 1 || !filepath.IsAbs(owner.Engine) {
		return owner, errors.New("desktop bridge ownership is missing or invalid; refusing to replace an unmanaged executable")
	}
	binary, err := readPrivate(path)
	if err != nil {
		return owner, err
	}
	digest := sha256.Sum256(binary)
	if hex.EncodeToString(digest[:]) != owner.SHA256 {
		return owner, errors.New("desktop bridge executable differs from its ownership record; refusing to replace it")
	}
	return owner, nil
}

func installDesktopBridge(dir, engine string) (path string, result error) {
	path = desktopBridgePath(dir, engine)
	if override, ok := os.LookupEnv("CODEX_CLI_PATH"); ok && override != "" && override != path {
		return "", errors.New("CODEX_CLI_PATH is a user-managed executable override; unset it before launching codex-desktop")
	}
	parent := filepath.Dir(filepath.Dir(path))
	if err := privateDir(parent); err != nil {
		return "", err
	}
	root := filepath.Dir(path)
	if err := os.Mkdir(root, 0700); errors.Is(err, os.ErrExist) {
		info, statErr := os.Lstat(root)
		if statErr != nil || !info.IsDir() || info.Mode().Perm()&0077 != 0 {
			return "", errors.New("desktop bridge directory must be a private, real directory")
		}
		owner, err := readDesktopBridge(path)
		if err != nil {
			return "", err
		}
		if owner.Engine != engine {
			return "", errors.New("desktop bridge belongs to a different bundled engine")
		}
		return path, nil
	} else if err != nil {
		return "", err
	}
	// Only a directory created by this invocation is eligible for setup rollback.
	defer func() {
		if result != nil {
			result = errors.Join(result, os.RemoveAll(root))
		}
	}()
	executable, err := os.Executable()
	if err != nil {
		return "", err
	}
	source, err := os.Open(executable)
	if err != nil {
		return "", err
	}
	defer source.Close()
	target, err := os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0700)
	if err != nil {
		return "", err
	}
	digest := sha256.New()
	_, copyErr := io.Copy(io.MultiWriter(target, digest), source)
	if err := errors.Join(copyErr, target.Close()); err != nil {
		return "", err
	}
	owner, err := json.Marshal(desktopBridgeOwner{Version: 1, Engine: engine, SHA256: hex.EncodeToString(digest.Sum(nil))})
	if err != nil {
		return "", err
	}
	if err := os.WriteFile(filepath.Join(root, "owner.json"), owner, 0600); err != nil {
		return "", err
	}
	return path, nil
}

func runDesktopBridge(args []string) error {
	path, err := os.Executable()
	if err != nil {
		return err
	}
	owner, err := readDesktopBridge(path)
	if err != nil {
		return err
	}
	env := os.Environ()
	if endpoint, present := os.LookupEnv("TOFA_DESKTOP_CONTEXT"); present {
		route, err := fetchDesktopRoute(endpoint, os.Getenv("TOFA_API_KEY"))
		if err != nil {
			return err
		}
		if route.Bridge != path || route.Engine != owner.Engine || route.Home != os.Getenv("CODEX_HOME") {
			return errors.New("desktop launch context does not match this bridge or home; relaunch through tofa")
		}
		// Codex's app-server has its own -c parser. Global -c values can be
		// displaced when Electron supplies any subcommand overrides.
		if index := desktopAppServerIndex(args); index >= 0 {
			end := len(args)
			if separator := slices.Index(args[index+1:], "--"); separator >= 0 {
				end = index + 1 + separator
			}
			merged := append([]string{}, args[:end]...)
			for _, value := range route.Overrides {
				merged = append(merged, "-c", value)
			}
			args = append(merged, args[end:]...)
		}
	} else {
		// An ordinary invocation must not revive inactive Token Factory metadata
		// using a bearer inherited from an earlier launch.
		env = slices.DeleteFunc(env, func(entry string) bool { return strings.HasPrefix(entry, "TOFA_API_KEY=") })
	}
	if err := syscall.Exec(owner.Engine, append([]string{owner.Engine}, args...), env); err != nil {
		return fmt.Errorf("desktop bridge could not execute the installed bundled engine: %w", err)
	}
	return nil
}

// The pinned desktop prefixes app-server with global config/feature options.
// Stop at any other command, prompt, or separator; argument values are never
// subcommands. Unrecognized invocation shapes are passed to Codex unchanged.
func desktopAppServerIndex(args []string) int {
	for index := 0; index < len(args); index++ {
		arg := args[index]
		switch arg {
		case "app-server":
			return index
		case "-c", "--config", "--enable", "--disable":
			index++
		case "--strict-config":
		default:
			if !strings.HasPrefix(arg, "--config=") && !strings.HasPrefix(arg, "--enable=") && !strings.HasPrefix(arg, "--disable=") && !(strings.HasPrefix(arg, "-c") && len(arg) > 2) {
				return -1
			}
		}
	}
	return -1
}

func fetchDesktopRoute(endpoint, token string) (desktopRoute, error) {
	var route desktopRoute
	invalid := errors.New("desktop launch context is invalid or expired; relaunch through tofa")
	u, err := url.Parse(endpoint)
	if err != nil || u.Scheme != "http" || u.Hostname() != "127.0.0.1" || u.User != nil || u.Path != "" || u.RawQuery != "" || u.Fragment != "" {
		return route, invalid
	}
	port, err := strconv.Atoi(u.Port())
	if err != nil || port < 1 || port > 65535 {
		return route, invalid
	}
	credential, err := hex.DecodeString(token)
	if err != nil || len(credential) != 32 {
		return route, invalid
	}
	request, err := http.NewRequest(http.MethodGet, endpoint+"/desktop-launch", nil)
	if err != nil {
		return route, invalid
	}
	request.Header.Set("Authorization", "Bearer "+token)
	transport := &http.Transport{Proxy: nil}
	defer transport.CloseIdleConnections()
	client := &http.Client{Transport: transport, Timeout: 3 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	response, err := client.Do(request)
	if err != nil {
		return route, invalid
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK || json.NewDecoder(io.LimitReader(response.Body, 64<<10)).Decode(&route) != nil || len(route.Overrides) == 0 {
		return route, invalid
	}
	return route, nil
}
