package api

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"time"

	"github.com/bm-197/tessera/core/crypto"
	"github.com/bm-197/tessera/core/otp"
	"github.com/bm-197/tessera/core/sync"
	"github.com/bm-197/tessera/core/vault"
)

type Session struct {
	path  string
	key   []byte
	salt  []byte
	kdf   crypto.KDFParams
	vault *vault.Vault

	now func() time.Time
}

var ErrVaultExists = errors.New("api: vault already exists at path")
var ErrNoVault = errors.New("api: no vault at path")

func Create(path string, passphrase []byte) (*Session, error) {
	if _, err := os.Stat(path); err == nil {
		return nil, ErrVaultExists
	} else if !errors.Is(err, fs.ErrNotExist) {
		return nil, fmt.Errorf("api: stat vault: %w", err)
	}

	salt, err := crypto.NewSalt()
	if err != nil {
		return nil, err
	}
	kdf := crypto.DefaultKDFParams()
	s := &Session{
		path:  path,
		key:   crypto.DeriveKey(passphrase, salt, kdf),
		salt:  salt,
		kdf:   kdf,
		vault: vault.New(),
		now:   time.Now,
	}
	if err := s.save(); err != nil {
		return nil, err
	}
	return s, nil
}

func Open(path string, passphrase []byte) (*Session, error) {
	env, err := readEnvelope(path)
	if err != nil {
		return nil, err
	}
	key := crypto.DeriveKey(passphrase, env.Salt, env.KDF)
	plaintext, err := crypto.OpenEnvelope(key, env)
	if err != nil {
		crypto.Zero(key)
		return nil, err
	}
	v, err := vault.Decode(plaintext)
	crypto.Zero(plaintext)
	if err != nil {
		crypto.Zero(key)
		return nil, err
	}
	return &Session{
		path:  path,
		key:   key,
		salt:  env.Salt,
		kdf:   env.KDF,
		vault: v,
		now:   time.Now,
	}, nil
}

func (s *Session) Lock() {
	crypto.Zero(s.key)
	s.key = nil
	s.vault = nil
}

func (s *Session) AddURI(uri string, recoveryCodes []string) (*vault.Entry, error) {
	acc, err := otp.ParseURI(uri)
	if err != nil {
		return nil, err
	}
	e := s.vault.Add(*acc, recoveryCodes, s.now())
	if err := s.save(); err != nil {
		return nil, err
	}
	return e, nil
}

func (s *Session) AddManual(issuer, label, secret string, params otp.Params, recoveryCodes []string) (*vault.Entry, error) {
	if _, err := otp.DecodeSecret(secret); err != nil {
		return nil, err
	}
	acc := otp.Account{Issuer: issuer, Label: label, Secret: secret, Params: params}
	e := s.vault.Add(acc, recoveryCodes, s.now())
	if err := s.save(); err != nil {
		return nil, err
	}
	return e, nil
}

func (s *Session) SetRecoveryCodes(id string, codes []string) error {
	e := s.vault.Get(id)
	if e == nil || e.Deleted {
		return fmt.Errorf("api: no account %q", id)
	}
	e.SetRecoveryCodes(codes, s.now())
	return s.save()
}

func (s *Session) RecoveryCodes(id string) ([]string, error) {
	e := s.vault.Get(id)
	if e == nil || e.Deleted {
		return nil, fmt.Errorf("api: no account %q", id)
	}
	return e.RecoveryCodes, nil
}

// SetName updates an account's issuer/label and persists.
func (s *Session) SetName(id, issuer, label string) error {
	e := s.vault.Get(id)
	if e == nil || e.Deleted {
		return fmt.Errorf("api: no account %q", id)
	}
	e.SetLabel(issuer, label, s.now())
	return s.save()
}

func (s *Session) Delete(id string) error {
	e := s.vault.Get(id)
	if e == nil || e.Deleted {
		return fmt.Errorf("api: no account %q", id)
	}
	e.Delete(s.now())
	return s.save()
}

func (s *Session) List() []*vault.Entry {
	return s.vault.Live()
}

type LiveCode struct {
	Entry     *vault.Entry
	Code      string
	ExpiresIn time.Duration
	Err       error
}

