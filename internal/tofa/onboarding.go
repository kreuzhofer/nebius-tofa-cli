package tofa

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"os"
	"path/filepath"

	"golang.org/x/term"
)

// Only a valid configuration without a credential reference indicates first use.
// Missing or inaccessible saved credentials must remain explicit recovery errors.
func (a *App) onboard(ctx context.Context, s Store, project string) error {
	c, err := s.Config()
	if err != nil {
		return err
	}
	if !term.IsTerminal(int(os.Stdin.Fd())) {
		return nil
	}
	if c.Reference != "" {
		_, _, err := s.Credentials()
		return err
	}
	// Leftover files/markers indicate interrupted setup, not a clean first use.
	if _, err := os.Lstat(filepath.Join(s.Dir, "credentials.yml")); err == nil {
		return errors.New("credential recovery required: credentials.yml exists without a saved reference; run tofa auth login or auth logout explicitly")
	} else if !errors.Is(err, os.ErrNotExist) {
		return err
	}
	entries, err := os.ReadDir(filepath.Join(s.Dir, "keyring-refs"))
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	if len(entries) != 0 {
		return errors.New("credential recovery required: pending vault references; run tofa auth logout explicitly when the vault is available")
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	if _, err := fmt.Fprintln(a.Out, "First-use setup: connect to Nebius Token Factory."); err != nil {
		return err
	}
	if err := a.login(s, nil); err != nil {
		return err
	}
	c, key, err := s.Credentials()
	if err != nil {
		return err
	}
	if project != "" {
		c.ProjectID = project
	}
	models, err := a.models(c.ProjectID, key)
	if err != nil {
		return err
	}
	if len(models) == 0 {
		return errors.New("no models available in this project's catalog")
	}
	_, err = fmt.Fprintln(a.Out, "Catalog authentication succeeded. Continuing launch.")
	return err
}

func (a *App) login(s Store, args []string) error {
	fs := flags("auth login")
	backend := fs.String("storage", "", "")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() != 0 {
		return errors.New("unexpected login argument")
	}
	explicit := false
	fs.Visit(func(f *flag.Flag) { explicit = explicit || f.Name == "storage" })
	if explicit && *backend != "keyring" && *backend != "file" {
		return errors.New("storage must be keyring or file")
	}
	if !explicit {
		c, err := s.Config()
		if err != nil {
			return err
		}
		*backend = c.Backend
		if *backend == "" {
			*backend = "keyring"
			probe, ok := a.Vault.(interface{ Availability() error })
			if !ok {
				return errors.New("credential vault availability is unknown; choose --storage keyring or --storage file explicitly")
			}
			if err := probe.Availability(); errors.Is(err, ErrVaultAbsent) {
				*backend = "file"
				if _, err := fmt.Fprintln(a.Out, "Credential vault facility absent or unsupported; file storage selected automatically."); err != nil {
					return err
				}
			} else if err != nil {
				return errors.New("cannot determine credential vault availability; check the vault is unlocked and access is allowed, or explicitly choose --storage file")
			}
		} else {
			if _, err := fmt.Fprintf(a.Out, "Reusing saved %s storage choice.\n", *backend); err != nil {
				return err
			}
		}
	}
	if *backend == "file" {
		if explicit {
			if _, err := fmt.Fprintln(a.Out, "Plaintext storage explicitly selected."); err != nil {
				return err
			}
		}
		if _, err := fmt.Fprintf(a.Out, "Credentials file: %s (unencrypted). File permissions restrict access; they do not encrypt the key.\n", filepath.Join(s.Dir, "credentials.yml")); err != nil {
			return err
		}
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
}
