package api

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/bm-197/tessera/core/sync"
)

const testSecret = "GEZDGNBVGY3TQOJQGEZDGNBVGY3TQOJQ"

func TestCreateOpenRoundTrip(t *testing.T) {
	path := filepath.Join(t.TempDir(), "vault.json")
	pass := []byte("hunter2")

	s, err := Create(path, pass)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.AddURI("otpauth://totp/GitHub:me?secret="+testSecret+"&issuer=GitHub", []string{"rec-1"}); err != nil {
		t.Fatal(err)
	}
	s.Lock()

	s2, err := Open(path, pass)
	if err != nil {
		t.Fatal(err)
	}
	defer s2.Lock()
	live := s2.List()
	if len(live) != 1 || live[0].Issuer != "GitHub" {
		t.Fatalf("reopened vault wrong: %+v", live)
	}
	codes, err := s2.RecoveryCodes(live[0].ID)
	if err != nil || len(codes) != 1 || codes[0] != "rec-1" {
		t.Fatalf("recovery codes not persisted: %v %v", codes, err)
	}

	if _, err := Open(path, []byte("wrong")); err == nil {
		t.Fatal("open with wrong passphrase should fail")
	}
}

// End-to-end no-loss: two vaults, each adding a different account, converge
// through the backend (encrypt/decrypt/CAS) to a vault with both.
func TestSync_TwoDevicesNoLoss(t *testing.T) {
	dir := t.TempDir()
	remote := filepath.Join(dir, "remote", "vault.blob")
	pass := []byte("shared-passphrase")
	ctx := context.Background()
	backend := sync.NewFS(remote)

	d1, err := Create(filepath.Join(dir, "d1.json"), pass)
	if err != nil {
		t.Fatal(err)
	}
	a, err := d1.AddURI("otpauth://totp/GitHub:alice?secret="+testSecret, nil)
	if err != nil {
		t.Fatal(err)
	}
	if err := d1.Sync(ctx, backend, pass); err != nil {
		t.Fatal(err)
	}

	// Fresh vault with a different salt.
	d2, err := Create(filepath.Join(dir, "d2.json"), pass)
	if err != nil {
		t.Fatal(err)
	}
	if err := d2.Sync(ctx, backend, pass); err != nil {
		t.Fatal(err)
	}
	b, err := d2.AddURI("otpauth://totp/AWS:bob?secret="+testSecret, nil)
	if err != nil {
		t.Fatal(err)
	}
	if err := d2.Sync(ctx, backend, pass); err != nil {
		t.Fatal(err)
	}

	if err := d1.Sync(ctx, backend, pass); err != nil {
		t.Fatal(err)
	}

	for _, dev := range []struct {
		name string
		s    *Session
	}{{"device1", d1}, {"device2", d2}} {
		if dev.s.vault.Get(a.ID) == nil {
			t.Errorf("%s lost account A (GitHub)", dev.name)
		}
		if dev.s.vault.Get(b.ID) == nil {
			t.Errorf("%s lost account B (AWS)", dev.name)
		}
		if n := len(dev.s.List()); n != 2 {
			t.Errorf("%s has %d accounts, want 2", dev.name, n)
		}
	}
}

// The stored blob must contain no plaintext secret/recovery code/label.
func TestSync_ZeroKnowledge(t *testing.T) {
	dir := t.TempDir()
	remote := filepath.Join(dir, "vault.blob")
	pass := []byte("pw")
	backend := sync.NewFS(remote)

	d, err := Create(filepath.Join(dir, "d.json"), pass)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := d.AddURI("otpauth://totp/Secret:user?secret="+testSecret, []string{"RECOVERY-CODE-XYZ"}); err != nil {
		t.Fatal(err)
	}
	if err := d.Sync(context.Background(), backend, pass); err != nil {
		t.Fatal(err)
	}

	blob, err := os.ReadFile(remote)
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(blob, []byte(testSecret)) {
		t.Error("OTP secret found in plaintext in synced blob")
	}
	if bytes.Contains(blob, []byte("RECOVERY-CODE-XYZ")) {
		t.Error("recovery code found in plaintext in synced blob")
	}
	if bytes.Contains(blob, []byte("user")) {
		t.Error("account label found in plaintext in synced blob")
	}
}
