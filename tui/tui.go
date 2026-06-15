// Package tui is a Bubble Tea terminal UI for tessera. Like the CLI, it is a
// thin client over core/api and never touches crypto or vault internals.
package tui

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/bm-197/tessera/core/api"
	"github.com/bm-197/tessera/core/crypto"
	"github.com/bm-197/tessera/core/otp"
	"github.com/bm-197/tessera/core/sync"
	"github.com/bm-197/tessera/core/vault"
	"github.com/charmbracelet/bubbles/textarea"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type Options struct {
	VaultPath  string
	SyncPath   string
	Profile    string
	Passphrase []byte // optional; if it unlocks, the passphrase screen is skipped
}

// Run starts the TUI and blocks until the user quits.
func Run(opts Options) error {
	m := newModel(opts)
	p := tea.NewProgram(m, tea.WithAltScreen())
	final, err := p.Run()
	if fm, ok := final.(model); ok {
		if fm.session != nil {
			fm.session.Lock()
		}
		crypto.Zero(fm.pass)
	}
	return err
}

type state int

const (
	statePass state = iota
	stateList
	stateDetail
	stateAdd
	stateEditName
	stateRecovery
)

type (
	tickMsg     time.Time
	syncDoneMsg struct{ err error }
)

func tickCmd() tea.Cmd {
	return tea.Tick(time.Second, func(t time.Time) tea.Msg { return tickMsg(t) })
}

type model struct {
	opts       Options
	state      state
	input      textinput.Model
	addSecret  textinput.Model
	addIssuer  textinput.Model
	addLabel   textinput.Model
	addFocus   int
	editIssuer textinput.Model
	editLabel  textinput.Model
	editFocus  int
	editID     string
	recovery   textarea.Model
	confirming bool
	session    *api.Session
	pass       []byte
	codes      []api.LiveCode
	cursor     int
	status     string
	err        error
	width      int
	height     int
}

func newModel(opts Options) model {
	ti := textinput.New()
	ti.Placeholder = "passphrase"
	ti.EchoMode = textinput.EchoPassword
	ti.EchoCharacter = '•'
	ti.Focus()

	si := textinput.New()
	si.Placeholder = "otpauth://… or a base32 secret"
	si.Width = 50
	ii := textinput.New()
	ii.Placeholder = "account name, e.g. GitHub"
	ii.Width = 50
	li := textinput.New()
	li.Placeholder = "username/email, e.g. you@example.com"
	li.Width = 50

	ei := textinput.New()
	ei.Width = 50
	el := textinput.New()
	el.Width = 50

	ta := textarea.New()
	ta.Placeholder = "one recovery code per line"
	ta.SetWidth(50)
	ta.SetHeight(6)

	m := model{
		opts: opts, state: statePass, input: ti,
		addSecret: si, addIssuer: ii, addLabel: li,
		editIssuer: ei, editLabel: el, recovery: ta,
	}

	// If a passphrase was supplied (e.g. $TESSERA_PASSPHRASE) and it unlocks,
	// skip the passphrase screen entirely.
	if len(opts.Passphrase) > 0 {
		if s, err := api.Open(opts.VaultPath, opts.Passphrase); err == nil {
			m.session = s
			m.pass = append([]byte(nil), opts.Passphrase...)
			m.state = stateList
			m.codes = s.RenderCodes()
			m.input.Blur()
		}
	}
	return m
}

