// Package bridge is the single JSON command surface every GUI client uses. The
// desktop sidecar wraps it over stdio JSON-RPC; the mobile build binds it via
// gomobile. Methods take and return JSON strings so the contract is identical
// across transports — and, like the CLI and TUI, this is a thin layer over
// core/api: no crypto or vault logic lives here.
package bridge

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/bm-197/tessera/core/api"
	"github.com/bm-197/tessera/core/otp"
	"github.com/bm-197/tessera/core/sync"
	"github.com/bm-197/tessera/core/vault"
)

// Bridge holds the unlocked session for one client. It is not safe for
// concurrent use; callers (sidecar / native module) serialize calls.
type Bridge struct {
	session *api.Session
	pass    []byte // retained while unlocked, needed to sync the remote envelope
}

// New returns a locked bridge.
func New() *Bridge { return &Bridge{} }

// Call dispatches a method by name with JSON params and returns a JSON result.
// An empty params string is treated as no params. Errors are returned as Go
// errors for the transport to encode.
func (b *Bridge) Call(method, paramsJSON string) (string, error) {
	var params json.RawMessage
	if paramsJSON != "" {
		params = json.RawMessage(paramsJSON)
	}
	result, err := b.dispatch(method, params)
	if err != nil {
		return "", err
	}
	if result == nil {
		return "{}", nil
	}
	out, err := json.Marshal(result)
	if err != nil {
		return "", fmt.Errorf("bridge: marshal result: %w", err)
	}
	return string(out), nil
}

// Close locks the session and zeroes retained secrets.
func (b *Bridge) Close() {
	b.lock()
}

func (b *Bridge) dispatch(method string, params json.RawMessage) (any, error) {
	switch method {
	case "status":
		return statusResult{Unlocked: b.session != nil}, nil
	case "vaultExists":
		return b.vaultExists(params)
	case "create":
		return b.openOrCreate(params, true)
	case "open":
		return b.openOrCreate(params, false)
	case "lock":
		b.lock()
		return nil, nil
	case "list":
		return b.list()
	case "codes":
		return b.codes()
	case "addURI":
		return b.addURI(params)
	case "addManual":
		return b.addManual(params)
	case "setRecoveryCodes":
		return b.setRecoveryCodes(params)
	case "recoveryCodes":
		return b.recoveryCodes(params)
	case "setName":
		return b.setName(params)
	case "delete":
		return b.deleteAccount(params)
	case "sync":
		return b.sync(params)
	default:
		return nil, fmt.Errorf("bridge: unknown method %q", method)
	}
}

// --- views (JSON DTOs; never expose the OTP secret) ---

type accountView struct {
	ID            string `json:"id"`
	Issuer        string `json:"issuer"`
	Label         string `json:"label"`
	Type          string `json:"type"`
	Algorithm     string `json:"algorithm"`
	Digits        int    `json:"digits"`
	Period        int    `json:"period"`
	RecoveryCount int    `json:"recoveryCount"`
}

type codeView struct {
	accountView
	Code        string `json:"code"`
	ExpiresInMs int64  `json:"expiresInMs"`
	Error       string `json:"error,omitempty"`
}

func toAccountView(e *vault.Entry) accountView {
	return accountView{
		ID:            e.ID,
		Issuer:        e.Issuer,
		Label:         e.Label,
		Type:          string(e.Params.Type),
		Algorithm:     string(e.Params.Algorithm),
		Digits:        e.Params.Digits,
		Period:        e.Params.Period,
		RecoveryCount: len(e.RecoveryCodes),
	}
}

// --- results ---

type statusResult struct {
	Unlocked bool `json:"unlocked"`
}

type existsResult struct {
	Exists bool `json:"exists"`
}

type accountsResult struct {
	Accounts []accountView `json:"accounts"`
}

type codesResult struct {
	Codes []codeView `json:"codes"`
}

type accountResult struct {
	Account accountView `json:"account"`
}

type recoveryResult struct {
	Codes []string `json:"codes"`
}

// --- handlers ---

func (b *Bridge) requireUnlocked() error {
	if b.session == nil {
		return fmt.Errorf("bridge: vault is locked")
	}
	return nil
}

func (b *Bridge) lock() {
	if b.session != nil {
		b.session.Lock()
		b.session = nil
	}
	for i := range b.pass {
		b.pass[i] = 0
	}
	b.pass = nil
}

func (b *Bridge) vaultExists(params json.RawMessage) (any, error) {
	var p struct {
		Path string `json:"path"`
	}
	if err := json.Unmarshal(params, &p); err != nil {
		return nil, err
	}
	_, err := api.Open(p.Path, []byte("\x00probe")) // never matches; distinguishes missing vs wrong-pass
	if err == api.ErrNoVault {
		return existsResult{Exists: false}, nil
	}
	return existsResult{Exists: true}, nil
}

