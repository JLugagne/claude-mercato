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

type entryDelegate struct{}

func (d entryDelegate) Height() int                             { return 1 }
func (d entryDelegate) Spacing() int                            { return 0 }
func (d entryDelegate) Update(_ tea.Msg, _ *list.Model) tea.Cmd { return nil }

func (d entryDelegate) Render(w io.Writer, m list.Model, index int, item list.Item) {
	switch v := item.(type) {
	case EntryItem:
		d.renderEntry(w, m, index, v)
	case SkillFileItem:
		d.renderSkillFile(w, m, index, v)
	}
}

func (d entryDelegate) renderEntry(w io.Writer, m list.Model, index int, ei EntryItem) {
	name := strings.TrimSuffix(ei.Entry.Filename, ".md")

	nameStyle := lipgloss.NewStyle()
	typeStyle := lipgloss.NewStyle().Foreground(ColorMuted)

	if index == m.Index() {
		nameStyle = nameStyle.Foreground(ColorSelected).Bold(true)
		typeStyle = typeStyle.Foreground(ColorActive)
	}

	cursor := "  "
	if index == m.Index() {
		cursor = "> "
	}

	typeLabel := string(ei.Entry.Type)
	line := cursor + typeStyle.Render(typeLabel) + " " + nameStyle.Render(name)
	line = ansi.Truncate(line, m.Width(), "")

	_, _ = fmt.Fprint(w, line)
}

func (d entryDelegate) renderSkillFile(w io.Writer, m list.Model, index int, sf SkillFileItem) {
	nameStyle := lipgloss.NewStyle()
	muted := lipgloss.NewStyle().Foreground(ColorMuted)

	if index == m.Index() {
		nameStyle = nameStyle.Foreground(ColorSelected).Bold(true)
	}

	cursor := "  "
	if index == m.Index() {
		cursor = "> "
	}

	name := sf.File.Name
	isMd := strings.HasSuffix(name, ".md")

	var line string
	if isMd {
		line = cursor + nameStyle.Render(name)
	} else {
		line = cursor + muted.Render(name)
	}
	line = ansi.Truncate(line, m.Width(), "")

	_, _ = fmt.Fprint(w, line)
}