func (m model) Init() tea.Cmd {
	if m.state == stateList {
		return tickCmd()
	}
	return textinput.Blink
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		return m, nil

	case tea.KeyMsg:
		switch m.state {
		case statePass:
			return m.updatePass(msg)
		case stateList:
			return m.updateList(msg)
		case stateDetail:
			return m.updateDetail(msg)
		case stateAdd:
			return m.updateAdd(msg)
		case stateEditName:
			return m.updateEditName(msg)
		case stateRecovery:
			return m.updateRecovery(msg)
		}

	case tickMsg:
		if m.session != nil {
			m.codes = m.session.RenderCodes()
			m.clampCursor()
		}
		return m, tickCmd()

	case syncDoneMsg:
		if msg.err != nil {
			m.status = "sync failed: " + msg.err.Error()
		} else {
			m.codes = m.session.RenderCodes()
			m.clampCursor()
			m.status = "synced ✓"
		}
		return m, nil
	}

	var cmd tea.Cmd
	switch m.state {
	case statePass:
		m.input, cmd = m.input.Update(msg)
	case stateAdd:
		switch m.addFocus {
		case 0:
			m.addSecret, cmd = m.addSecret.Update(msg)
		case 1:
			m.addIssuer, cmd = m.addIssuer.Update(msg)
		case 2:
			m.addLabel, cmd = m.addLabel.Update(msg)
		}
	case stateEditName:
		if m.editFocus == 0 {
			m.editIssuer, cmd = m.editIssuer.Update(msg)
		} else {
			m.editLabel, cmd = m.editLabel.Update(msg)
		}
	case stateRecovery:
		m.recovery, cmd = m.recovery.Update(msg)
	}
	return m, cmd
}

func (m model) updatePass(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "ctrl+c", "esc":
		return m, tea.Quit
	case "enter":
		val := m.input.Value()
		s, err := api.Open(m.opts.VaultPath, []byte(val))
		if err != nil {
			m.err = err
			m.input.Reset()
			return m, nil
		}
		m.session = s
		m.pass = []byte(val)
		m.err = nil
		m.input.Blur()
		m.state = stateList
		m.codes = s.RenderCodes()
		return m, tickCmd()
	}
	var cmd tea.Cmd
	m.input, cmd = m.input.Update(msg)
	return m, cmd
}

func (m model) updateList(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if m.confirming {
		switch msg.String() {
		case "y", "Y":
			e := m.codes[m.cursor].Entry
			name := displayName(e)
			if err := m.session.Delete(e.ID); err != nil {
				m.status = "delete failed: " + err.Error()
			} else {
				m.codes = m.session.RenderCodes()
				m.clampCursor()
				m.status = "deleted " + name
			}
			m.confirming = false
		case "n", "N", "esc", "ctrl+c":
			m.confirming = false
			m.status = "cancelled"
		}
		return m, nil
	}

	switch msg.String() {
	case "ctrl+c", "q":
		return m, tea.Quit
	case "up", "k":
		if m.cursor > 0 {
			m.cursor--
		}
	case "down", "j":
		if m.cursor < len(m.codes)-1 {
			m.cursor++
		}
	case "enter", "right", "l":
		if len(m.codes) > 0 {
			m.state = stateDetail
		}
	case "a":
		m.addSecret.Reset()
		m.addIssuer.Reset()
		m.addLabel.Reset()
		m.addFocus = 0
		m.focusAddInputs()
		m.status = ""
		m.state = stateAdd
		return m, textinput.Blink
	case "d":
		if len(m.codes) > 0 {
			m.confirming = true
			m.status = ""
		}
	case "r":
		if m.session != nil {
			m.codes = m.session.RenderCodes()
			m.status = "refreshed"
		}
	case "s":
		if m.opts.SyncPath == "" {
			m.status = "no sync path set — run `tessera sync --path FILE` once from the CLI"
			return m, nil
		}
		m.status = "syncing…"
		return m, m.syncCmd()
	}
	return m, nil
}

func (m model) updateDetail(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "ctrl+c", "q":
		return m, tea.Quit
	case "esc", "left", "h", "backspace":
		m.state = stateList
	case "e":
		if m.cursor < len(m.codes) {
			e := m.codes[m.cursor].Entry
			m.editID = e.ID
			m.editIssuer.SetValue(e.Issuer)
			m.editLabel.SetValue(e.Label)
			m.editFocus = 0
			m.focusEditInputs()
			m.status = ""
			m.state = stateEditName
			return m, textinput.Blink
		}
	case "r":
		if m.cursor < len(m.codes) {
			e := m.codes[m.cursor].Entry
			m.editID = e.ID
			m.recovery.SetValue(strings.Join(e.RecoveryCodes, "\n"))
			m.status = ""
			m.state = stateRecovery
			return m, m.recovery.Focus()
		}
	}
	return m, nil
}

