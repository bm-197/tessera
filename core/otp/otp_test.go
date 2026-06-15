package otp

import (
	"testing"
	"time"
)

// rfcSecret is the 20-byte ASCII secret "12345678901234567890" used by the
// RFC 4226 / RFC 6238 test vectors, base32-encoded.
const rfcSecret = "GEZDGNBVGY3TQOJQGEZDGNBVGY3TQOJQ"

func TestHOTP_RFC4226Vectors(t *testing.T) {
	// RFC 4226 Appendix D, 6 digits, SHA1.
	want := []string{
		"755224", "287082", "359152", "969429", "338314",
		"254676", "287922", "162583", "399871", "520489",
	}
	for counter, exp := range want {
		p := Params{Type: HOTP, Algorithm: SHA1, Digits: 6, Counter: uint64(counter)}
		got, err := Generate(p, rfcSecret, time.Time{})
		if err != nil {
			t.Fatalf("counter %d: %v", counter, err)
		}
		if got.Value != exp {
			t.Errorf("counter %d: got %s want %s", counter, got.Value, exp)
		}
	}
}

func TestTOTP_RFC6238Vectors(t *testing.T) {
	// RFC 6238 Appendix B, 8 digits, SHA1, 30s period.
	cases := []struct {
		unix int64
		want string
	}{
		{59, "94287082"},
		{1111111109, "07081804"},
		{1111111111, "14050471"},
		{1234567890, "89005924"},
		{2000000000, "69279037"},
		{20000000000, "65353130"},
	}
	for _, c := range cases {
		p := Params{Type: TOTP, Algorithm: SHA1, Digits: 8, Period: 30}
		got, err := Generate(p, rfcSecret, time.Unix(c.unix, 0))
		if err != nil {
			t.Fatalf("unix %d: %v", c.unix, err)
		}
		if got.Value != c.want {
			t.Errorf("unix %d: got %s want %s", c.unix, got.Value, c.want)
		}
	}
}

func TestTOTP_ExpiresIn(t *testing.T) {
	p := Params{Type: TOTP, Algorithm: SHA1, Digits: 6, Period: 30}
	// 10s into a 30s window → 20s remaining.
	got, err := Generate(p, rfcSecret, time.Unix(10, 0))
	if err != nil {
		t.Fatal(err)
	}
	if got.ExpiresIn != 20*time.Second {
		t.Errorf("ExpiresIn = %v, want 20s", got.ExpiresIn)
	}
}

func TestVerify_TOTPWindow(t *testing.T) {
	p := Params{Type: TOTP, Algorithm: SHA1, Digits: 8, Period: 30}
	// Code generated for unix=1111111109 (step centered) should verify at a
	// neighboring step with skew=1.
	at := time.Unix(1111111109, 0)
	code, _ := Generate(p, rfcSecret, at)

	if !Verify(p, rfcSecret, code.Value, at.Add(30*time.Second), 1) {
		t.Error("expected code to verify within +1 step skew")
	}
	if Verify(p, rfcSecret, code.Value, at.Add(120*time.Second), 1) {
		t.Error("did not expect code to verify 4 steps away")
	}
	if Verify(p, rfcSecret, "00000000", at, 1) {
		t.Error("wrong code must not verify")
	}
}

func TestDecodeSecret_Tolerant(t *testing.T) {
	// Lowercase, spaces and missing padding should all decode the same.
	variants := []string{
		rfcSecret,
		"gezdgnbvgy3tqojqgezdgnbvgy3tqojq",
		"GEZD GNBV GY3T QOJQ GEZD GNBV GY3T QOJQ",
	}
	var first []byte
	for i, v := range variants {
		b, err := DecodeSecret(v)
		if err != nil {
			t.Fatalf("variant %d: %v", i, err)
		}
		if i == 0 {
			first = b
			continue
		}
		if string(b) != string(first) {
			t.Errorf("variant %d decoded differently", i)
		}
	}
}
