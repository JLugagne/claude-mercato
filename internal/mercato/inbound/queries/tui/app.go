package tui

import (
	"fmt"
	"sort"
	"strings"

	"github.com/JLugagne/claude-mercato/internal/mercato/domain"
	"github.com/JLugagne/claude-mercato/internal/mercato/domain/service"
	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type AppModel struct {
	svc    TUIServices
	mode   Mode
	focus  Focus
	width  int
	height int

	profilesList list.Model
	detailView   viewport.Model
	entriesList  list.Model
	contentView  viewport.Model

	searchInput textinput.Model
	cmdInput    textinput.Model
	spinner     spinner.Model

	allEntries      []EntryItem
	filteredEntries []EntryItem
	allProfiles     []ProfileItem
	searchQuery     string

	selectedEntryContent string
	selectedEntryRef     domain.MctRef
	statusByRef          map[domain.MctRef]domain.EntryState

	marketPopup MarketPopup

	profileAction       string // "install" or "remove"
	profileActionTarget ProfileItem

	loading      bool
	loadingPhase string
	statusMsg    string
}

func NewAppModel(svc TUIServices) AppModel {
	sp := spinner.New()
	sp.Spinner = spinner.Dot

	si := textinput.New()
	si.Placeholder = "Search..."
	si.CharLimit = 256

	ci := textinput.New()
	ci.Placeholder = ":"
	ci.CharLimit = 256

	pl := list.New(nil, profileDelegate{}, 0, 0)
	pl.Title = "Profiles"
	pl.SetShowHelp(false)
	pl.SetShowStatusBar(false)
	pl.SetShowPagination(false)
	pl.InfiniteScrolling = true

	el := list.New(nil, entryDelegate{}, 0, 0)
	el.Title = "Agents & Skills"
	el.SetShowHelp(false)
	el.SetShowStatusBar(false)
	el.SetShowPagination(false)
	el.InfiniteScrolling = true

	vp := viewport.New(0, 0)
	cv := viewport.New(0, 0)

	return AppModel{
		svc:          svc,
		mode:         ModeLoading,
		focus:        FocusProfiles,
		profilesList: pl,
		detailView:   vp,
		entriesList:  el,
		contentView:  cv,
		searchInput:  si,
		cmdInput:     ci,
		spinner:      sp,
		marketPopup:  newMarketPopup(),
		loading:      true,
		loadingPhase: "Loading markets...",
	}
}

func (m AppModel) Init() tea.Cmd {
	return tea.Batch(
		m.spinner.Tick,
		m.loadAllMarkets(),
	)
}

func (m AppModel) loadAllMarkets() tea.Cmd {
	return func() tea.Msg {
		entries, err := m.svc.Search.DumpIndex()
		if err != nil {
			return IndexReadyMsg{Entries: nil, Elapsed: 0}
		}
		statuses, _ := m.svc.Check.Check(service.CheckOpts{})
		return IndexReadyMsg{Entries: entries, Statuses: statuses, Elapsed: 0}
	}
}

func (m AppModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tea.KeyMsg:
		return m.handleKey(msg)

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.updateLayout()
		return m, nil

	case spinner.TickMsg:
		var cmd tea.Cmd
		m.spinner, cmd = m.spinner.Update(msg)
		return m, cmd

	case IndexReadyMsg:
		m.loading = false
		m.mode = ModeNormal
		m.statusByRef = make(map[domain.MctRef]domain.EntryState)
		for _, s := range msg.Statuses {
			m.statusByRef[s.Ref] = s.State
		}
		m.allEntries = make([]EntryItem, len(msg.Entries))
		for i, e := range msg.Entries {
			m.allEntries[i] = EntryItem{Entry: e}
		}
		// Apply market selection filter if any markets are deselected.
		if len(m.marketPopup.selected) > 0 {
			filtered := make([]EntryItem, 0, len(m.allEntries))
			for _, ei := range m.allEntries {
				if m.marketPopup.selected[ei.Entry.Market] {
					filtered = append(filtered, ei)
				}
			}
			m.filteredEntries = filtered
		} else {
			m.filteredEntries = m.allEntries
		}
		m.updateProfilesList()
		m.updateEntriesList()
		m.updateDetailContent()
		return m, nil

	case SearchResultMsg:
		m.filteredEntries = make([]EntryItem, len(msg.Results))
		for i, r := range msg.Results {
			m.filteredEntries[i] = EntryItem{Entry: r.Entry}
		}
		m.updateProfilesList()
		m.updateEntriesList()
		m.updateDetailContent()
		return m, nil

	case EntryContentMsg:
		if msg.Err != nil {
			m.selectedEntryContent = "Error loading content: " + msg.Err.Error()
		} else {
			m.selectedEntryContent = msg.Content
		}
		m.selectedEntryRef = msg.Ref
		w := m.contentView.Width
		if w < 1 {
			w = 40
		}
		wrapped := lipgloss.NewStyle().Width(w).Render(m.selectedEntryContent)
		m.contentView.SetContent(wrapped)
		m.contentView.GotoTop()
		return m, nil

	case FetchCompleteMsg:
		return m, m.loadAllMarkets()

	case InstallCompleteMsg, UpdateCompleteMsg, PruneCompleteMsg:
		return m, m.loadAllMarkets()

	case ProfileInstallMsg:
		m.mode = ModeNormal
		if len(msg.Errors) > 0 {
			m.statusMsg = fmt.Sprintf("Installed profile %s with %d error(s)", msg.Profile, len(msg.Errors))
		} else {
			m.statusMsg = fmt.Sprintf("Installed profile %s", msg.Profile)
		}
		return m, m.loadAllMarkets()

	case ProfileRemoveMsg:
		m.mode = ModeNormal
		if len(msg.Errors) > 0 {
			m.statusMsg = fmt.Sprintf("Removed profile %s with %d error(s)", msg.Profile, len(msg.Errors))
		} else {
			m.statusMsg = fmt.Sprintf("Removed profile %s", msg.Profile)
		}
		return m, m.loadAllMarkets()

	case DiffCompleteMsg:
		m.mode = ModeNormal
		return m, m.loadAllMarkets()

	case MarketAddedMsg:
		if msg.Err != nil {
			m.marketPopup.errMsg = msg.Err.Error()
			return m, nil
		}
		markets, _ := m.svc.Markets.ListMarkets()
		m.marketPopup.load(markets)
		return m, m.loadAllMarkets()

	case MarketRemovedMsg:
		if msg.Err != nil {
			m.marketPopup.errMsg = msg.Err.Error()
			return m, nil
		}
		markets, _ := m.svc.Markets.ListMarkets()
		m.marketPopup.load(markets)
		return m, m.loadAllMarkets()

	case MarketRenamedMsg:
		if msg.Err != nil {
			m.marketPopup.errMsg = msg.Err.Error()
			return m, nil
		}
		markets, _ := m.svc.Markets.ListMarkets()
		m.marketPopup.load(markets)
		return m, m.loadAllMarkets()
	}

	switch m.focus {
	case FocusProfiles:
		var cmd tea.Cmd
		m.profilesList, cmd = m.profilesList.Update(msg)
		cmds = append(cmds, cmd)
	case FocusDetail:
		var cmd tea.Cmd
		m.detailView, cmd = m.detailView.Update(msg)
		cmds = append(cmds, cmd)
	case FocusEntries:
		var cmd tea.Cmd
		m.entriesList, cmd = m.entriesList.Update(msg)
		cmds = append(cmds, cmd)
	case FocusContent:
		var cmd tea.Cmd
		m.contentView, cmd = m.contentView.Update(msg)
		cmds = append(cmds, cmd)
	}

	return m, tea.Batch(cmds...)
}

