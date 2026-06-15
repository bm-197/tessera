package vault

import (
	"crypto/rand"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/bm-197/tessera/core/otp"
)

// Field names used as keys in Entry.Modified for per-field LWW.
const (
	FieldIssuer   = "issuer"
	FieldLabel    = "label"
	FieldSecret   = "secret"
	FieldRecovery = "recovery_codes"
	FieldParams   = "otp_params"
	FieldDeleted  = "deleted"
)

type Entry struct {
	ID            string     `json:"id"`
	Issuer        string     `json:"issuer"`
	Label         string     `json:"label"`
	Secret        string     `json:"secret"`
	RecoveryCodes []string   `json:"recovery_codes"`
	Params        otp.Params `json:"otp_params"`
	Deleted       bool       `json:"deleted"`
	CreatedAt     time.Time  `json:"created_at"`
	ModifiedAt    time.Time  `json:"modified_at"`

	// Modified holds the last-write time per field (Field* keys) for merge.
	Modified map[string]time.Time `json:"modified"`
}

type Vault struct {
	Entries map[string]*Entry `json:"entries"`
}

func New() *Vault {
	return &Vault{Entries: map[string]*Entry{}}
}

func Decode(b []byte) (*Vault, error) {
	v := New()
	if len(b) == 0 {
		return v, nil
	}
	if err := json.Unmarshal(b, v); err != nil {
		return nil, fmt.Errorf("vault: decode: %w", err)
	}
	if v.Entries == nil {
		v.Entries = map[string]*Entry{}
	}
	return v, nil
}

func (v *Vault) Encode() ([]byte, error) {
	return json.Marshal(v)
}

func NewID() string {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		panic(fmt.Sprintf("vault: cannot read randomness for UUID: %v", err))
	}
	b[6] = (b[6] & 0x0f) | 0x40
	b[8] = (b[8] & 0x3f) | 0x80
	return fmt.Sprintf("%x-%x-%x-%x-%x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:16])
}

func (v *Vault) Add(acc otp.Account, recovery []string, t time.Time) *Entry {
	e := &Entry{
		ID:            NewID(),
		Issuer:        acc.Issuer,
		Label:         acc.Label,
		Secret:        acc.Secret,
		RecoveryCodes: recovery,
		Params:        acc.Params,
		CreatedAt:     t,
		ModifiedAt:    t,
		Modified: map[string]time.Time{
			FieldIssuer:   t,
			FieldLabel:    t,
			FieldSecret:   t,
			FieldRecovery: t,
			FieldParams:   t,
			FieldDeleted:  t,
		},
	}
	v.Entries[e.ID] = e
	return e
}

func (v *Vault) Get(id string) *Entry {
	return v.Entries[id]
}

func (v *Vault) Live() []*Entry {
	out := make([]*Entry, 0, len(v.Entries))
	for _, e := range v.Entries {
		if !e.Deleted {
			out = append(out, e)
		}
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Issuer != out[j].Issuer {
			return strings.ToLower(out[i].Issuer) < strings.ToLower(out[j].Issuer)
		}
		if out[i].Label != out[j].Label {
			return strings.ToLower(out[i].Label) < strings.ToLower(out[j].Label)
		}
		return out[i].ID < out[j].ID
	})
	return out
}

func (e *Entry) touch(field string, t time.Time) {
	if e.Modified == nil {
		e.Modified = map[string]time.Time{}
	}
	e.Modified[field] = t
	if t.After(e.ModifiedAt) {
		e.ModifiedAt = t
	}
}

func (e *Entry) SetRecoveryCodes(codes []string, t time.Time) {
	e.RecoveryCodes = codes
	e.touch(FieldRecovery, t)
}

func (e *Entry) SetSecret(secret string, t time.Time) {
	e.Secret = secret
	e.touch(FieldSecret, t)
}

func (e *Entry) SetLabel(issuer, label string, t time.Time) {
	e.Issuer = issuer
	e.Label = label
	e.touch(FieldIssuer, t)
	e.touch(FieldLabel, t)
}

func (e *Entry) SetParams(p otp.Params, t time.Time) {
	e.Params = p
	e.touch(FieldParams, t)
}

// Delete tombstones the entry; it is retained so the deletion propagates and is
// not undone by a stale copy on another device.
func (e *Entry) Delete(t time.Time) {
	e.Deleted = true
	e.touch(FieldDeleted, t)
}

func (e *Entry) Clone() *Entry {
	c := *e
	if e.RecoveryCodes != nil {
		c.RecoveryCodes = append([]string(nil), e.RecoveryCodes...)
	}
	c.Modified = make(map[string]time.Time, len(e.Modified))
	for k, v := range e.Modified {
		c.Modified[k] = v
	}
	return &c
}