func (m model) updateEditName(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "ctrl+c":
		return m, tea.Quit
	case "esc":
		m.state = stateDetail
		m.status = ""
		return m, nil
	case "tab", "shift+tab", "up", "down":
		m.editFocus = (m.editFocus + 1) % 2
		m.focusEditInputs()
		return m, nil
	case "enter":
		issuer := strings.TrimSpace(m.editIssuer.Value())
		if issuer == "" {
			m.status = "enter an account name"
			return m, nil
		}
		if err := m.session.SetName(m.editID, issuer, strings.TrimSpace(m.editLabel.Value())); err != nil {
			m.status = "could not rename: " + err.Error()
			return m, nil
		}
		m.codes = m.session.RenderCodes()
		m.selectByID(m.editID) // re-select: a rename can change sort order
		m.state = stateDetail
		m.status = "renamed"
		return m, nil
	}
	var cmd tea.Cmd
	if m.editFocus == 0 {
		m.editIssuer, cmd = m.editIssuer.Update(msg)
	} else {
		m.editLabel, cmd = m.editLabel.Update(msg)
	}
	return m, cmd
}

func (m model) updateRecovery(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "ctrl+c":
		return m, tea.Quit
	case "esc":
		m.state = stateDetail
		m.status = "cancelled"
		return m, nil
	case "ctrl+s":
		var codes []string
		for _, line := range strings.Split(m.recovery.Value(), "\n") {
			if l := strings.TrimSpace(line); l != "" {
				codes = append(codes, l)
			}
		}
		if err := m.session.SetRecoveryCodes(m.editID, codes); err != nil {
			m.status = "could not save: " + err.Error()
			return m, nil
		}
		m.codes = m.session.RenderCodes()
		m.selectByID(m.editID)
		m.state = stateDetail
		m.status = fmt.Sprintf("saved %d recovery codes", len(codes))
		return m, nil
	}
	var cmd tea.Cmd
	m.recovery, cmd = m.recovery.Update(msg)
	return m, cmd
}

func (m *model) focusEditInputs() {
	if m.editFocus == 0 {
		m.editIssuer.Focus()
		m.editLabel.Blur()
	} else {
		m.editLabel.Focus()
		m.editIssuer.Blur()
	}
}

func (m model) updateAdd(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "ctrl+c":
		return m, tea.Quit
	case "esc":
		m.addSecret.Reset()
		m.addIssuer.Reset()
		m.addLabel.Reset()
		m.state = stateList
		m.status = ""
		return m, nil
	case "tab", "down":
		m.addFocus = (m.addFocus + 1) % 3
		m.focusAddInputs()
		return m, nil
	case "shift+tab", "up":
		m.addFocus = (m.addFocus + 2) % 3
		m.focusAddInputs()
		return m, nil
	case "enter":
		return m.submitAdd()
	}
	var cmd tea.Cmd
	switch m.addFocus {
	case 0:
		m.addSecret, cmd = m.addSecret.Update(msg)
	case 1:
		m.addIssuer, cmd = m.addIssuer.Update(msg)
	case 2:
		m.addLabel, cmd = m.addLabel.Update(msg)
	}
	return m, cmd
}

func (m model) submitAdd() (tea.Model, tea.Cmd) {
	raw := strings.TrimSpace(m.addSecret.Value())
	name := strings.TrimSpace(m.addIssuer.Value())
	if raw == "" {
		m.status = "enter a secret or otpauth:// URI"
		return m, nil
	}
	if name == "" {
		m.status = "enter an account name"
		return m, nil
	}

	// Parse a URI for its secret/label/params, but always use the typed name as
	// the account name so every entry is named.
	label, secret, params := strings.TrimSpace(m.addLabel.Value()), raw, otp.Defaults()
	if strings.HasPrefix(raw, "otpauth://") {
		acc, err := otp.ParseURI(raw)
		if err != nil {
			m.status = "could not add: " + err.Error()
			return m, nil
		}
		secret, params = acc.Secret, acc.Params
		if label == "" {
			label = acc.Label // fall back to the URI's label if none typed
		}
	}

	entry, err := m.session.AddManual(name, label, secret, params, nil)
	if err != nil {
		m.status = "could not add: " + err.Error()
		return m, nil
	}

	m.addSecret.Reset()
	m.addIssuer.Reset()
	m.addLabel.Reset()
	m.codes = m.session.RenderCodes()
	m.selectByID(entry.ID)
	m.state = stateList
	m.status = "added " + displayName(entry)
	return m, nil
}