func (m *AppModel) handleKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "ctrl+c":
		return m, tea.Quit
	case "q":
		if m.mode == ModeNormal {
			return m, tea.Quit
		}
	}

	switch m.mode {
	case ModeSearch:
		return m.handleSearchKey(msg)
	case ModeHelp:
		if msg.String() == "?" || msg.String() == "esc" || msg.String() == "q" {
			m.mode = ModeNormal
			return m, nil
		}
	case ModeMarketPopup:
		return m.handleMarketPopupKey(msg)
	case ModeProfileAction:
		return m.handleProfileActionKey(msg)
	case ModeNormal:
		return m.handleNormalKey(msg)
	}

	return m, nil
}

func (m *AppModel) handleNormalKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	m.statusMsg = ""
	switch msg.String() {
	case "/":
		m.mode = ModeSearch
		m.searchInput.Focus()
		return m, textinput.Blink
	case "?":
		m.mode = ModeHelp
		return m, nil
	case "tab":
		maxFocus := Focus(2) // profiles, detail
		if m.threeCol() {
			maxFocus = 4 // profiles, detail, entries, content
		}
		m.focus = (m.focus + 1) % maxFocus
		return m, nil
	case "shift+tab":
		maxFocus := Focus(2)
		if m.threeCol() {
			maxFocus = 4
		}
		m.focus = (m.focus + maxFocus - 1) % maxFocus
		return m, nil
	case "h", "left":
		switch m.focus {
		case FocusDetail:
			m.focus = FocusProfiles
		case FocusEntries, FocusContent:
			m.focus = FocusDetail
		}
		return m, nil
	case "l", "right":
		switch m.focus {
		case FocusProfiles:
			m.focus = FocusDetail
		case FocusDetail:
			m.focus = FocusEntries
		}
		return m, nil
	case "r":
		return m, m.refreshCmd()
	case "R":
		return m, m.refreshAllCmd()
	case "m":
		m.mode = ModeMarketPopup
		markets, _ := m.svc.Markets.ListMarkets()
		m.marketPopup.load(markets)
		return m, nil
	case "i":
		item, ok := m.profilesList.SelectedItem().(ProfileItem)
		if !ok {
			return m, nil
		}
		m.profileAction = "install"
		m.profileActionTarget = item
		m.mode = ModeProfileAction
		m.statusMsg = ""
		return m, nil
	case "x":
		item, ok := m.profilesList.SelectedItem().(ProfileItem)
		if !ok || !item.HasInstalled {
			return m, nil
		}
		m.profileAction = "remove"
		m.profileActionTarget = item
		m.mode = ModeProfileAction
		m.statusMsg = ""
		return m, nil
	}

	// Delegate to the focused component for navigation keys
	var cmd tea.Cmd
	switch m.focus {
	case FocusProfiles:
		prevIdx := m.profilesList.Index()
		m.profilesList, cmd = m.profilesList.Update(msg)
		if m.profilesList.Index() != prevIdx {
			m.updateEntriesList()
			m.updateDetailContent()
			m.selectedEntryContent = ""
			m.contentView.SetContent("")
		}
		return m, cmd
	case FocusDetail:
		m.detailView, cmd = m.detailView.Update(msg)
	case FocusEntries:
		prevIdx := m.entriesList.Index()
		m.entriesList, cmd = m.entriesList.Update(msg)
		if m.entriesList.Index() != prevIdx {
			return m, tea.Batch(cmd, m.loadEntryContent())
		}
		// Handle enter to load content and focus content pane
		if msg.String() == "enter" {
			m.focus = FocusContent
			return m, m.loadEntryContent()
		}
	case FocusContent:
		m.contentView, cmd = m.contentView.Update(msg)
	}
	return m, cmd
}

