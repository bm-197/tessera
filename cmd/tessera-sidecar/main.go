// Command tessera-sidecar exposes the Tessera core to the desktop app over a
// line-delimited JSON-RPC protocol on stdin/stdout. The Electron main process
// spawns it, writes one JSON request per line, and reads one JSON response per
// line. All secret handling stays here (in the Go core); the Electron/React
// layers are UI only.
//
// Request:  {"id": <any>, "method": "open", "params": {...}}
// Response: {"id": <any>, "result": {...}}   or   {"id": <any>, "error": "..."}
package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"

	"github.com/bm-197/tessera/core/bridge"
)

type request struct {
	ID     json.RawMessage `json:"id"`
	Method string          `json:"method"`
	Params json.RawMessage `json:"params"`
}

type response struct {
	ID     json.RawMessage `json:"id"`
	Result json.RawMessage `json:"result,omitempty"`
	Error  string          `json:"error,omitempty"`
}

func main() {
	b := bridge.New()
	defer b.Close()

	in := bufio.NewScanner(os.Stdin)
	in.Buffer(make([]byte, 0, 64*1024), 16*1024*1024) // allow large payloads
	out := bufio.NewWriter(os.Stdout)
	enc := json.NewEncoder(out)

	for in.Scan() {
		line := in.Bytes()
		if len(line) == 0 {
			continue
		}

		var req request
		resp := response{}
		if err := json.Unmarshal(line, &req); err != nil {
			resp.Error = "invalid request: " + err.Error()
			writeResponse(enc, out, resp)
			continue
		}
		resp.ID = req.ID

		result, err := b.Call(req.Method, string(req.Params))
		if err != nil {
			resp.Error = err.Error()
		} else {
			resp.Result = json.RawMessage(result)
		}
		writeResponse(enc, out, resp)
	}

	if err := in.Err(); err != nil {
		fmt.Fprintln(os.Stderr, "tessera-sidecar: stdin error:", err)
		os.Exit(1)
	}
}

func writeResponse(enc *json.Encoder, out *bufio.Writer, resp response) {
	// json.Encoder writes a trailing newline, giving us line-delimited output.
	_ = enc.Encode(resp)
	_ = out.Flush()
}
