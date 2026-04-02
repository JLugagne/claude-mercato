package tui

import (
	"fmt"
	"io"
	"strings"

	"github.com/charmbracelet/bubbles/list"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
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

	width := m.Width()
	if width < 1 {
		width = 20
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

	// Line 1: cursor + name@market (truncated to width)
	line1 := cursor + nameStyle.Render(p.Name) + marketStyle.Render("@"+p.Market)
	line1 = ansi.Truncate(line1, width, "")

	// Line 2: status tags or description
	var line2 string
	if p.HasInstalled || p.HasOutdated {
		var tags []string
		if p.HasInstalled {
			tags = append(tags, installedStyle.Render("installed"))
		}
		if p.HasOutdated {
			tags = append(tags, outdatedStyle.Render("outdated"))
		}
		line2 = profileIndent + strings.Join(tags, " ")
	}
	if line2 != "" {
		line2 = ansi.Truncate(line2, width, "")
	}

	// Line 3: description (only if line 2 was used for tags, otherwise desc goes on line 2)
	var line3 string
	if p.Desc != "" {
		maxW := width - len(profileIndent)
		if maxW < 10 {
			maxW = 10
		}
		desc := truncateStr(p.Desc, maxW)
		if line2 == "" {
			line2 = profileIndent + descStyle.Render(desc)
		} else {
			line3 = profileIndent + descStyle.Render(desc)
		}
	}

	_, _ = fmt.Fprint(w, line1+"\n"+line2+"\n"+line3)
}

func truncateStr(s string, maxLen int) string {
	if maxLen < 0 {
		maxLen = 0
	}
	if len(s) <= maxLen {
		return s
	}
	if maxLen > 3 {
		return s[:maxLen-3] + "..."
	}
	return s[:maxLen]
}

