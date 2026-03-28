package tui

import (
	"fmt"
	"io"
	"strings"

	"github.com/charmbracelet/bubbles/list"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type entryDelegate struct{}

func (d entryDelegate) Height() int                             { return 1 }
func (d entryDelegate) Spacing() int                            { return 0 }
func (d entryDelegate) Update(_ tea.Msg, _ *list.Model) tea.Cmd { return nil }

func (d entryDelegate) Render(w io.Writer, m list.Model, index int, item list.Item) {
	ei, ok := item.(EntryItem)
	if !ok {
		return
	}

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

	fmt.Fprint(w, line)
}
