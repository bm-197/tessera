package vault

import (
	"encoding/json"
	"strings"
	"time"

	"github.com/bm-197/tessera/core/otp"
)

// Merge is a three-way merge that prevents account loss: union of entries
// (nothing dropped), per-field last-write-wins, and tombstones that survive a
// stale copy. Timestamp ties are broken by value so all devices converge
// regardless of sync order.
func Merge(a, b *Vault) *Vault {
	out := New()
	for id, ea := range a.Entries {
		if eb, ok := b.Entries[id]; ok {
			out.Entries[id] = mergeEntry(ea, eb)
		} else {
			out.Entries[id] = ea.Clone()
		}
	}
	for id, eb := range b.Entries {
		if _, ok := a.Entries[id]; !ok {
			out.Entries[id] = eb.Clone()
		}
	}
	return out
}

func mergeEntry(a, b *Entry) *Entry {
	out := &Entry{ID: a.ID, Modified: map[string]time.Time{}}
	out.CreatedAt = earliest(a.CreatedAt, b.CreatedAt)

	out.Issuer, _ = pick(FieldIssuer, a, b, a.Issuer, b.Issuer, strings.Compare, out.Modified)
	out.Label, _ = pick(FieldLabel, a, b, a.Label, b.Label, strings.Compare, out.Modified)
	out.Secret, _ = pick(FieldSecret, a, b, a.Secret, b.Secret, strings.Compare, out.Modified)
	out.RecoveryCodes, _ = pick(FieldRecovery, a, b, a.RecoveryCodes, b.RecoveryCodes, compareCodes, out.Modified)
	out.Params, _ = pick(FieldParams, a, b, a.Params, b.Params, compareParams, out.Modified)
	out.Deleted, _ = pick(FieldDeleted, a, b, a.Deleted, b.Deleted, preferDeleted, out.Modified)

	for _, t := range out.Modified {
		if t.After(out.ModifiedAt) {
			out.ModifiedAt = t
		}
	}
	return out
}

// pick chooses the field value with the newer per-field timestamp, breaking
// ties with tiebreak (>=0 keeps va) so the merge converges deterministically.
func pick[T any](field string, a, b *Entry, va, vb T, tiebreak func(x, y T) int, mod map[string]time.Time) (T, time.Time) {
	ta := a.Modified[field]
	tb := b.Modified[field]
	switch {
	case ta.After(tb):
		mod[field] = ta
		return va, ta
	case tb.After(ta):
		mod[field] = tb
		return vb, tb
	default:
		mod[field] = ta
		if tiebreak(va, vb) >= 0 {
			return va, ta
		}
		return vb, tb
	}
}

func earliest(a, b time.Time) time.Time {
	switch {
	case a.IsZero():
		return b
	case b.IsZero():
		return a
	case a.Before(b):
		return a
	default:
		return b
	}
}

func compareCodes(a, b []string) int {
	return strings.Compare(strings.Join(a, "\x00"), strings.Join(b, "\x00"))
}

func compareParams(a, b otp.Params) int {
	ba, _ := json.Marshal(a)
	bb, _ := json.Marshal(b)
	return strings.Compare(string(ba), string(bb))
}

// preferDeleted breaks a delete/undelete tie toward deletion (a later real edit
// can still revive it; wrongly resurrecting loses the user's intent).
func preferDeleted(a, b bool) int {
	if a == b {
		return 0
	}
	if a {
		return 1
	}
	return -1
}
