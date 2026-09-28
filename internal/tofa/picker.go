package tofa

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"golang.org/x/term"
)

// Redraw the visible list with a leading indicator beside the selected model.
// Read exactly one byte on demand so confirming never consumes client input.
func (a *App) pickMainModel(ctx context.Context, choices []modelChoice) (identity string, result error) {
	eligible := []int{}
	if _, err := fmt.Fprintln(a.Out, "Target client: Codex CLI\nAvailable main models:"); err != nil {
		return "", err
	}
	for i, choice := range choices {
		if choice.disabled == "" {
			eligible = append(eligible, i)
		}
	}
	if len(eligible) == 0 {
		for _, choice := range choices {
			if _, err := fmt.Fprintf(a.Out, "  %s [disabled: %s]\n", choice.identity, choice.disabled); err != nil {
				return "", err
			}
		}
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
		_, cursorErr := fmt.Fprint(a.Out, "\x1b[?25h")
		result = errors.Join(result, term.Restore(fd, state), cursorErr)
	}()
	if _, err := fmt.Fprint(a.Out, "\x1b[?25l"); err != nil {
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
	selected, first, rendered := 0, 0, 0
	for {
		width, height, err := term.GetSize(int(os.Stdout.Fd()))
		if err != nil || width < 10 || height < 6 {
			return "", errors.New("model picker requires a terminal at least 10 columns wide and 6 rows tall")
		}
		// Leave space for the heading, navigation help and cursor. Each displayed
		// entry occupies one physical line so cursor movement survives long IDs.
		pageSize := height - 5
		row := eligible[selected]
		if row < first {
			first = row
		}
		if row >= first+pageSize {
			first = row - pageSize + 1
		}
		last := min(first+pageSize, len(choices))
		var frame strings.Builder
		if rendered > 0 {
			fmt.Fprintf(&frame, "\x1b[%dA", rendered)
		}
		frame.WriteString("\r\x1b[2K\x1b[J")
		line := func(value string) {
			if len(value) >= width {
				value = value[:width-4] + "..."
			}
			frame.WriteString(value + "\r\n")
		}
		for i := first; i < last; i++ {
			choice := choices[i]
			indicator, label := "  ", choice.status
			if i == row {
				indicator = "> "
			}
			if choice.disabled != "" {
				label = "disabled: " + choice.disabled
			}
			line(indicator + choice.identity + " [" + label + "]")
		}
		line("L: full catalog and disabled reasons.")
		help := "Up/Down selects; Enter confirms; Escape/Ctrl-C cancels."
		if len(choices) > pageSize {
			help += fmt.Sprintf(" (%d-%d of %d)", first+1, last, len(choices))
		}
		line(help)
		if _, err := fmt.Fprint(a.Out, frame.String()); err != nil {
			return "", err
		}
		rendered = last - first + 2
		b, err := read(nil)
		if err != nil {
			return "", err
		}
		switch b {
		case 'l', 'L':
			// Preserve an unabridged catalog in scrollback, including disabled
			// rows beyond the viewport and explanations clipped by its width.
			for _, choice := range choices {
				label := choice.status
				if choice.disabled != "" {
					label = "disabled: " + choice.disabled
				}
				if _, err := fmt.Fprintf(a.Out, "  %s [%s]\r\n", choice.identity, label); err != nil {
					return "", err
				}
			}
			rendered = 0
		case 3, 4:
			return "", errors.New("model selection cancelled")
		case '\r', '\n':
			return choices[eligible[selected]].identity, nil
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