func (m *model) focusAddInputs() {
	m.addSecret.Blur()
	m.addIssuer.Blur()
	m.addLabel.Blur()
	switch m.addFocus {
	case 0:
		m.addSecret.Focus()
	case 1:
		m.addIssuer.Focus()
	case 2:
		m.addLabel.Focus()
	}
}

func (m *model) selectByID(id string) {
	for i, c := range m.codes {
		if c.Entry.ID == id {
			m.cursor = i
			return
		}
	}
}

func (m model) syncCmd() tea.Cmd {
	session := m.session
	pass := m.pass
	path := m.opts.SyncPath
	return func() tea.Msg {
		backend := sync.NewFS(path)
		return syncDoneMsg{err: session.Sync(context.Background(), backend, pass)}
	}
}

func (m *model) clampCursor() {
	if m.cursor >= len(m.codes) {
		m.cursor = len(m.codes) - 1
	}
	if m.cursor < 0 {
		m.cursor = 0
	}
}

// --- styles ---

var (
	titleStyle    = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("13"))
	dimStyle      = lipgloss.NewStyle().Foreground(lipgloss.Color("240"))
	selectedStyle = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("14"))
	codeStyle     = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("10"))
	errStyle      = lipgloss.NewStyle().Foreground(lipgloss.Color("9"))
	keyStyle      = lipgloss.NewStyle().Foreground(lipgloss.Color("11"))
)

// minPanelWidth keeps the divider and panel from collapsing on narrow content.
const minPanelWidth = 48

func (m model) View() string {
	var top, footer string
	switch m.state {
	case statePass:
		top, footer = m.viewPass()
	case stateDetail:
		top, footer = m.viewDetail()
	case stateAdd:
		top, footer = m.viewAdd()
	case stateEditName:
		top, footer = m.viewEditName()
	case stateRecovery:
		top, footer = m.viewRecovery()
	default:
		top, footer = m.viewList()
	}
	if m.width == 0 || m.height == 0 {
		if footer != "" {
			return top + "\n\n" + footer
		}
		return top
	}
	return m.frame(top, footer)
}

// frame lays out a screen like terminal.shop: the top content sits near the top
// of a centered panel and the commands footer is pinned to the bottom, divided
// by a thin rule, with vertical breathing room between them.
func (m model) frame(top, footer string) string {
	width := lipgloss.Width(top)
	if w := lipgloss.Width(footer); w > width {
		width = w
	}
	if width < minPanelWidth {
		width = minPanelWidth
	}
	if max := m.width - 4; width > max && max > 0 {
		width = max
	}

	var bottom strings.Builder
	if m.status != "" {
		bottom.WriteString(statusView(m.status) + "\n")
	}
	bottom.WriteString(dimStyle.Render(strings.Repeat("─", width)))
	if footer != "" {
		bottom.WriteString("\n" + footer)
	}
	bottomStr := bottom.String()

	// Panel height: most of the viewport, leaving a margin so it doesn't touch
	// the edges, capped so it doesn't stretch absurdly on huge terminals.
	panelH := m.height - 6
	if panelH > 32 {
		panelH = 32
	}
	if panelH < 8 {
		panelH = m.height
	}
	gap := panelH - lipgloss.Height(top) - lipgloss.Height(bottomStr)
	if gap < 1 {
		gap = 1
	}

	body := top + strings.Repeat("\n", gap) + bottomStr
	block := lipgloss.NewStyle().Width(width).Render(body)
	return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, block)
}