func (s *Session) Codes() ([]LiveCode, error) {
	now := s.now()
	entries := s.vault.Live()
	out := make([]LiveCode, 0, len(entries))
	hotpAdvanced := false

	for _, e := range entries {
		code, err := otp.Generate(e.Params, e.Secret, now)
		lc := LiveCode{Entry: e}
		if err != nil {
			lc.Err = err
			out = append(out, lc)
			continue
		}
		lc.Code = code.Value
		lc.ExpiresIn = code.ExpiresIn
		out = append(out, lc)

		if e.Params.Type == otp.HOTP {
			p := e.Params
			p.Counter++
			e.SetParams(p, now)
			hotpAdvanced = true
		}
	}
	if hotpAdvanced {
		if err := s.save(); err != nil {
			return out, err
		}
	}
	return out, nil
}

// RenderCodes returns current codes for display only: it has no side effects
// and does not advance HOTP counters, so it is safe to call repeatedly (e.g.
// once a second from a TUI).
func (s *Session) RenderCodes() []LiveCode {
	now := s.now()
	entries := s.vault.Live()
	out := make([]LiveCode, 0, len(entries))
	for _, e := range entries {
		lc := LiveCode{Entry: e}
		code, err := otp.Generate(e.Params, e.Secret, now)
		if err != nil {
			lc.Err = err
		} else {
			lc.Code = code.Value
			lc.ExpiresIn = code.ExpiresIn
		}
		out = append(out, lc)
	}
	return out
}

func (s *Session) Sync(ctx context.Context, backend sync.Backend, passphrase []byte) error {
	const maxAttempts = 5
	for attempt := 0; attempt < maxAttempts; attempt++ {
		blob, remoteVersion, err := backend.Get(ctx)
		if err != nil {
			return err
		}

		remote := vault.New()
		if len(blob) > 0 {
			env := &crypto.Envelope{}
			if err := json.Unmarshal(blob, env); err != nil {
				return fmt.Errorf("api: remote envelope: %w", err)
			}
			rkey := crypto.DeriveKey(passphrase, env.Salt, env.KDF)
			plaintext, err := crypto.OpenEnvelope(rkey, env)
			crypto.Zero(rkey)
			if err != nil {
				return fmt.Errorf("api: decrypting remote vault: %w", err)
			}
			remote, err = vault.Decode(plaintext)
			crypto.Zero(plaintext)
			if err != nil {
				return err
			}
		}

		merged := vault.Merge(s.vault, remote)
		s.vault = merged
		if err := s.save(); err != nil {
			return err
		}

		sealed, err := s.sealedBlob()
		if err != nil {
			return err
		}
		if _, err := backend.Put(ctx, sealed, remoteVersion); err != nil {
			if errors.Is(err, sync.ErrConflict) {
				continue // someone wrote between Get and Put; re-merge and retry
			}
			return err
		}
		return nil
	}
	return fmt.Errorf("api: sync failed after %d attempts due to repeated conflicts", maxAttempts)
}

func (s *Session) save() error {
	sealed, err := s.sealedBlob()
	if err != nil {
		return err
	}
	return writeAtomic(s.path, sealed)
}

func (s *Session) sealedBlob() ([]byte, error) {
	plaintext, err := s.vault.Encode()
	if err != nil {
		return nil, err
	}
	env, err := crypto.SealEnvelope(s.key, s.salt, s.kdf, plaintext)
	crypto.Zero(plaintext)
	if err != nil {
		return nil, err
	}
	return json.Marshal(env)
}

func readEnvelope(path string) (*crypto.Envelope, error) {
	data, err := os.ReadFile(path)
	if errors.Is(err, fs.ErrNotExist) {
		return nil, ErrNoVault
	}
	if err != nil {
		return nil, fmt.Errorf("api: read vault: %w", err)
	}
	env := &crypto.Envelope{}
	if err := json.Unmarshal(data, env); err != nil {
		return nil, fmt.Errorf("api: parse vault: %w", err)
	}
	return env, nil
}

func writeAtomic(path string, data []byte) error {
	if dir := filepath.Dir(path); dir != "" {
		if err := os.MkdirAll(dir, 0o700); err != nil {
			return fmt.Errorf("api: mkdir %s: %w", dir, err)
		}
	}
	tmp, err := os.CreateTemp(filepath.Dir(path), ".tessera-*.tmp")
	if err != nil {
		return fmt.Errorf("api: temp file: %w", err)
	}
	tmpName := tmp.Name()
	defer os.Remove(tmpName)

	if err := tmp.Chmod(0o600); err != nil {
		tmp.Close()
		return err
	}
	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Sync(); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return os.Rename(tmpName, path)
}
