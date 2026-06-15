package tui

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/bm-197/tessera/core/api"
	tea "github.com/charmbracelet/bubbletea"
)

const testSecret = "JBSWY3DPEHPK3PXP"

func setupVault(t *testing.T) Options {
	t.Helper()
	path := filepath.Join(t.TempDir(), "vault.json")
	pass := []byte("tui-test-pass")

	s, err := api.Create(path, pass)
	if err != nil {
		t.Fatal(err)
	}
	gh, err := s.AddURI("otpauth://totp/GitHub:alice@example.com?secret="+testSecret+"&issuer=GitHub", nil)
	if err != nil {
		t.Fatal(err)
	}
	if err := s.SetRecoveryCodes(gh.ID, []string{"zwdg-bywm-btgv", "aojo-diof-1234"}); err != nil {
		t.Fatal(err)
	}
	if _, err := s.AddURI("otpauth://totp/AWS:bob@example.com?secret="+testSecret+"&issuer=AWS", nil); err != nil {
		t.Fatal(err)
	}
	s.Lock()

	return Options{VaultPath: path, Profile: "default", Passphrase: pass}
}

func key(s string) tea.KeyMsg {
	switch s {
	case "down":
		return tea.KeyMsg{Type: tea.KeyDown}
	case "enter":
		return tea.KeyMsg{Type: tea.KeyEnter}
	case "esc":
		return tea.KeyMsg{Type: tea.KeyEsc}
	case "ctrl+s":
		return tea.KeyMsg{Type: tea.KeyCtrlS}
	default:
		return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(s)}
	}
}

func send(m model, msg tea.Msg) model {
	next, _ := m.Update(msg)
	return next.(model)
}

func TestTUI_UnlockAndList(t *testing.T) {
	m := newModel(setupVault(t))
	if m.state != stateList {
		t.Fatalf("supplied passphrase should auto-unlock to list, state=%d", m.state)
	}
	if len(m.codes) != 2 {
		t.Fatalf("expected 2 accounts, got %d", len(m.codes))
	}

	view := m.View()
	for _, want := range []string{"GitHub", "AWS", "🔑"} {
		if !strings.Contains(view, want) {
			t.Errorf("list view missing %q\n%s", want, view)
		}
	}
	t.Logf("\n--- LIST ---\n%s", view)
}

func TestTUI_NavigateToRecoveryCodes(t *testing.T) {
	m := newModel(setupVault(t))

	// Accounts are sorted (AWS, GitHub); move to GitHub and open detail.
	m = send(m, key("down"))
	m = send(m, key("enter"))
	if m.state != stateDetail {
		t.Fatalf("enter should open detail, state=%d", m.state)
	}

	view := m.View()
	if !strings.Contains(view, "zwdg-bywm-btgv") || !strings.Contains(view, "aojo-diof-1234") {
		t.Errorf("detail view should show recovery codes\n%s", view)
	}
	t.Logf("\n--- DETAIL ---\n%s", view)

	// Esc returns to the list.
	m = send(m, key("esc"))
	if m.state != stateList {
		t.Fatalf("esc should return to list, state=%d", m.state)
	}
}

func TestTUI_TickRefreshNoSideEffects(t *testing.T) {
	m := newModel(setupVault(t))
	before := m.codes[0].Entry.Params.Counter
	for i := 0; i < 3; i++ {
		m = send(m, tickMsg{})
	}
	if m.codes[0].Entry.Params.Counter != before {
		t.Error("tick refresh must not advance counters")
	}
	if len(m.codes) != 2 {
		t.Errorf("tick changed account count: %d", len(m.codes))
	}
}

func TestTUI_AddViaURI(t *testing.T) {
	m := newModel(setupVault(t))
	m = send(m, key("a"))
	if m.state != stateAdd {
		t.Fatalf("'a' should open add screen, state=%d", m.state)
	}
	t.Logf("\n--- ADD ---\n%s", m.View())

	m.addSecret.SetValue("otpauth://totp/Stripe:me@example.com?secret=" + testSecret + "&issuer=Stripe")
	m = send(m, key("tab"))
	m.addIssuer.SetValue("Stripe")
	m = send(m, key("enter"))
	if m.state != stateList {
		t.Fatalf("add should return to list, state=%d", m.state)
	}
	if len(m.codes) != 3 {
		t.Fatalf("expected 3 accounts after add, got %d", len(m.codes))
	}
	if !strings.Contains(m.View(), "Stripe") {
		t.Error("added account not shown in list")
	}
}

func TestTUI_AddRequiresName(t *testing.T) {
	m := newModel(setupVault(t))
	start := len(m.codes)
	m = send(m, key("a"))

	// Secret given but no name: must be rejected and stay on the add screen.
	m.addSecret.SetValue(testSecret)
	m = send(m, key("enter"))
	if m.state != stateAdd {
		t.Fatalf("missing name should keep us on the add screen, state=%d", m.state)
	}
	if m.status == "" {
		t.Error("expected an error message about the missing name")
	}
	if len(m.session.List()) != start {
		t.Error("no account should have been added without a name")
	}
}