func statusView(s string) string {
	lower := strings.ToLower(s)
	for _, k := range []string{"fail", "could not", "enter ", "conflict", "wrong"} {
		if strings.Contains(lower, k) {
			return errStyle.Render(s)
		}
	}
	return dimStyle.Render(s)
}

func (m model) viewPass() (string, string) {
	var b strings.Builder
	b.WriteString(titleStyle.Render("Tessera") + "\n\n")
	b.WriteString("Unlock vault (" + m.opts.Profile + ")\n\n")
	b.WriteString(m.input.View())
	if m.err != nil {
		b.WriteString("\n\n" + errStyle.Render("wrong passphrase, try again"))
	}
	return b.String(), dimStyle.Render("enter unlock · esc quit")
}

func (m model) viewList() (string, string) {
	var b strings.Builder
	b.WriteString(titleStyle.Render("Tessera") + dimStyle.Render("  ·  "+m.opts.Profile) + "\n\n")

	if len(m.codes) == 0 {
		b.WriteString(dimStyle.Render("no accounts — press a to add one"))
	}
	for i, c := range m.codes {
		// Fixed-width name column (truncated with …) so a long label can't
		// push this row's code/countdown out of line with the others. Pad the
		// plain text BEFORE styling — ANSI bytes would throw off width math.
		const nameW = 28
		cursor := "  "
		nameCol := padName(truncate(displayName(c.Entry), nameW), nameW)
		if i == m.cursor {
			cursor = selectedStyle.Render("▸ ")
			nameCol = selectedStyle.Render(nameCol)
		}
		line := cursor + nameCol + " "
		if c.Err != nil {
			line += errStyle.Render("error: " + c.Err.Error())
		} else {
			line += codeStyle.Render(formatCode(c.Code))
			if c.ExpiresIn > 0 {
				line += "  " + countdown(c)
			}
			if n := len(c.Entry.RecoveryCodes); n > 0 {
				line += dimStyle.Render(fmt.Sprintf("  🔑%d", n))
			}
		}
		if i > 0 {
			b.WriteString("\n\n") // blank line between accounts
		}
		b.WriteString(line)
	}

	if m.confirming {
		name := "(none)"
		if m.cursor < len(m.codes) {
			name = displayName(m.codes[m.cursor].Entry)
		}
		footer := errStyle.Render("delete "+name+"?  ") + keyStyle.Render("y") + dimStyle.Render(" / ") + keyStyle.Render("n")
		return b.String(), footer
	}

	footer := dimStyle.Render("↑/↓ move · ") + keyStyle.Render("enter") + dimStyle.Render(" details · ") + keyStyle.Render("a") + dimStyle.Render(" add · ") + keyStyle.Render("d") + dimStyle.Render(" delete · ") + keyStyle.Render("s") + dimStyle.Render(" sync · ") + keyStyle.Render("q") + dimStyle.Render(" quit")
	return b.String(), footer
}

func (m model) viewAdd() (string, string) {
	var b strings.Builder
	b.WriteString(titleStyle.Render("Add account") + "\n\n")
	b.WriteString("Secret or otpauth:// URI " + dimStyle.Render("(required)") + "\n")
	b.WriteString(m.addSecret.View() + "\n\n")
	b.WriteString("Account name " + dimStyle.Render("(required)") + "\n")
	b.WriteString(m.addIssuer.View() + "\n\n")
	b.WriteString("Label " + dimStyle.Render("(optional, e.g. your username/email)") + "\n")
	b.WriteString(m.addLabel.View())
	return b.String(), dimStyle.Render("tab switch field · enter add · esc cancel")
}

