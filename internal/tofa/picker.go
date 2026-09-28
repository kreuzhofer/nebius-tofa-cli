package tofa

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"golang.org/x/term"
)

// The catalog remains in scrollback; only the current selection line changes.
// Read exactly one byte on demand so confirming never consumes client input.
func (a *App) pickMainModel(ctx context.Context, choices []modelChoice) (identity string, result error) {
	eligible := []string{}
	if _, err := fmt.Fprintln(a.Out, "Target client: Codex CLI\nAvailable main models:"); err != nil {
		return "", err
	}
	for _, choice := range choices {
		label := choice.status
		if choice.disabled != "" {
			label = "disabled: " + choice.disabled
		} else {
			eligible = append(eligible, choice.identity)
		}
		if _, err := fmt.Fprintf(a.Out, "  %s [%s]\n", choice.identity, label); err != nil {
			return "", err
		}
	}
	if len(eligible) == 0 {
		return "", errors.New("no eligible main models; all available entries are disabled")
	}
	ctx, stop := signal.NotifyContext(ctx, os.Interrupt, syscall.SIGTERM)
	defer stop()
	fd := int(os.Stdin.Fd())
	state, err := term.MakeRaw(fd)
	if err != nil {
		return "", fmt.Errorf("cannot prepare model picker terminal: %w", err)
	}
	defer func() {
		result = errors.Join(result, term.Restore(fd, state))
	}()
	if _, err := fmt.Fprint(a.Out, "Up/Down selects; Enter confirms; Escape/Ctrl-C cancels.\r\n"); err != nil {
		return "", err
	}
	type input struct {
		b   byte
		err error
	}
	read := func(timeout <-chan time.Time) (byte, error) {
		done := make(chan input, 1)
		go func() {
			var b [1]byte
			_, err := os.Stdin.Read(b[:])
			done <- input{b[0], err}
		}()
		select {
		case value := <-done:
			return value.b, value.err
		case <-ctx.Done():
			return 0, ctx.Err()
		case <-timeout:
			return 0, errors.New("model selection cancelled")
		}
	}
	selected := 0
	for {
		if _, err := fmt.Fprintf(a.Out, "\r\x1b[2K> %s", eligible[selected]); err != nil {
			return "", err
		}
		b, err := read(nil)
		if err != nil {
			return "", err
		}
		switch b {
		case 3, 4:
			return "", errors.New("model selection cancelled")
		case '\r', '\n':
			if _, err := fmt.Fprint(a.Out, "\r\n"); err != nil {
				return "", err
			}
			return eligible[selected], nil
		case 27:
			// A lone Escape cancels; CSI and application-mode arrows are accepted.
			timer := time.NewTimer(150 * time.Millisecond)
			prefix, err := read(timer.C)
			if err != nil {
				timer.Stop()
				return "", err
			}
			if prefix != '[' && prefix != 'O' {
				timer.Stop()
				return "", errors.New("model selection cancelled")
			}
			key, err := read(timer.C)
			timer.Stop()
			if err != nil {
				return "", err
			}
			switch key {
			case 'A':
				selected = (selected + len(eligible) - 1) % len(eligible)
			case 'B':
				selected = (selected + 1) % len(eligible)
			}
		}
	}
}