func (m *AppModel) loadEntryContent() tea.Cmd {
	item, ok := m.entriesList.SelectedItem().(EntryItem)
	if !ok {
		return nil
	}
	entry := item.Entry
	return func() tea.Msg {
		content, err := m.svc.Content.ReadEntryContent(entry.Market, entry.RelPath)
		if err != nil {
			return EntryContentMsg{Ref: entry.Ref, Err: err}
		}
		return EntryContentMsg{Ref: entry.Ref, Content: string(content)}
	}
}

func (m *AppModel) handleSearchKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc":
		m.mode = ModeNormal
		m.searchInput.Blur()
		m.searchInput.Reset()
		m.filteredEntries = m.allEntries
		m.updateProfilesList()
		m.updateEntriesList()
		return m, nil
	case "enter":
		m.mode = ModeNormal
		m.searchInput.Blur()
		return m, nil
	}

	var cmd tea.Cmd
	m.searchInput, cmd = m.searchInput.Update(msg)
	m.searchQuery = m.searchInput.Value()

	if m.searchQuery != "" {
		return m, tea.Batch(cmd, m.searchCmd(m.searchQuery))
	}
	m.filteredEntries = m.allEntries
	m.updateProfilesList()
	m.updateEntriesList()
	return m, cmd
}

func (m *AppModel) refreshCmd() tea.Cmd {
	return func() tea.Msg {
		results, err := m.svc.Sync.Refresh(service.RefreshOpts{})
		if err != nil || len(results) == 0 {
			return FetchCompleteMsg{Err: err}
		}
		return FetchCompleteMsg{Market: results[0].Market, NewSHA: results[0].NewSHA}
	}
}