func (b *Bridge) openOrCreate(params json.RawMessage, create bool) (any, error) {
	var p struct {
		Path       string `json:"path"`
		Passphrase string `json:"passphrase"`
	}
	if err := json.Unmarshal(params, &p); err != nil {
		return nil, err
	}
	var (
		s   *api.Session
		err error
	)
	if create {
		s, err = api.Create(p.Path, []byte(p.Passphrase))
	} else {
		s, err = api.Open(p.Path, []byte(p.Passphrase))
	}
	if err != nil {
		return nil, err
	}
	b.lock() // drop any previous session
	b.session = s
	b.pass = []byte(p.Passphrase)
	return nil, nil
}

func (b *Bridge) list() (any, error) {
	if err := b.requireUnlocked(); err != nil {
		return nil, err
	}
	entries := b.session.List()
	out := make([]accountView, 0, len(entries))
	for _, e := range entries {
		out = append(out, toAccountView(e))
	}
	return accountsResult{Accounts: out}, nil
}

func (b *Bridge) codes() (any, error) {
	if err := b.requireUnlocked(); err != nil {
		return nil, err
	}
	live := b.session.RenderCodes()
	out := make([]codeView, 0, len(live))
	for _, c := range live {
		cv := codeView{
			accountView: toAccountView(c.Entry),
			Code:        c.Code,
			ExpiresInMs: c.ExpiresIn.Milliseconds(),
		}
		if c.Err != nil {
			cv.Error = c.Err.Error()
		}
		out = append(out, cv)
	}
	return codesResult{Codes: out}, nil
}

func (b *Bridge) addURI(params json.RawMessage) (any, error) {
	if err := b.requireUnlocked(); err != nil {
		return nil, err
	}
	var p struct {
		URI           string   `json:"uri"`
		RecoveryCodes []string `json:"recoveryCodes"`
	}
	if err := json.Unmarshal(params, &p); err != nil {
		return nil, err
	}
	e, err := b.session.AddURI(p.URI, p.RecoveryCodes)
	if err != nil {
		return nil, err
	}
	return accountResult{Account: toAccountView(e)}, nil
}

func (b *Bridge) addManual(params json.RawMessage) (any, error) {
	if err := b.requireUnlocked(); err != nil {
		return nil, err
	}
	var p struct {
		Issuer        string   `json:"issuer"`
		Label         string   `json:"label"`
		Secret        string   `json:"secret"`
		Type          string   `json:"type"`
		Algorithm     string   `json:"algorithm"`
		Digits        int      `json:"digits"`
		Period        int      `json:"period"`
		Counter       uint64   `json:"counter"`
		RecoveryCodes []string `json:"recoveryCodes"`
	}
	if err := json.Unmarshal(params, &p); err != nil {
		return nil, err
	}
	prm := otp.Params{
		Type:      otp.Kind(p.Type),
		Algorithm: otp.Algorithm(p.Algorithm),
		Digits:    p.Digits,
		Period:    p.Period,
		Counter:   p.Counter,
	}
	e, err := b.session.AddManual(p.Issuer, p.Label, p.Secret, prm, p.RecoveryCodes)
	if err != nil {
		return nil, err
	}
	return accountResult{Account: toAccountView(e)}, nil
}

func (b *Bridge) setRecoveryCodes(params json.RawMessage) (any, error) {
	if err := b.requireUnlocked(); err != nil {
		return nil, err
	}
	var p struct {
		ID    string   `json:"id"`
		Codes []string `json:"codes"`
	}
	if err := json.Unmarshal(params, &p); err != nil {
		return nil, err
	}
	return nil, b.session.SetRecoveryCodes(p.ID, p.Codes)
}

func (b *Bridge) recoveryCodes(params json.RawMessage) (any, error) {
	if err := b.requireUnlocked(); err != nil {
		return nil, err
	}
	var p struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(params, &p); err != nil {
		return nil, err
	}
	codes, err := b.session.RecoveryCodes(p.ID)
	if err != nil {
		return nil, err
	}
	return recoveryResult{Codes: codes}, nil
}

func (b *Bridge) setName(params json.RawMessage) (any, error) {
	if err := b.requireUnlocked(); err != nil {
		return nil, err
	}
	var p struct {
		ID     string `json:"id"`
		Issuer string `json:"issuer"`
		Label  string `json:"label"`
	}
	if err := json.Unmarshal(params, &p); err != nil {
		return nil, err
	}
	return nil, b.session.SetName(p.ID, p.Issuer, p.Label)
}

func (b *Bridge) deleteAccount(params json.RawMessage) (any, error) {
	if err := b.requireUnlocked(); err != nil {
		return nil, err
	}
	var p struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(params, &p); err != nil {
		return nil, err
	}
	return nil, b.session.Delete(p.ID)
}

func (b *Bridge) sync(params json.RawMessage) (any, error) {
	if err := b.requireUnlocked(); err != nil {
		return nil, err
	}
	var p struct {
		Path string `json:"path"`
	}
	if err := json.Unmarshal(params, &p); err != nil {
		return nil, err
	}
	if p.Path == "" {
		return nil, fmt.Errorf("bridge: sync requires a path")
	}
	backend := sync.NewFS(p.Path)
	return nil, b.session.Sync(context.Background(), backend, b.pass)
}
