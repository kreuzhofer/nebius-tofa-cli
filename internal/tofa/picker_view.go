package tofa

import (
	"fmt"
	"strings"
)

// pickerView contains presentation state only; launch eligibility remains in the
// selection policy and is checked again by the launcher after confirmation.
type pickerView struct {
	choices       []modelChoice
	query         string
	unavailable   bool
	cursor, first int
	details       bool
	detailOffset  int
	notice        string
}

func (p *pickerView) matches() []modelChoice {
	result := []modelChoice{}
	for _, choice := range p.choices {
		if (choice.disabled != "") != p.unavailable {
			continue
		}
		if strings.Contains(strings.ToLower(choice.identity+" "+choice.displayName()), strings.ToLower(p.query)) {
			result = append(result, choice)
		}
	}
	return result
}

func (c modelChoice) displayName() string {
	metadata, _ := metadataFor(c.identity)
	if metadata.DisplayName != "" {
		return metadata.DisplayName
	}
	_, name, found := strings.Cut(c.identity, "/")
	if found {
		return name
	}
	return c.identity
}

func (p *pickerView) move(amount int) {
	p.notice = ""
	if p.details {
		p.detailOffset = max(0, p.detailOffset+amount)
		return
	}
	if count := len(p.matches()); count > 0 {
		p.cursor = (p.cursor + amount%count + count) % count
	}
}

func (p *pickerView) resetFilter() { p.cursor, p.first, p.detailOffset = 0, 0, 0; p.notice = "" }

func clipPickerText(value string, width int) string {
	runes := []rune(value)
	if len(runes) <= width {
		return value
	}
	if width < 4 {
		return string(runes[:max(0, width)])
	}
	return string(runes[:width-3]) + "..."
}

func wrapPickerText(value string, width int) []string {
	var lines []string
	for _, paragraph := range strings.Split(value, "\n") {
		runes := []rune(paragraph)
		for len(runes) > width {
			end := width
			// Prefer word boundaries for prose, but retain every character of IDs.
			if space := strings.LastIndex(string(runes[:width]), " "); space > width/2 {
				end = space + 1
			}
			lines = append(lines, string(runes[:end]))
			runes = runes[end:]
		}
		lines = append(lines, string(runes))
	}
	return lines
}

func choiceDetails(choice modelChoice, width int) []string {
	text := choice.identity + "\n"
	switch {
	case choice.disabled != "":
		text += "Unavailable: " + choice.disabled
	case choice.status == "supported":
		text += "Supported for this main model, Guardian and route."
	default:
		text += "Experimental combination. Compatibility has not been verified."
	}
	return wrapPickerText(text, width)
}

// Frame returns physical terminal rows, excluding the cursor row. All text is
// bounded before styling; ANSI sequences do not contribute to display width.
func (p *pickerView) frame(width, height int, route, guardian string, color bool) []string {
	contentWidth := min(width-4, 88)
	style := func(text, code string) string {
		if !color {
			return text
		}
		return "\x1b[" + code + "m" + text + "\x1b[0m"
	}
	line := func(text string) string { return "  " + clipPickerText(text, contentWidth) }
	title := "Choose a main model"
	if p.unavailable {
		title = "Inspect unavailable models"
	}
	if p.details {
		title = "Model details"
	}
	lines := []string{style(line("tofa  /  Token Factory"), "36"), style(line(title), "1"), line("Codex CLI  /  " + route), ""}
	ready := 0
	for _, c := range p.choices {
		if c.disabled == "" {
			ready++
		}
	}
	tabs := fmt.Sprintf("[ Ready %d ]   Unavailable %d", ready, len(p.choices)-ready)
	if p.unavailable {
		tabs = fmt.Sprintf("Ready %d   [ Unavailable %d ]", ready, len(p.choices)-ready)
	}
	lines = append(lines, style(line(tabs), "1"), line("Filter: "+p.query), "")
	matches := p.matches()
	if p.cursor >= len(matches) {
		p.cursor = max(0, len(matches)-1)
	}
	// Seven header rows, eight lower-panel rows and the final cursor row leave
	// a useful list even in an 18-row terminal. Larger terminals show at most eight choices at once.
	listHeight := min(8, height-16)
	detail := []string{"Type a model name or provider to narrow the list."}
	if len(matches) > 0 {
		detail = choiceDetails(matches[p.cursor], contentWidth)
	}
	if p.details {
		bodyHeight := height - 11
		p.detailOffset = min(p.detailOffset, max(0, len(detail)-bodyHeight))
		for i := 0; i < bodyHeight; i++ {
			text := ""
			if p.detailOffset+i < len(detail) {
				text = detail[p.detailOffset+i]
			}
			lines = append(lines, line(text))
		}
		lines = append(lines, style(line("Up/Down scroll  |  ? back  |  Esc cancels"), "2"))
		return lines
	}
	if p.cursor < p.first {
		p.first = p.cursor
	}
	if p.cursor >= p.first+listHeight {
		p.first = p.cursor - listHeight + 1
	}
	for i := 0; i < listHeight; i++ {
		index := p.first + i
		if index >= len(matches) {
			text := ""
			if i == 0 {
				text = "No models match. Backspace edits; Ctrl-U clears."
			}
			lines = append(lines, style(line(text), "2"))
			continue
		}
		choice := matches[index]
		badge := "Experimental"
		if choice.status == "supported" {
			badge = "Supported"
		}
		if choice.disabled != "" {
			badge = "Unavailable"
		}
		nameWidth := contentWidth - len(badge) - 5
		prefix := "  "
		if index == p.cursor {
			prefix = "> "
		}
		name := clipPickerText(choice.displayName(), nameWidth)
		row := prefix + name + strings.Repeat(" ", max(1, contentWidth-len([]rune(name))-len(badge)-2)) + badge
		if index == p.cursor {
			lines = append(lines, style(line(row), "1;7"))
		} else {
			lines = append(lines, line(row))
		}
	}
	count := fmt.Sprintf("%d matches", len(matches))
	if len(matches) == 1 {
		count = "1 match"
	}
	if len(matches) > listHeight {
		count += fmt.Sprintf("  /  %d-%d shown", p.first+1, min(p.first+listHeight, len(matches)))
	}
	lines = append(lines, style(line(count), "2"), style(line(strings.Repeat("─", contentWidth)), "2"))
	for i := 0; i < 2; i++ {
		text := ""
		if i < len(detail) {
			text = detail[i]
		}
		if i == 1 && len(detail) > 2 {
			text = clipPickerText(text, contentWidth-15) + "  (? details)"
		}
		lines = append(lines, line(text))
	}
	if p.notice != "" {
		lines = append(lines, style(line(p.notice), "33"))
	} else {
		reviewer := guardian
		if reviewer == "" {
			reviewer = "native reviewer"
		}
		lines = append(lines, style(line("Guardian: "+reviewer), "2"))
	}
	navigation, shortcuts := "Up/Down choose | Enter confirms | Esc cancels", "Type to filter | Tab unavailable | ? details"
	if p.unavailable {
		navigation, shortcuts = "Up/Down inspect | Tab ready models | Esc cancels", "Type to filter | ? full details"
	}
	if contentWidth < 48 {
		navigation, shortcuts = "Up/Down  Enter launch  Esc cancel", "Type filter  Tab views  ? details"
		if p.unavailable {
			navigation = "Up/Down inspect  Tab ready  Esc exit"
		}
	}
	lines = append(lines, "", style(line(navigation), "2"), style(line(shortcuts), "2"))
	return lines
}
