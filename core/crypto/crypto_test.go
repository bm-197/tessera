package crypto

import (
	"bytes"
	"testing"
)

func TestEnvelopeRoundTrip(t *testing.T) {
	pass := []byte("correct horse battery staple")
	salt, err := NewSalt()
	if err != nil {
		t.Fatal(err)
	}
	kdf := DefaultKDFParams()
	key := DeriveKey(pass, salt, kdf)

	plaintext := []byte(`{"entries":{"x":"secret-totp-seed"}}`)
	env, err := SealEnvelope(key, salt, kdf, plaintext)
	if err != nil {
		t.Fatal(err)
	}
	got, err := OpenEnvelope(key, env)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, plaintext) {
		t.Fatalf("round-trip mismatch: %q", got)
	}
}

func TestWrongPassphraseFails(t *testing.T) {
	salt, _ := NewSalt()
	kdf := DefaultKDFParams()
	good := DeriveKey([]byte("right"), salt, kdf)
	bad := DeriveKey([]byte("wrong"), salt, kdf)

	env, err := SealEnvelope(good, salt, kdf, []byte("top secret"))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := OpenEnvelope(bad, env); err != ErrDecrypt {
		t.Fatalf("want ErrDecrypt, got %v", err)
	}
}

func TestTamperDetected(t *testing.T) {
	salt, _ := NewSalt()
	kdf := DefaultKDFParams()
	key := DeriveKey([]byte("pw"), salt, kdf)
	env, _ := SealEnvelope(key, salt, kdf, []byte("authenticated data"))

	// Flip a bit in the ciphertext: GCM's tag must reject it.
	env.Cipher[0] ^= 0x01
	if _, err := OpenEnvelope(key, env); err != ErrDecrypt {
		t.Fatalf("tampered ciphertext must fail with ErrDecrypt, got %v", err)
	}
}

func TestNonceUniquePerSeal(t *testing.T) {
	salt, _ := NewSalt()
	kdf := DefaultKDFParams()
	key := DeriveKey([]byte("pw"), salt, kdf)

	e1, _ := SealEnvelope(key, salt, kdf, []byte("same plaintext"))
	e2, _ := SealEnvelope(key, salt, kdf, []byte("same plaintext"))
	if bytes.Equal(e1.Nonce, e2.Nonce) {
		t.Fatal("nonce reused across seals — GCM security broken")
	}
	if bytes.Equal(e1.Cipher, e2.Cipher) {
		t.Fatal("identical ciphertext for same plaintext — nonce not random")
	}
}

func TestEqualConstantTime(t *testing.T) {
	if !Equal([]byte("abc"), []byte("abc")) {
		t.Error("equal slices reported unequal")
	}
	if Equal([]byte("abc"), []byte("abd")) {
		t.Error("unequal slices reported equal")
	}
	if Equal([]byte("abc"), []byte("abcd")) {
		t.Error("different-length slices reported equal")
	}
}
