package bridge

import (
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"
)

const testSecret = "JBSWY3DPEHPK3PXP"

func call(t *testing.T, b *Bridge, method, params string) string {
	t.Helper()
	out, err := b.Call(method, params)
	if err != nil {
		t.Fatalf("%s: %v", method, err)
	}
	return out
}

func TestBridge_Lifecycle(t *testing.T) {
	path := filepath.Join(t.TempDir(), "vault.json")
	b := New()
	defer b.Close()

	// Locked until opened.
	if got := call(t, b, "status", ""); !strings.Contains(got, `"unlocked":false`) {
		t.Fatalf("status before open: %s", got)
	}
	if _, err := b.Call("list", ""); err == nil {
		t.Fatal("list should fail while locked")
	}

	// vaultExists is false, then create.
	if got := call(t, b, "vaultExists", `{"path":"`+path+`"}`); !strings.Contains(got, `"exists":false`) {
		t.Fatalf("vaultExists: %s", got)
	}
	call(t, b, "create", `{"path":"`+path+`","passphrase":"pw"}`)
	if got := call(t, b, "status", ""); !strings.Contains(got, `"unlocked":true`) {
		t.Fatalf("status after create: %s", got)
	}

	// Add via URI with recovery codes.
	addOut := call(t, b, "addURI", `{"uri":"otpauth://totp/GitHub:me?secret=`+testSecret+`&issuer=GitHub","recoveryCodes":["r1","r2"]}`)
	var added struct {
		Account struct {
			ID            string `json:"id"`
			Issuer        string `json:"issuer"`
			RecoveryCount int    `json:"recoveryCount"`
		} `json:"account"`
	}
	if err := json.Unmarshal([]byte(addOut), &added); err != nil {
		t.Fatal(err)
	}
	if added.Account.Issuer != "GitHub" || added.Account.RecoveryCount != 2 {
		t.Fatalf("added account wrong: %s", addOut)
	}

	// codes returns a 6-digit code and never leaks the secret.
	codesOut := call(t, b, "codes", "")
	if strings.Contains(codesOut, testSecret) {
		t.Fatal("codes output leaked the OTP secret")
	}
	var codes struct {
		Codes []struct {
			Code        string `json:"code"`
			ExpiresInMs int64  `json:"expiresInMs"`
		} `json:"codes"`
	}
	if err := json.Unmarshal([]byte(codesOut), &codes); err != nil {
		t.Fatal(err)
	}
	if len(codes.Codes) != 1 || len(codes.Codes[0].Code) != 6 || codes.Codes[0].ExpiresInMs <= 0 {
		t.Fatalf("codes wrong: %s", codesOut)
	}

	// recoveryCodes round-trips.
	recOut := call(t, b, "recoveryCodes", `{"id":"`+added.Account.ID+`"}`)
	if !strings.Contains(recOut, "r1") || !strings.Contains(recOut, "r2") {
		t.Fatalf("recoveryCodes: %s", recOut)
	}

	// rename, then verify via list.
	call(t, b, "setName", `{"id":"`+added.Account.ID+`","issuer":"GH","label":"alice"}`)
	if !strings.Contains(call(t, b, "list", ""), `"issuer":"GH"`) {
		t.Fatal("rename not reflected in list")
	}

	// lock clears state.
	call(t, b, "lock", "")
	if got := call(t, b, "status", ""); !strings.Contains(got, `"unlocked":false`) {
		t.Fatalf("status after lock: %s", got)
	}
}

func TestBridge_Sync(t *testing.T) {
	dir := t.TempDir()
	remote := filepath.Join(dir, "remote.blob")

	d1 := New()
	defer d1.Close()
	call(t, d1, "create", `{"path":"`+filepath.Join(dir, "d1.json")+`","passphrase":"pw"}`)
	call(t, d1, "addURI", `{"uri":"otpauth://totp/A:x?secret=`+testSecret+`&issuer=A"}`)
	call(t, d1, "sync", `{"path":"`+remote+`"}`)

	d2 := New()
	defer d2.Close()
	call(t, d2, "create", `{"path":"`+filepath.Join(dir, "d2.json")+`","passphrase":"pw"}`)
	call(t, d2, "addURI", `{"uri":"otpauth://totp/B:y?secret=`+testSecret+`&issuer=B"}`)
	call(t, d2, "sync", `{"path":"`+remote+`"}`)
	call(t, d1, "sync", `{"path":"`+remote+`"}`)

	out := call(t, d1, "list", "")
	if !strings.Contains(out, `"issuer":"A"`) || !strings.Contains(out, `"issuer":"B"`) {
		t.Fatalf("sync did not merge both accounts: %s", out)
	}
}

func TestBridge_UnknownMethod(t *testing.T) {
	b := New()
	if _, err := b.Call("nope", ""); err == nil {
		t.Fatal("unknown method should error")
	}
}
