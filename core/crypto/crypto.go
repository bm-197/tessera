package crypto

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/subtle"
	"errors"
	"fmt"

	"golang.org/x/crypto/argon2"
)

type KDFParams struct {
	Time    uint32 `json:"time"`
	Memory  uint32 `json:"memory"`
	Threads uint8  `json:"threads"`
	KeyLen  uint32 `json:"keyLen"`
}

// DefaultKDFParams follows the OWASP Argon2id baseline: 3 passes over 64 MiB.
func DefaultKDFParams() KDFParams {
	return KDFParams{Time: 3, Memory: 64 * 1024, Threads: 4, KeyLen: KeyLen}
}

const (
	KeyLen          = 32
	SaltLen         = 16
	nonceLen        = 12
	EnvelopeVersion = 1
)

// ErrDecrypt carries no detail so wrong-passphrase and tampering are not
// distinguishable to the caller (no oracle).
var ErrDecrypt = errors.New("crypto: cannot decrypt (wrong passphrase or corrupt data)")

// Envelope is the encrypted vault as stored on disk and over any sync backend:
// salt, KDF params, nonce and ciphertext only — never plaintext or the key.
type Envelope struct {
	Version int       `json:"version"`
	KDF     KDFParams `json:"kdf"`
	Salt    []byte    `json:"salt"`
	Nonce   []byte    `json:"nonce"`
	Cipher  []byte    `json:"cipher"`
}

func DeriveKey(passphrase, salt []byte, p KDFParams) []byte {
	return argon2.IDKey(passphrase, salt, p.Time, p.Memory, p.Threads, p.KeyLen)
}

func NewSalt() ([]byte, error) {
	salt := make([]byte, SaltLen)
	if _, err := rand.Read(salt); err != nil {
		return nil, fmt.Errorf("crypto: generating salt: %w", err)
	}
	return salt, nil
}

// SealEnvelope generates a fresh random nonce on every call so a key never
// reuses one (a hard GCM requirement).
func SealEnvelope(key, salt []byte, kdf KDFParams, plaintext []byte) (*Envelope, error) {
	gcm, err := newGCM(key)
	if err != nil {
		return nil, err
	}
	nonce := make([]byte, nonceLen)
	if _, err := rand.Read(nonce); err != nil {
		return nil, fmt.Errorf("crypto: generating nonce: %w", err)
	}
	ciphertext := gcm.Seal(nil, nonce, plaintext, nil)
	return &Envelope{
		Version: EnvelopeVersion,
		KDF:     kdf,
		Salt:    salt,
		Nonce:   nonce,
		Cipher:  ciphertext,
	}, nil
}

func OpenEnvelope(key []byte, e *Envelope) ([]byte, error) {
	if e == nil {
		return nil, ErrDecrypt
	}
	gcm, err := newGCM(key)
	if err != nil {
		return nil, err
	}
	if len(e.Nonce) != gcm.NonceSize() {
		return nil, ErrDecrypt
	}
	plaintext, err := gcm.Open(nil, e.Nonce, e.Cipher, nil)
	if err != nil {
		return nil, ErrDecrypt
	}
	return plaintext, nil
}

func newGCM(key []byte) (cipher.AEAD, error) {
	if len(key) != KeyLen {
		return nil, fmt.Errorf("crypto: key must be %d bytes, got %d", KeyLen, len(key))
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("crypto: aes cipher: %w", err)
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("crypto: gcm: %w", err)
	}
	return gcm, nil
}

// Equal compares secret material in constant time.
func Equal(a, b []byte) bool {
	return subtle.ConstantTimeCompare(a, b) == 1
}

func EqualString(a, b string) bool {
	return subtle.ConstantTimeCompare([]byte(a), []byte(b)) == 1
}

// Zero is best-effort; the GC may already have copied the bytes.
func Zero(b []byte) {
	for i := range b {
		b[i] = 0
	}
}
