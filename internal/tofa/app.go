// Package tofa implements the experimental direct Token Factory launcher.
package tofa

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"os/signal"
	"strconv"
	"strings"
	"syscall"

	"golang.org/x/term"
)

type App struct {
	Out     io.Writer
	Version string
	Dir     string
	Vault   Vault
	// Endpoint and HTTP exist for local protocol tests. The CLI never overrides the endpoint.
	Endpoint  string
	HTTP      *http.Client
	RunClient func(args, env []string) error
	Prompt    func(label string, secret bool) (string, error)
	Uninstall func(purge bool) error
}

const help = `tofa — direct Token Factory launcher (prototype)

  tofa                                      Interactive launcher
  tofa auth login [--storage keyring|file]   Save API key and project ID
  tofa auth logout                          Remove locally saved credentials
  tofa models [--project-id ID]              List available models
  tofa launch codex --model ID [--project-id ID] [--allow-unverified] [-- ARGS]
  tofa doctor                               Check local prerequisites; no inference
  tofa uninstall [--purge]                   Remove installation; optionally saved data
  tofa --version

No model/client combination is verified yet. Explicit --allow-unverified is
required for experimental launches. Models in the catalog are not certified.
`

func (a *App) Run(args []string) error {
	if a.Out == nil {
		a.Out = os.Stdout
	}
	if a.Vault == nil {
		a.Vault = NativeVault{}
	}
	if len(args) == 1 && args[0] == "--version" {
		_, err := fmt.Fprintln(a.Out, "tofa", a.Version)
		return err
	}
	if len(args) == 1 && (args[0] == "--help" || args[0] == "help" || args[0] == "-h") {
		fmt.Fprint(a.Out, help)
		return nil
	}
	if a.Dir == "" {
		var err error
		a.Dir, err = ConfigDir()
		if err != nil {
			return err
		}
	}
	s := Store{Dir: a.Dir, Vault: a.Vault}
	if len(args) == 0 {
		return a.interactive(s)
	}
	switch args[0] {
	case "auth":
		if len(args) < 2 {
			return errors.New("use tofa auth login or tofa auth logout")
		}
		if args[1] == "logout" && len(args) == 2 {
			if err := s.Logout(); err != nil {
				return err
			}
			fmt.Fprintln(a.Out, "Local credentials removed. Preferences retained; the Nebius API key remains valid.")
			return nil
		}
		if args[1] != "login" {
			return errors.New("use tofa auth login or tofa auth logout")
		}
		fs := flags("auth login")
		backend := fs.String("storage", "keyring", "")
		if err := fs.Parse(args[2:]); err != nil {
			return err
		}
		if fs.NArg() != 0 {
			return errors.New("unexpected login argument")
		}
		if *backend != "keyring" && *backend != "file" {
			return errors.New("storage must be keyring or file")
		}
		if *backend == "file" {
			fmt.Fprintln(a.Out, "Plaintext storage explicitly selected. File permissions restrict access; they do not encrypt the key.")
		}
		key, err := a.ask("API key", true)
		if err != nil {
			return err
		}
		project, err := a.ask("Project ID", false)
		if err != nil {
			return err
		}
		if err = s.Login(project, key, *backend); err != nil {
			return err
		}
		fmt.Fprintf(a.Out, "Credentials saved using %s. Remote authentication has not been tested.\n", *backend)
		return nil
	case "models":
		fs := flags("models")
		project := fs.String("project-id", "", "")
		if err := fs.Parse(args[1:]); err != nil {
			return err
		}
		if fs.NArg() != 0 {
			return errors.New("unexpected models argument")
		}
		c, key, err := s.Credentials()
		if err != nil {
			return err
		}
		if *project != "" {
			c.ProjectID = *project
		}
		if !validText(c.ProjectID, 256) {
			return errors.New("invalid project ID")
		}
		models, err := a.models(c.ProjectID, key)
		if err != nil {
			return err
		}
		fmt.Fprintln(a.Out, "MODEL\tCODEX COMPATIBILITY")
		for _, m := range models {
			fmt.Fprintln(a.Out, m.ID+"\tunverified")
		}
		if len(models) == 0 {
			fmt.Fprintln(a.Out, "No models available for this project.")
		}
		return nil
	case "launch":
		return a.launch(s, args[1:])
	case "doctor":
		if len(args) != 1 {
			return errors.New("doctor takes no arguments")
		}
		fmt.Fprintln(a.Out, "Local checks only; no network request or inference.")
		c, err := s.Config()
		if err != nil {
			return err
		}
		fmt.Fprintln(a.Out, "Configuration:", a.Dir)
		if c.Reference == "" {
			return errors.New("no saved login; run tofa auth login")
		}
		fmt.Fprintf(a.Out, "Credential backend: %s (access not tested; no keychain prompt)\n", c.Backend)
		if _, err = exec.LookPath("codex"); err != nil {
			return errors.New("Codex CLI not found on PATH")
		}
		fmt.Fprintln(a.Out, "Codex CLI found. Live protocol/model compatibility remains unverified.")
		return nil
	case "uninstall":
		fs := flags("uninstall")
		purge := fs.Bool("purge", false, "")
		if err := fs.Parse(args[1:]); err != nil {
			return err
		}
		if fs.NArg() != 0 {
			return errors.New("unexpected uninstall argument")
		}
		if a.Uninstall == nil {
			return errors.New("uninstall is unavailable in this build; use the standalone uninstall script")
		}
		return a.Uninstall(*purge)
	}
	return errors.New("unknown command; run tofa --help")
}
func flags(name string) *flag.FlagSet {
	fs := flag.NewFlagSet(name, flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	return fs
}
func (a *App) ask(label string, secret bool) (string, error) {
	if a.Prompt != nil {
		return a.Prompt(label, secret)
	}
	if !term.IsTerminal(int(os.Stdin.Fd())) {
		return "", errors.New("interactive terminal required; run tofa auth login in a terminal")
	}
	fmt.Fprint(a.Out, label+": ")
	if secret {
		b, err := readPassword()
		fmt.Fprintln(a.Out)
		if err != nil {
			return "", errors.New("credential input cancelled")
		}
		return strings.TrimSpace(string(b)), nil
	}
	// Read one byte at a time: a buffered reader must not consume the next prompt's input.
	var b strings.Builder
	for {
		one := make([]byte, 1)
		_, err := os.Stdin.Read(one)
		if err != nil {
			return "", errors.New("input cancelled")
		}
		if one[0] == '\n' {
			break
		}
		if b.Len() > 4096 {
			return "", errors.New("input too long")
		}
		b.WriteByte(one[0])
	}
	return strings.TrimSpace(b.String()), nil
}
func (a *App) launch(s Store, args []string) error {
	if len(args) == 0 || args[0] != "codex" {
		return errors.New("only Codex CLI is available in this prototype")
	}
	fs := flags("launch codex")
	model := fs.String("model", "", "")
	project := fs.String("project-id", "", "")
	allow := fs.Bool("allow-unverified", false, "")
	if err := fs.Parse(args[1:]); err != nil {
		return err
	}
	extra := fs.Args()
	if !*allow {
		return errors.New("no verified Codex/model combinations yet; experimental testing requires --allow-unverified")
	}
	c, key, err := s.Credentials()
	if err != nil {
		return err
	}
	if *model == "" {
		*model = c.Model
	}
	if *project != "" {
		c.ProjectID = *project
	}
	if !validText(*model, 512) {
		return errors.New("specify a valid --model ID; see tofa models")
	}
	if !validText(c.ProjectID, 256) {
		return errors.New("invalid project ID")
	}
	child, err := childArgs(*model, c.ProjectID, extra)
	if err != nil {
		return err
	}
	runner := a.RunClient
	if runner == nil {
		if _, err = exec.LookPath("codex"); err != nil {
			return errors.New("Codex CLI not found on PATH; install it first")
		}
		runner = runClient
	}
	models, err := a.models(c.ProjectID, key)
	if err != nil {
		return err
	}
	found := false
	for _, m := range models {
		if m.ID == *model {
			found = true
			break
		}
	}
	if !found {
		return errors.New("selected model is not available in this project's catalog")
	}
	fmt.Fprintf(a.Out, "Launching Codex with %s (unverified), project %s.\n", *model, c.ProjectID)
	return runner(child, childEnv(key))
}
func (a *App) interactive(s Store) error {
	if a.Prompt == nil && !term.IsTerminal(int(os.Stdin.Fd())) {
		return errors.New("choose a command for noninteractive use; run tofa --help")
	}
	c, key, err := s.Credentials()
	if err != nil {
		return err
	}
	fmt.Fprintln(a.Out, "Target client: Codex CLI\nNo verified models yet. Experimental launch is available explicitly.")
	answer, err := a.ask("List unverified models for experimental testing? [y/N]", false)
	if err != nil {
		return err
	}
	if strings.ToLower(answer) != "y" {
		return nil
	}
	models, err := a.models(c.ProjectID, key)
	if err != nil {
		return err
	}
	if len(models) == 0 {
		return errors.New("no models available")
	}
	for i, m := range models {
		fmt.Fprintf(a.Out, "%d. %s [unverified]\n", i+1, m.ID)
	}
	answer, err = a.ask("Model number", false)
	if err != nil {
		return err
	}
	n, err := strconv.Atoi(answer)
	if err != nil || n < 1 || n > len(models) {
		return errors.New("invalid model selection")
	}
	return a.launch(s, []string{"codex", "--model", models[n-1].ID, "--allow-unverified"})
}

// Intercept cancellation while terminal echo is disabled, restoring state before
// returning. The process exits after this error; no subsequent prompt is started.
func readPassword() ([]byte, error) {
	fd := int(os.Stdin.Fd())
	signals := make(chan os.Signal, 1)
	signal.Notify(signals, os.Interrupt, syscall.SIGTERM)
	defer signal.Stop(signals)
	state, err := term.MakeRaw(fd)
	if err != nil {
		return nil, err
	}
	defer term.Restore(fd, state)
	type result struct {
		value []byte
		err   error
	}
	done := make(chan result, 1)
	go func() {
		value := []byte{}
		for {
			var ch [1]byte
			if _, e := os.Stdin.Read(ch[:]); e != nil {
				done <- result{nil, e}
				return
			}
			switch ch[0] {
			case 3, 4:
				done <- result{nil, errors.New("input cancelled")}
				return
			case 10, 13:
				done <- result{value, nil}
				return
			case 8, 127:
				if len(value) > 0 {
					value = value[:len(value)-1]
				}
			default:
				if len(value) >= 2048 {
					done <- result{nil, errors.New("key too long")}
					return
				}
				value = append(value, ch[0])
			}
		}
	}()
	select {
	case r := <-done:
		return r.value, r.err
	case <-signals:
		return nil, errors.New("credential input cancelled")
	}
}
