package otp

import "testing"

func TestParseURI_TOTP(t *testing.T) {
	uri := "otpauth://totp/Example:alice@example.com?secret=" + rfcSecret +
		"&issuer=Example&algorithm=SHA256&digits=8&period=60"
	acc, err := ParseURI(uri)
	if err != nil {
		t.Fatal(err)
	}
	if acc.Issuer != "Example" {
		t.Errorf("issuer = %q", acc.Issuer)
	}
	if acc.Label != "alice@example.com" {
		t.Errorf("label = %q", acc.Label)
	}
	if acc.Params.Type != TOTP || acc.Params.Algorithm != SHA256 ||
		acc.Params.Digits != 8 || acc.Params.Period != 60 {
		t.Errorf("params = %+v", acc.Params)
	}
}

func TestParseURI_Defaults(t *testing.T) {
	acc, err := ParseURI("otpauth://totp/alice?secret=" + rfcSecret)
	if err != nil {
		t.Fatal(err)
	}
	if acc.Issuer != "" || acc.Label != "alice" {
		t.Errorf("issuer=%q label=%q", acc.Issuer, acc.Label)
	}
	if acc.Params.Algorithm != SHA1 || acc.Params.Digits != 6 || acc.Params.Period != 30 {
		t.Errorf("defaults not applied: %+v", acc.Params)
	}
}

func TestParseURI_Errors(t *testing.T) {
	bad := []string{
		"https://example.com",                     // wrong scheme
		"otpauth://foo/alice?secret=" + rfcSecret, // bad type
		"otpauth://totp/alice",                    // missing secret
		"otpauth://totp/alice?secret=not!base32!", // bad secret
	}
	for _, uri := range bad {
		if _, err := ParseURI(uri); err == nil {
			t.Errorf("expected error for %q", uri)
		}
	}
}

func TestURI_RoundTrip(t *testing.T) {
	orig := "otpauth://hotp/Acme:bob?secret=" + rfcSecret + "&issuer=Acme&counter=7"
	acc, err := ParseURI(orig)
	if err != nil {
		t.Fatal(err)
	}
	if acc.Params.Type != HOTP || acc.Params.Counter != 7 {
		t.Fatalf("params = %+v", acc.Params)
	}
	// Re-parse the rendered URI and confirm fields survive.
	acc2, err := ParseURI(ToURI(*acc))
	if err != nil {
		t.Fatal(err)
	}
	if acc2.Issuer != acc.Issuer || acc2.Label != acc.Label ||
		acc2.Secret != acc.Secret || acc2.Params != acc.Params {
		t.Errorf("round-trip mismatch:\n %+v\n %+v", acc, acc2)
	}
}
