package tui

import (
	"fmt"
	"io"
	"strings"

	"github.com/charmbracelet/bubbles/list"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

const profileIndent = "    "

type profileDelegate struct{}

func (d profileDelegate) Height() int                             { return 3 }
func (d profileDelegate) Spacing() int                            { return 0 }
func (d profileDelegate) Update(_ tea.Msg, _ *list.Model) tea.Cmd { return nil }

func (d profileDelegate) Render(w io.Writer, m list.Model, index int, item list.Item) {
	p, ok := item.(ProfileItem)
	if !ok {
		return
	}

	nameStyle := lipgloss.NewStyle().Bold(true)
	marketStyle := lipgloss.NewStyle().Foreground(ColorMuted)
	descStyle := lipgloss.NewStyle().Foreground(ColorMuted)
	installedStyle := lipgloss.NewStyle().Foreground(ColorClean)
	outdatedStyle := lipgloss.NewStyle().Foreground(ColorModified)

	if index == m.Index() {
		nameStyle = nameStyle.Foreground(ColorSelected)
		marketStyle = marketStyle.Foreground(ColorActive)
	}

	cursor := "  "
	if index == m.Index() {
		cursor = "> "
	}

	line := cursor + nameStyle.Render(p.Name) + marketStyle.Render("@"+p.Market)

	// Status tags on a new indented line
	if p.HasInstalled || p.HasOutdated {
		var tags []string
		if p.HasInstalled {
			tags = append(tags, installedStyle.Render("installed"))
		}
		if p.HasOutdated {
			tags = append(tags, outdatedStyle.Render("outdated"))
		}
		line += "\n" + profileIndent + strings.Join(tags, " ")
	}

	if p.Desc != "" {
		maxW := m.Width() - len(profileIndent)
		if maxW < 10 {
			maxW = 10
		}
		// Limit description to 1 line to keep delegate compact
		wrapped := wordWrap(p.Desc, maxW)
		wlines := strings.SplitN(wrapped, "\n", 2)
		if len(wlines) > 1 {
			wlines[0] = truncate(wlines[0], maxW-3) + "..."
			wlines = wlines[:1]
		}
		for _, wl := range wlines {
			line += "\n" + profileIndent + descStyle.Render(wl)
		}
	} else {
		line += "\n"
	}

	fmt.Fprint(w, line)
}

func truncate(s string, maxLen int) string {
	if maxLen < 0 {
		maxLen = 0
	}
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen]
}

func wordWrap(s string, width int) string {
	words := strings.Fields(s)
	if len(words) == 0 {
		return ""
	}
	var lines []string
	cur := words[0]
	for _, word := range words[1:] {
		if len(cur)+1+len(word) > width {
			lines = append(lines, cur)
			cur = word
		} else {
			cur += " " + word
		}
	}
	lines = append(lines, cur)
	return strings.Join(lines, "\n")
}