func (m *AppModel) refreshAllCmd() tea.Cmd {
	return func() tea.Msg {
		_, err := m.svc.Sync.Refresh(service.RefreshOpts{})
		return FetchCompleteMsg{Err: err}
	}
}

func (m *AppModel) installCmd(ref domain.MctRef) tea.Cmd {
	return func() tea.Msg {
		err := m.svc.Entries.Add(ref, service.AddOpts{})
		return InstallCompleteMsg{Ref: ref, Err: err}
	}
}

func (m *AppModel) updateCmd(ref domain.MctRef) tea.Cmd {
	return func() tea.Msg {
		results, err := m.svc.Sync.Update(service.UpdateOpts{Ref: ref})
		if err != nil || len(results) == 0 {
			return UpdateCompleteMsg{Ref: ref, Err: err}
		}
		return UpdateCompleteMsg{Ref: ref, NewVersion: results[0].NewVersion}
	}
}

func (m *AppModel) removeCmd(ref domain.MctRef) tea.Cmd {
	return func() tea.Msg {
		err := m.svc.Entries.Remove(ref)
		return PruneCompleteMsg{Ref: ref, Action: "removed", Err: err}
	}
}

func (m *AppModel) handleProfileActionKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc", "n":
		m.mode = ModeNormal
		return m, nil
	case "y", "enter":
		p := m.profileActionTarget
		m.mode = ModeLoading
		m.loading = true
		switch m.profileAction {
		case "install":
			m.loadingPhase = fmt.Sprintf("Installing %s...", p.Name)
			return m, m.profileInstallCmd(p)
		case "remove":
			m.loadingPhase = fmt.Sprintf("Removing %s...", p.Name)
			return m, m.profileRemoveCmd(p)
		}
		m.mode = ModeNormal
		return m, nil
	}
	return m, nil
}

func (m *AppModel) profileInstallCmd(p ProfileItem) tea.Cmd {
	return func() tea.Msg {
		var errs []error
		for _, e := range p.Entries {
			if e.Installed {
				continue
			}
			if err := m.svc.Entries.Add(e.Ref, service.AddOpts{}); err != nil {
				errs = append(errs, err)
			}
		}
		return ProfileInstallMsg{Profile: p.Name, Errors: errs}
	}
}

func (m *AppModel) profileRemoveCmd(p ProfileItem) tea.Cmd {
	return func() tea.Msg {
		var errs []error
		for _, e := range p.Entries {
			if !e.Installed {
				continue
			}
			if err := m.svc.Entries.Remove(e.Ref); err != nil {
				errs = append(errs, err)
			}
		}
		return ProfileRemoveMsg{Profile: p.Name, Errors: errs}
	}
}

