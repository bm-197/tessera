// Package mobile is the gomobile-bound surface for the React Native app. It is
// a thin wrapper over core/bridge — the exact same JSON command surface the
// desktop sidecar exposes over stdio — but embedded in-process and bound to
// Swift/Kotlin via `gomobile bind`. Only string/error methods are exported so
// gomobile can bind them; no crypto or vault logic lives here.
//
// The JS layer supplies the vault path (the app's documents directory) inside
// the params of open/create/vaultExists, the same way the desktop main process
// injects it.
package mobile

import "github.com/bm-197/tessera/core/bridge"

// Client is the bound object the native module holds for the lifetime of the
// app session. It is not safe for concurrent use; the native module serializes
// calls.
type Client struct {
	b *bridge.Bridge
}

// New returns a new, locked client.
func New() *Client {
	return &Client{b: bridge.New()}
}

// Call dispatches a JSON command and returns a JSON result. See core/bridge for
// the method list (status, vaultExists, create, open, lock, list, codes,
// addURI, addManual, setRecoveryCodes, recoveryCodes, setName, delete, sync).
func (c *Client) Call(method, paramsJSON string) (string, error) {
	return c.b.Call(method, paramsJSON)
}

// Close locks the vault and zeroes retained secrets.
func (c *Client) Close() {
	c.b.Close()
}
