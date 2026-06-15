package vault

import (
	"testing"
	"time"

	"github.com/bm-197/tessera/core/otp"
)

func acct(issuer, label, secret string) otp.Account {
	return otp.Account{Issuer: issuer, Label: label, Secret: secret, Params: otp.Defaults()}
}

// Mandatory invariant: A added on device 1 + B added on device 2 → merge has both.
func TestMergeInvariant_NoAccountLost(t *testing.T) {
	base := time.Unix(1_000_000, 0)

	device1 := New()
	a := device1.Add(acct("GitHub", "alice", "GEZDGNBVGY3TQOJQ"), nil, base)

	device2 := New()
	b := device2.Add(acct("AWS", "bob", "GEZDGNBVGY3TQOJQ"), nil, base.Add(time.Second))

	merged := Merge(device1, device2)

	if merged.Get(a.ID) == nil {
		t.Error("account A (added on device 1) was lost in merge")
	}
	if merged.Get(b.ID) == nil {
		t.Error("account B (added on device 2) was lost in merge")
	}
	if got := len(merged.Live()); got != 2 {
		t.Errorf("expected 2 live accounts after merge, got %d", got)
	}

	reverse := Merge(device2, device1) // commutative

	if len(reverse.Live()) != 2 || reverse.Get(a.ID) == nil || reverse.Get(b.ID) == nil {
		t.Error("merge is not commutative")
	}
}

// Concurrent edits to different fields of the same account must both survive.
func TestMerge_PerFieldLWW(t *testing.T) {
	base := time.Unix(1_000_000, 0)

	device1 := New()
	e := device1.Add(acct("Bank", "old-name", "GEZDGNBVGY3TQOJQ"), nil, base)
	device2 := &Vault{Entries: map[string]*Entry{e.ID: e.Clone()}}

	device1.Get(e.ID).SetLabel("Bank", "new-name", base.Add(10*time.Second))
	device2.Get(e.ID).SetRecoveryCodes([]string{"r1", "r2"}, base.Add(20*time.Second))

	merged := Merge(device1, device2)
	got := merged.Get(e.ID)
	if got.Label != "new-name" {
		t.Errorf("label edit lost: got %q", got.Label)
	}
	if len(got.RecoveryCodes) != 2 {
		t.Errorf("recovery-code edit lost: got %v", got.RecoveryCodes)
	}
}

// A delete must survive a merge with a stale copy that still has it live.
func TestMerge_TombstoneNotResurrected(t *testing.T) {
	base := time.Unix(1_000_000, 0)

	device1 := New()
	e := device1.Add(acct("Old", "x", "GEZDGNBVGY3TQOJQ"), nil, base)
	stale := &Vault{Entries: map[string]*Entry{e.ID: e.Clone()}}

	device1.Get(e.ID).Delete(base.Add(time.Hour))

	merged := Merge(stale, device1)
	if !merged.Get(e.ID).Deleted {
		t.Error("deleted account was resurrected by stale copy")
	}
	if len(merged.Live()) != 0 {
		t.Error("tombstoned account still appears as live")
	}
}

// A newer un-delete must win over an older tombstone.
func TestMerge_UndeleteWins(t *testing.T) {
	base := time.Unix(1_000_000, 0)

	withTombstone := New()
	e := withTombstone.Add(acct("Svc", "y", "GEZDGNBVGY3TQOJQ"), nil, base)
	withTombstone.Get(e.ID).Delete(base.Add(time.Minute))

	revived := &Vault{Entries: map[string]*Entry{e.ID: e.Clone()}}
	revived.Get(e.ID).Deleted = false
	revived.Get(e.ID).touch(FieldDeleted, base.Add(time.Hour))

	merged := Merge(withTombstone, revived)
	if merged.Get(e.ID).Deleted {
		t.Error("newer un-delete should win over older tombstone")
	}
}

func TestMerge_Idempotent(t *testing.T) {
	base := time.Unix(1_000_000, 0)
	v := New()
	v.Add(acct("A", "a", "GEZDGNBVGY3TQOJQ"), []string{"r"}, base)
	v.Add(acct("B", "b", "GEZDGNBVGY3TQOJQ"), nil, base)

	merged := Merge(v, v)
	if len(merged.Live()) != 2 {
		t.Errorf("idempotent merge changed entry count: %d", len(merged.Live()))
	}
}