func (m *AppModel) searchCmd(query string) tea.Cmd {
	markets := m.marketPopup.selectedMarkets()
	return func() tea.Msg {
		var results []service.SearchResult
		var err error
		if len(markets) == 0 {
			// No markets selected: return empty result set immediately.
			return SearchResultMsg{Query: query}
		}
		if len(markets) == 1 {
			results, err = m.svc.Search.Search(query, service.SearchOpts{Limit: 50, Market: markets[0]})
		} else {
			results, err = m.svc.Search.Search(query, service.SearchOpts{Limit: 50})
			if err == nil {
				marketSet := make(map[string]bool, len(markets))
				for _, mk := range markets {
					marketSet[mk] = true
				}
				filtered := results[:0]
				for _, r := range results {
					if marketSet[r.Entry.Market] {
						filtered = append(filtered, r)
					}
				}
				results = filtered
			}
		}
		if err != nil {
			return SearchResultMsg{Query: query}
		}
		return SearchResultMsg{Query: query, Results: results}
	}
}

// applyMarketFilterCmd re-applies the market selection filter to allEntries
// and re-runs any active search query.
func (m *AppModel) applyMarketFilterCmd() tea.Cmd {
	selected := m.marketPopup.selected
	filtered := make([]EntryItem, 0, len(m.allEntries))
	for _, ei := range m.allEntries {
		if selected[ei.Entry.Market] {
			filtered = append(filtered, ei)
		}
	}
	m.filteredEntries = filtered
	m.updateProfilesList()
	m.updateEntriesList()
	m.updateDetailContent()
	if m.searchQuery != "" {
		return m.searchCmd(m.searchQuery)
	}
	return nil
}

func (m *AppModel) updateProfilesList() {
	m.allProfiles = m.buildProfiles(m.filteredEntries)
	items := make([]list.Item, len(m.allProfiles))
	for i, p := range m.allProfiles {
		items[i] = p
	}
	m.profilesList.SetItems(items)
	// Force all items onto a single page so the list never paginates.
	if len(items) > m.profilesList.Paginator.PerPage {
		m.profilesList.Paginator.PerPage = len(items)
		m.profilesList.Paginator.SetTotalPages(1)
	}
}

func (m *AppModel) updateDetailContent() {
	m.detailView.SetContent(m.buildDetailContent())
	m.detailView.GotoTop()
}

func (m *AppModel) updateEntriesList() {
	item, ok := m.profilesList.SelectedItem().(ProfileItem)
	if !ok {
		m.entriesList.SetItems(nil)
		return
	}

	items := make([]list.Item, len(item.Entries))
	for i, e := range item.Entries {
		items[i] = EntryItem{Entry: e}
	}
	m.entriesList.SetItems(items)
}

func (m *AppModel) buildProfiles(entries []EntryItem) []ProfileItem {
	profileMap := make(map[string]*ProfileItem)
	var order []string
	for _, ei := range entries {
		e := ei.Entry
		key := e.Market + "/" + e.Category
		if _, ok := profileMap[key]; !ok {
			profileMap[key] = &ProfileItem{
				Name:   e.Category,
				Market: e.Market,
				Desc:   e.ProfileDescription,
				Readme: e.ReadmeContext,
				Tags:   e.MctTags,
			}
			order = append(order, key)
		}
		p := profileMap[key]
		p.Entries = append(p.Entries, e)
		if e.Installed {
			p.HasInstalled = true
		}
		if state, ok := m.statusByRef[e.Ref]; ok {
			if state == domain.StateUpdateAvailable || state == domain.StateUpdateAndDrift {
				p.HasOutdated = true
			}
		}
	}
	profiles := make([]ProfileItem, 0, len(order))
	for _, key := range order {
		profiles = append(profiles, *profileMap[key])
	}
	sort.Slice(profiles, func(i, j int) bool {
		return profiles[i].Name < profiles[j].Name
	})
	return profiles
}

func (m AppModel) threeCol() bool { return m.width > 100 }