func TestTUI_AddViaRawSecret(t *testing.T) {
	m := newModel(setupVault(t))
	m = send(m, key("a"))
	m.addSecret.SetValue(testSecret) // raw base32, no URI
	m = send(m, key("tab"))          // focus the account-name field
	m.addIssuer.SetValue("Dropbox")
	m = send(m, key("enter"))

	if len(m.codes) != 3 {
		t.Fatalf("expected 3 accounts, got %d", len(m.codes))
	}
	if !strings.Contains(m.View(), "Dropbox") {
		t.Error("raw-secret account not added with its name")
	}
}

func TestTUI_AddWithLabel(t *testing.T) {
	m := newModel(setupVault(t))
	m = send(m, key("a"))
	m.addSecret.SetValue(testSecret)
	m = send(m, key("tab")) // → account name
	m.addIssuer.SetValue("Stripe")
	m = send(m, key("tab")) // → label
	m.addLabel.SetValue("me@example.com")
	m = send(m, key("enter"))

	e := m.codes[m.cursor].Entry
	if e.Issuer != "Stripe" || e.Label != "me@example.com" {
		t.Fatalf("label not stored: issuer=%q label=%q", e.Issuer, e.Label)
	}

	// Detail view shows the label on its own line.
	m = send(m, key("enter"))
	if !strings.Contains(m.View(), "me@example.com") {
		t.Error("detail view should show the label")
	}
}

func TestTruncate(t *testing.T) {
	if got := truncate("short", 28); got != "short" {
		t.Errorf("short string changed: %q", got)
	}
	long := "Stripe (bisrat.maru-ug@aau.edu.et)"
	got := truncate(long, 28)
	if len([]rune(got)) != 28 || !strings.HasSuffix(got, "…") {
		t.Errorf("expected 28-rune string ending in …, got %q (%d)", got, len([]rune(got)))
	}
}

func TestTUI_DeleteWithConfirm(t *testing.T) {
	m := newModel(setupVault(t))
	start := len(m.codes)

	m = send(m, key("d"))
	if !m.confirming {
		t.Fatal("'d' should start a delete confirmation")
	}
	// 'n' cancels — nothing removed.
	m = send(m, key("n"))
	if m.confirming || len(m.codes) != start {
		t.Fatalf("cancel should keep all accounts: confirming=%v count=%d", m.confirming, len(m.codes))
	}

	// 'd' then 'y' actually deletes.
	m = send(m, key("d"))
	m = send(m, key("y"))
	if len(m.codes) != start-1 {
		t.Fatalf("expected %d accounts after delete, got %d", start-1, len(m.codes))
	}
}

func TestTUI_EditName(t *testing.T) {
	m := newModel(setupVault(t))
	m = send(m, key("enter")) // open detail on first account (AWS)
	m = send(m, key("e"))
	if m.state != stateEditName {
		t.Fatalf("'e' should open the edit-name screen, state=%d", m.state)
	}

	m.editIssuer.SetValue("Amazon")
	m.editLabel.SetValue("ops@example.com")
	m = send(m, key("enter"))
	if m.state != stateDetail {
		t.Fatalf("saving should return to detail, state=%d", m.state)
	}

	e := m.codes[m.cursor].Entry // re-selected after the rename
	if e.ID != m.editID || e.Issuer != "Amazon" || e.Label != "ops@example.com" {
		t.Fatalf("rename not applied: issuer=%q label=%q", e.Issuer, e.Label)
	}
}

func TestTUI_EditRecoveryCodes(t *testing.T) {
	m := newModel(setupVault(t))
	m = send(m, key("enter")) // detail
	m = send(m, key("r"))
	if m.state != stateRecovery {
		t.Fatalf("'r' should open the recovery editor, state=%d", m.state)
	}

	m.recovery.SetValue("code-a\ncode-b\n\n  code-c  ")
	m = send(m, key("ctrl+s"))
	if m.state != stateDetail {
		t.Fatalf("ctrl+s should save and return to detail, state=%d", m.state)
	}

	got, err := m.session.RecoveryCodes(m.editID)
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"code-a", "code-b", "code-c"} // blank lines dropped, trimmed
	if strings.Join(got, ",") != strings.Join(want, ",") {
		t.Fatalf("recovery codes = %v, want %v", got, want)
	}
}

func TestTUI_WrongPassphraseStaysOnPrompt(t *testing.T) {
	opts := setupVault(t)
	opts.Passphrase = nil // force the passphrase screen
	m := newModel(opts)
	if m.state != statePass {
		t.Fatalf("expected passphrase screen, state=%d", m.state)
	}

	m.input.SetValue("wrong")
	m = send(m, key("enter"))
	if m.state != statePass || m.err == nil {
		t.Error("wrong passphrase should stay on prompt with an error")
	}
}
