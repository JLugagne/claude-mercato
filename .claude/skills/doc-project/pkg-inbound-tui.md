# inbound/queries/tui — Terminal User Interface

**Location**: `internal/mercato/inbound/queries/tui/`

## Purpose

Interactive terminal UI built with the Charmbracelet stack (Bubble Tea, Bubbles, Glamour, Lipgloss). Provides a browsable, searchable interface to market entries.

## Architecture

Follows the Elm architecture (Bubble Tea pattern): Model → Update → View.

## Main Model (`AppModel`)

### Modes
- **Loading** — Initial data fetch
- **Normal** — Standard browsing
- **Help** — Help overlay
- **Search** — Search input active
- **CommandMode** — `:` command input

### Focus Areas
- **Profiles** — Left panel, profile list
- **Entries** — Center panel, entry list within selected profile
- **Detail** — Right panel, entry metadata
- **Content** — Right panel, full file content / skill directory browser

## Files

| File | Key Types | Description |
|------|-----------|-------------|
| `app.go` | `AppModel`, `TUIServices` | Main state machine, Init/Update/View cycle |
| `entry_delegate.go` | — | Custom renderer for entry list items |
| `profile_delegate.go` | — | Custom renderer for profile list items |
| `market_popup.go` | — | Confirmation dialog for cross-market dependency actions |
| `messages.go` | Tea message types | Async operation results (load complete, search results, etc.) |
| `styles.go` | Lipgloss styles | Color schemes, borders, padding |
| `types.go` | `EntryItem`, `ProfileItem`, `MarketPopupAction` | List item models and action enums |

## TUIServices

```go
type TUIServices struct {
    EntryQueries
    SearchQueries
    ReadmeQueries
    MarketQueries
}
```

## Features

- Three-panel layout (profiles / entries / detail)
- Full-text search with real-time results
- Skill directory file browser
- Markdown rendering via Glamour
- Cross-market dependency confirmation popup
- Command mode (`:` prefix) for inline commands