func (m AppModel) colWidths() (w1, w2, w3 int) {
	if m.threeCol() {
		w1 = m.width * 2 / 10  // ~20% profiles
		w3 = m.width * 3 / 10  // ~30% right panel
		w2 = m.width - w1 - w3 // ~50% detail
		return
	}
	if m.width > 60 {
		w1 = m.width * 3 / 10
		w2 = m.width - w1
		return
	}
	w1 = m.width
	return
}

func (m *AppModel) updateLayout() {
	h := m.height - 4
	if h < 1 {
		h = 1
	}

	w1, w2, w3 := m.colWidths()

	m.profilesList.SetSize(w1-2, h)

	if w2 > 0 {
		m.detailView.Width = w2 - 2
		m.detailView.Height = h
	}

	if w3 > 0 {
		panelH := h + 2
		innerTotal := panelH - 4
		entriesH := innerTotal / 3
		contentH := innerTotal - entriesH
		m.entriesList.SetSize(w3-2, entriesH)
		m.contentView.Width = w3 - 4
		m.contentView.Height = contentH
	}
}

func (m AppModel) View() string {
	if m.loading {
		return m.viewLoading()
	}

	var panels []string
	h := m.height - 4
	w1, w2, w3 := m.colWidths()
	panelH := h + 2 // total outer height per column (inner h + 2 for border)

	// Left panel: profiles list
	{
		style := StyleBorder
		if m.focus == FocusProfiles {
			style = StyleActiveBorder
		}
		panels = append(panels, style.Width(w1-2).MaxWidth(w1).Height(h).MaxHeight(panelH).Render(m.profilesList.View()))
	}

	// Middle panel: detail (readme)
	if w2 > 0 {
		style := StyleBorder
		if m.focus == FocusDetail {
			style = StyleActiveBorder
		}
		panels = append(panels, style.Width(w2-2).MaxWidth(w2).Height(h).MaxHeight(panelH).Render(m.detailView.View()))
	}

	// Right panel: entries list (top) + content (bottom)
	if w3 > 0 {
		// Two bordered sub-panels must sum to panelH.
		// Each border adds 2, so inner heights sum to panelH - 4.
		innerTotal := panelH - 4
		entriesH := innerTotal / 3
		contentH := innerTotal - entriesH

		entriesStyle := StyleBorder
		if m.focus == FocusEntries {
			entriesStyle = StyleActiveBorder
		}
		entriesPanel := entriesStyle.Width(w3-2).MaxWidth(w3).Height(entriesH).Render(m.entriesList.View())

		contentStyle := StyleBorder
		if m.focus == FocusContent {
			contentStyle = StyleActiveBorder
		}
		contentPanel := contentStyle.Width(w3-2).MaxWidth(w3).Height(contentH).Render(m.contentView.View())

		rightCol := lipgloss.JoinVertical(lipgloss.Left, entriesPanel, contentPanel)
		panels = append(panels, rightCol)
	}

	main := lipgloss.JoinHorizontal(lipgloss.Top, panels...)

	title := StyleTitle.Render("claude-mercato")

	statusBar := m.viewStatusBar()

	if m.mode == ModeSearch {
		statusBar = m.searchInput.View()
	}

	if m.mode == ModeHelp {
		return m.viewHelp()
	}

	if m.mode == ModeMarketPopup {
		return m.viewMarketPopup()
	}

	if m.mode == ModeProfileAction {
		return m.viewProfileAction()
	}

	return lipgloss.JoinVertical(lipgloss.Left, title, main, statusBar)
}

func (m AppModel) viewLoading() string {
	return lipgloss.JoinVertical(lipgloss.Left,
		StyleTitle.Render("claude-mercato"),
		"",
		"  "+m.spinner.View()+" "+m.loadingPhase,
	)
}

