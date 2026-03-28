package tui

import "github.com/charmbracelet/lipgloss"

var (
	ColorClean    = lipgloss.AdaptiveColor{Light: "#22863a", Dark: "#3fb950"}
	ColorUpdate   = lipgloss.AdaptiveColor{Light: "#0366d6", Dark: "#58a6ff"}
	ColorModified = lipgloss.AdaptiveColor{Light: "#b08800", Dark: "#e3b341"}
	ColorDanger   = lipgloss.AdaptiveColor{Light: "#cb2431", Dark: "#f85149"}
	ColorDeleted  = lipgloss.AdaptiveColor{Light: "#6a737d", Dark: "#484f58"}
	ColorSelected = lipgloss.AdaptiveColor{Light: "#6f42c1", Dark: "#bc8cff"}
	ColorMuted    = lipgloss.AdaptiveColor{Light: "#6a737d", Dark: "#8b949e"}
	ColorBorder   = lipgloss.AdaptiveColor{Light: "#d0d7de", Dark: "#30363d"}
	ColorActive   = lipgloss.AdaptiveColor{Light: "#0366d6", Dark: "#58a6ff"}

	StyleTitle = lipgloss.NewStyle().Bold(true).Padding(0, 1)
	StyleBorder = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(ColorBorder)
	StyleActiveBorder = lipgloss.NewStyle().
				Border(lipgloss.RoundedBorder()).
				BorderForeground(ColorActive)
	StyleStatusBar = lipgloss.NewStyle().
			Padding(0, 1).
			Background(lipgloss.AdaptiveColor{Light: "#e1e4e8", Dark: "#21262d"})
	StyleIndicatorClean    = lipgloss.NewStyle().Foreground(ColorClean)
	StyleIndicatorUpdate   = lipgloss.NewStyle().Foreground(ColorUpdate)
	StyleIndicatorModified = lipgloss.NewStyle().Foreground(ColorModified)
	StyleIndicatorDanger   = lipgloss.NewStyle().Foreground(ColorDanger)
	StyleIndicatorDeleted  = lipgloss.NewStyle().Foreground(ColorDeleted)
	StyleTag               = lipgloss.NewStyle().
				Padding(0, 1).
				Foreground(lipgloss.AdaptiveColor{Light: "#6f42c1", Dark: "#bc8cff"}).
				Background(lipgloss.AdaptiveColor{Light: "#f5f0ff", Dark: "#2d1f4e"})
)