func (m model) viewDetail() (string, string) {
	if m.cursor >= len(m.codes) {
		return m.viewList()
	}
	c := m.codes[m.cursor]
	e := c.Entry
	p := e.Params

	title := e.Issuer
	if title == "" {
		title = e.Label
	}
	if title == "" {
		title = "(unnamed)"
	}

	var b strings.Builder
	b.WriteString(titleStyle.Render(title) + "\n\n")
	if e.Issuer != "" && e.Label != "" {
		b.WriteString("Account: " + dimStyle.Render(e.Label) + "\n")
	}
	if c.Err != nil {
		b.WriteString("Code:    " + errStyle.Render(c.Err.Error()) + "\n")
	} else {
		exp := ""
		if c.ExpiresIn > 0 {
			exp = dimStyle.Render(fmt.Sprintf("   (expires in %ds)", int(c.ExpiresIn/time.Second)))
		}
		b.WriteString("Code:    " + codeStyle.Render(formatCode(c.Code)) + exp + "\n")
	}
	b.WriteString("Type:    " + dimStyle.Render(fmt.Sprintf("%s · %s · %d digits · %ds",
		strings.ToUpper(string(p.Type)), p.Algorithm, p.Digits, p.Period)) + "\n")
	b.WriteString("ID:      " + dimStyle.Render(e.ID) + "\n\n")

	b.WriteString(keyStyle.Render("Recovery codes") + "\n")
	if len(e.RecoveryCodes) == 0 {
		b.WriteString(dimStyle.Render("  (none stored)"))
	} else {
		for i, rc := range e.RecoveryCodes {
			if i > 0 {
				b.WriteString("\n")
			}
			b.WriteString("  " + rc)
		}
	}
	footer := keyStyle.Render("e") + dimStyle.Render(" edit name · ") + keyStyle.Render("r") + dimStyle.Render(" recovery codes · ") + keyStyle.Render("esc") + dimStyle.Render(" back · ") + keyStyle.Render("q") + dimStyle.Render(" quit")
	return b.String(), footer
}

func (m model) viewEditName() (string, string) {
	var b strings.Builder
	b.WriteString(titleStyle.Render("Edit name") + "\n\n")
	b.WriteString("Account name " + dimStyle.Render("(required)") + "\n")
	b.WriteString(m.editIssuer.View() + "\n\n")
	b.WriteString("Label " + dimStyle.Render("(optional)") + "\n")
	b.WriteString(m.editLabel.View())
	return b.String(), dimStyle.Render("tab switch field · enter save · esc cancel")
}

func (m model) viewRecovery() (string, string) {
	name := "account"
	if m.cursor < len(m.codes) {
		name = displayName(m.codes[m.cursor].Entry)
	}
	var b strings.Builder
	b.WriteString(titleStyle.Render("Recovery codes") + dimStyle.Render("  ·  "+name) + "\n\n")
	b.WriteString(m.recovery.View())
	return b.String(), dimStyle.Render("one code per line · ") + keyStyle.Render("ctrl+s") + dimStyle.Render(" save · ") + keyStyle.Render("esc") + dimStyle.Render(" cancel")
}

func padName(s string, w int) string {
	if n := len([]rune(s)); n < w {
		return s + strings.Repeat(" ", w-n)
	}
	return s
}

func truncate(s string, w int) string {
	r := []rune(s)
	if len(r) <= w {
		return s
	}
	if w <= 1 {
		return string(r[:max(w, 0)])
	}
	return string(r[:w-1]) + "…"
}

func countdown(c api.LiveCode) string {
	secs := int(c.ExpiresIn / time.Second)
	period := c.Entry.Params.Period
	if period <= 0 {
		period = 30
	}
	const width = 8
	filled := secs * width / period
	if filled > width {
		filled = width
	}
	bar := strings.Repeat("█", filled) + strings.Repeat("░", width-filled)
	style := codeStyle
	if secs <= 5 {
		style = errStyle
	}
	return style.Render(bar) + dimStyle.Render(fmt.Sprintf(" %2ds", secs))
}

func formatCode(code string) string {
	switch len(code) {
	case 6:
		return code[:3] + " " + code[3:]
	case 8:
		return code[:4] + " " + code[4:]
	default:
		return code
	}
}

func displayName(e *vault.Entry) string {
	switch {
	case e.Issuer != "" && e.Label != "":
		return e.Issuer + " (" + e.Label + ")"
	case e.Issuer != "":
		return e.Issuer
	case e.Label != "":
		return e.Label
	default:
		return "(unnamed)"
	}
}
