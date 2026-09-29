package tofa

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/signal"
	"runtime"
	"strings"
	"syscall"
	"time"

	"golang.org/x/term"
)

func (a *App) pickTargetClient(ctx context.Context) (string, error) {
	choices := []pickerChoice{
		{identity: "codex", name: "Codex CLI", description: "Terminal coding client. Experimental models need confirmation."},
		{identity: "codex-desktop", name: "Codex desktop", description: "Desktop app. DeepSeek and GLM 5.3 have supported model pairs."},
	}
	if runtime.GOOS != "darwin" || runtime.GOARCH != "arm64" {
		choices[1].disabled = "requires the tested macOS arm64 desktop application"
	}
	return a.pick(ctx, pickerView{choices: choices, apps: true}, "", "", "")
}

func (a *App) pickMainModel(ctx context.Context, choices []pickerChoice, targetName, route, guardian string, allowUnverified bool) (string, error) {
	return a.pick(ctx, pickerView{choices: choices, confirmExperimental: !allowUnverified}, targetName, route, guardian)
}

// Share terminal input and restoration between the app and model stages.
// Read one byte on demand so confirmation leaves later-stage input untouched.
func (a *App) pick(ctx context.Context, view pickerView, targetName, route, guardian string) (identity string, result error) {
	choices := view.choices
	noun := "model"
	if view.apps {
		noun = "app"
	}

	eligible := []int{}
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
	restoreOutput, err := preparePickerOutput()
	if err != nil {
		return "", fmt.Errorf("cannot prepare %s picker output: %w", noun, err)
	}
	defer func() { result = errors.Join(result, restoreOutput()) }()
	fd := int(os.Stdin.Fd())
	state, err := term.MakeRaw(fd)
	if err != nil {
		return "", fmt.Errorf("cannot prepare %s picker terminal: %w", noun, err)
	}
	defer func() {
		_, cursorErr := fmt.Fprint(a.Out, "\x1b[0m\x1b[?25h\x1b[?1049l")
		result = errors.Join(result, term.Restore(fd, state), cursorErr)
	}()
	if _, err := fmt.Fprint(a.Out, "\x1b[?1049h\x1b[?25l"); err != nil {
		return "", err
	}
	type input struct {
		b   byte
		err error
	}
	var pending chan input
	var width, height int
	resize := time.NewTicker(150 * time.Millisecond)
	defer resize.Stop()
	resized := errors.New("picker resized")
	read := func(timeout <-chan time.Time) (byte, error) {
		if pending == nil {
			pending = make(chan input, 1)
			done := pending
			go func() {
				var b [1]byte
				_, err := os.Stdin.Read(b[:])
				done <- input{b[0], err}
			}()
		}
		for {
			select {
			case value := <-pending:
				pending = nil
				return value.b, value.err
			case <-ctx.Done():
				return 0, ctx.Err()
			case <-timeout:
				return 0, fmt.Errorf("%s selection cancelled", noun)
			case <-resize.C:
				if timeout == nil {
					w, h, err := term.GetSize(int(os.Stdout.Fd()))
					if err != nil || w != width || h != height {
						return 0, resized
					}
				}
			}
		}
	}
	color := os.Getenv("NO_COLOR") == "" && os.Getenv("TERM") != "dumb"
	for {
		width, height, err = term.GetSize(int(os.Stdout.Fd()))
		if err != nil || width < 40 || height < 18 {
			if view.apps {
				return "", errors.New("app picker requires a terminal at least 40 columns wide and 18 rows tall; enlarge it or use launch codex / launch codex-desktop")
			}
			return "", errors.New("model picker requires a terminal at least 40 columns wide and 18 rows tall; enlarge it or supply --model ID")
		}
		lines := view.frame(width, height, targetName, route, guardian, color)
		var frame strings.Builder
		frame.WriteString("\x1b[H\r\x1b[2K\x1b[J")
		frame.WriteString(strings.Join(lines, "\r\n") + "\r\n")
		if _, err := fmt.Fprint(a.Out, frame.String()); err != nil {
			return "", err
		}
		b, err := read(nil)
		if errors.Is(err, resized) {
			continue
		}
		if err != nil {
			return "", err
		}
		if view.confirming != nil {
			switch b {
			case 'y', 'Y':
				return view.confirming.identity, nil
			case 'n', 'N', '\r', '\n':
				view.confirming = nil
				continue
			case 3, 4, 27:
				return "", fmt.Errorf("%s selection cancelled", noun)
			default:
				continue
			}
		}
		switch b {
		case 3, 4:
			return "", fmt.Errorf("%s selection cancelled", noun)
		case '\r', '\n':
			matches := view.matches()
			if len(matches) == 0 {
				view.notice = "Choose a matching " + noun + " before launching."
				continue
			}
			if matches[view.cursor].disabled != "" {
				view.notice = "This " + noun + " cannot be launched. Tab returns to ready choices."
				continue
			}
			selected := matches[view.cursor]
			if view.confirmExperimental && selected.status != "supported" {
				view.confirming = &selected
				continue
			}
			return selected.identity, nil
		case '\t':
			view.unavailable = !view.unavailable
			view.details = false
			view.resetFilter()
		case '?':
			view.details = !view.details
			view.detailOffset = 0
		case 8, 127:
			if view.details {
				continue
			}
			if len(view.query) > 0 {
				view.query = view.query[:len(view.query)-1]
			}
			view.resetFilter()
		case 21:
			if view.details {
				continue
			}
			view.query = ""
			view.resetFilter()
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
				return "", fmt.Errorf("%s selection cancelled", noun)
			}
			var sequence strings.Builder
			for sequence.Len() < 16 {
				key, err := read(timer.C)
				if err != nil {
					timer.Stop()
					return "", err
				}
				sequence.WriteByte(key)
				if key >= 0x40 && key <= 0x7e {
					break
				}
			}
			timer.Stop()
			page := min(8, height-16)
			if view.details {
				page = height - 11
			}
			switch sequence.String() {
			case "A":
				view.move(-1)
			case "B":
				view.move(1)
			case "5~":
				view.move(-page)
			case "6~":
				view.move(page)
			case "H", "1~", "7~":
				if view.details {
					view.detailOffset = 0
				} else {
					view.cursor = 0
				}
			case "F", "4~", "8~":
				if view.details {
					view.detailOffset = 1024
				} else {
					view.cursor = max(0, len(view.matches())-1)
				}
			}

		default:
			if b >= 32 && b <= 126 && len(view.query) < 512 && !view.details {
				view.query += string(b)
				view.resetFilter()
			}
		}
	}
}
