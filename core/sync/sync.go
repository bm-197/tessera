package sync

import (
	"context"
	"errors"
)

// ErrConflict means the backend's current version no longer matches the
// expected version; the caller should Get, re-merge and retry.
var ErrConflict = errors.New("sync: version conflict")

// Backend stores a single encrypted blob with an opaque version token. It never
// sees a key or plaintext, so a new backend changes nothing in crypto or vault.
type Backend interface {
	Get(ctx context.Context) (blob []byte, version string, err error)
	Put(ctx context.Context, blob []byte, expectedVersion string) (version string, err error)
}