func (m AppModel) buildDetailContent() string {
	item, ok := m.profilesList.SelectedItem().(ProfileItem)
	if !ok {
		return "No profile selected"
	}

	w := m.detailView.Width
	if w < 1 {
		w = 40
	}

	bold := lipgloss.NewStyle().Bold(true)
	muted := lipgloss.NewStyle().Foreground(ColorMuted)
	wrap := lipgloss.NewStyle().Width(w)

	var s string
	s += bold.Render(item.Name) + "\n"
	s += muted.Render(item.Market) + "\n\n"

	if len(item.Tags) > 0 {
		for _, tag := range item.Tags {
			s += StyleTag.Render(tag) + " "
		}
		s += "\n\n"
	}

	if item.Readme != "" {
		s += wrap.Render(stripFrontmatter(item.Readme))
	}

	return s
}

func (m AppModel) viewStatusBar() string {
	if m.statusMsg != "" {
		return StyleStatusBar.Width(m.width).Render(m.statusMsg)
	}
	hints := "/ search  i install  x remove  m markets  r refresh  ? help  q quit"
	return StyleStatusBar.Width(m.width).Render(hints)
}

func (m AppModel) viewProfileAction() string {
	p := m.profileActionTarget
	bold := lipgloss.NewStyle().Bold(true)
	muted := lipgloss.NewStyle().Foreground(ColorMuted)

	maxRef := 0
	for _, e := range p.Entries {
		if l := len(string(e.Ref)); l > maxRef {
			maxRef = l
		}
	}
	width := maxRef + 8 // prefix ("  - ") + padding
	if width < 50 {
		width = 50
	}
	if width > m.width-10 {
		width = m.width - 10
	}

	var s string
	var count int
	switch m.profileAction {
	case "install":
		for _, e := range p.Entries {
			if !e.Installed {
				count++
			}
		}
		s += bold.Render("Install profile") + "\n\n"
		s += fmt.Sprintf("Install %d entries from ", count) + bold.Render(p.Name) + muted.Render("@"+p.Market) + "?\n\n"
		for _, e := range p.Entries {
			if e.Installed {
				s += muted.Render("  ✓ "+string(e.Ref)) + "\n"
			} else {
				s += "  + " + string(e.Ref) + "\n"
			}
		}
	case "remove":
		for _, e := range p.Entries {
			if e.Installed {
				count++
			}
		}
		s += bold.Render("Remove profile") + "\n\n"
		s += fmt.Sprintf("Remove %d entries from ", count) + bold.Render(p.Name) + muted.Render("@"+p.Market) + "?\n\n"
		for _, e := range p.Entries {
			if e.Installed {
				s += lipgloss.NewStyle().Foreground(ColorDanger).Render("  - "+string(e.Ref)) + "\n"
			} else {
				s += muted.Render("  · "+string(e.Ref)) + "\n"
			}
		}
	}

	s += "\n" + muted.Render("y confirm  n/esc cancel")

	popup := StyleBorder.
		Width(width).
		Padding(1, 2).
		Render(s)

	return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, popup)
}

func (m AppModel) viewHelp() string {
	help := `
  Keybindings

  Navigation
  j/↓        Move down
  k/↑        Move up
  h/l        Focus left/right panel
  tab        Cycle focus forward
  shift+tab  Cycle focus backward
  pgup/pgdn  Scroll content
  g/G        Top/bottom of list
  enter      View entry content

  Actions
  i          Install profile
  x          Remove profile
  r/R        Refresh market / all
  m          Markets popup
  /          Search mode
  ?          Help (this screen)
  q          Quit
  ctrl+c     Force quit

  Press ? or esc to close
`
	return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center,
		StyleBorder.Padding(1, 2).Render(help))
}

func stripFrontmatter(s string) string {
	if !strings.HasPrefix(s, "---") {
		return strings.TrimSpace(s)
	}
	if end := strings.Index(s[3:], "---"); end != -1 {
		return strings.TrimSpace(s[3+end+3:])
	}
	return strings.TrimSpace(s)
}

func RunTUI(svc TUIServices) error {
	p := tea.NewProgram(NewAppModel(svc), tea.WithAltScreen())
	_, err := p.Run()
	return err
}
