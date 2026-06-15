package otp

import (
	"crypto/hmac"
	"crypto/sha1"
	"crypto/sha256"
	"crypto/sha512"
	"encoding/base32"
	"encoding/binary"
	"fmt"
	"hash"
	"strings"
	"time"

	"github.com/bm-197/tessera/core/crypto"
)

type Kind string

const (
	TOTP Kind = "totp"
	HOTP Kind = "hotp"
)

type Algorithm string

const (
	SHA1   Algorithm = "SHA1"
	SHA256 Algorithm = "SHA256"
	SHA512 Algorithm = "SHA512"
)

type Params struct {
	Type      Kind      `json:"type"`
	Algorithm Algorithm `json:"algorithm"`
	Digits    int       `json:"digits"`
	Period    int       `json:"period"`
	Counter   uint64    `json:"counter"`
}

func Defaults() Params {
	return Params{Type: TOTP, Algorithm: SHA1, Digits: 6, Period: 30}
}

func (p Params) normalize() Params {
	if p.Type == "" {
		p.Type = TOTP
	}
	if p.Algorithm == "" {
		p.Algorithm = SHA1
	}
	if p.Digits == 0 {
		p.Digits = 6
	}
	if p.Period == 0 {
		p.Period = 30
	}
	return p
}

func (a Algorithm) newHash() (func() hash.Hash, error) {
	switch Algorithm(strings.ToUpper(string(a))) {
	case SHA1, "":
		return sha1.New, nil
	case SHA256:
		return sha256.New, nil
	case SHA512:
		return sha512.New, nil
	default:
		return nil, fmt.Errorf("otp: unsupported algorithm %q", a)
	}
}

// DecodeSecret tolerates lowercase, spaces and missing padding.
func DecodeSecret(secret string) ([]byte, error) {
	s := strings.ToUpper(strings.TrimSpace(secret))
	s = strings.ReplaceAll(s, " ", "")
	s = strings.ReplaceAll(s, "-", "")
	if pad := len(s) % 8; pad != 0 {
		s += strings.Repeat("=", 8-pad)
	}
	b, err := base32.StdEncoding.DecodeString(s)
	if err != nil {
		return nil, fmt.Errorf("otp: invalid base32 secret: %w", err)
	}
	return b, nil
}

func hotp(secret []byte, counter uint64, digits int, newHash func() hash.Hash) string {
	var buf [8]byte
	binary.BigEndian.PutUint64(buf[:], counter)

	mac := hmac.New(newHash, secret)
	mac.Write(buf[:])
	sum := mac.Sum(nil)

	offset := sum[len(sum)-1] & 0x0f
	code := (uint32(sum[offset]&0x7f) << 24) |
		(uint32(sum[offset+1]) << 16) |
		(uint32(sum[offset+2]) << 8) |
		uint32(sum[offset+3])

	code %= pow10(digits)
	return fmt.Sprintf("%0*d", digits, code)
}

func pow10(n int) uint32 {
	p := uint32(1)
	for i := 0; i < n; i++ {
		p *= 10
	}
	return p
}

func counterFor(p Params, t time.Time) uint64 {
	return uint64(t.Unix() / int64(p.Period))
}

type Code struct {
	Value     string
	ExpiresIn time.Duration
}

func Generate(p Params, secret string, t time.Time) (Code, error) {
	p = p.normalize()
	newHash, err := p.Algorithm.newHash()
	if err != nil {
		return Code{}, err
	}
	key, err := DecodeSecret(secret)
	if err != nil {
		return Code{}, err
	}

	switch p.Type {
	case HOTP:
		return Code{Value: hotp(key, p.Counter, p.Digits, newHash)}, nil
	case TOTP, "":
		counter := counterFor(p, t)
		value := hotp(key, counter, p.Digits, newHash)
		elapsed := t.Unix() % int64(p.Period)
		return Code{
			Value:     value,
			ExpiresIn: time.Duration(int64(p.Period)-elapsed) * time.Second,
		}, nil
	default:
		return Code{}, fmt.Errorf("otp: unsupported type %q", p.Type)
	}
}

// Verify uses constant-time comparison and tolerates skew steps on either side.
func Verify(p Params, secret, code string, t time.Time, skew int) bool {
	p = p.normalize()
	newHash, err := p.Algorithm.newHash()
	if err != nil {
		return false
	}
	key, err := DecodeSecret(secret)
	if err != nil {
		return false
	}
	if skew < 0 {
		skew = 0
	}

	switch p.Type {
	case HOTP:
		for i := 0; i <= skew; i++ {
			if crypto.EqualString(code, hotp(key, p.Counter+uint64(i), p.Digits, newHash)) {
				return true
			}
		}
	default:
		base := counterFor(p, t)
		for d := -skew; d <= skew; d++ {
			counter := uint64(int64(base) + int64(d))
			if crypto.EqualString(code, hotp(key, counter, p.Digits, newHash)) {
				return true
			}
		}
	}
	return false
}
