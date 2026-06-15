package otp

import (
	"fmt"
	"net/url"
	"strconv"
	"strings"
)

type Account struct {
	Issuer string
	Label  string
	Secret string
	Params Params
}

func ParseURI(uri string) (*Account, error) {
	u, err := url.Parse(strings.TrimSpace(uri))
	if err != nil {
		return nil, fmt.Errorf("otp: parsing uri: %w", err)
	}
	if !strings.EqualFold(u.Scheme, "otpauth") {
		return nil, fmt.Errorf("otp: not an otpauth uri (scheme %q)", u.Scheme)
	}

	kind := Kind(strings.ToLower(u.Host))
	if kind != TOTP && kind != HOTP {
		return nil, fmt.Errorf("otp: unsupported otp type %q", u.Host)
	}

	label := strings.TrimPrefix(u.Path, "/")
	var issuer, account string
	if i := strings.Index(label, ":"); i >= 0 {
		issuer = strings.TrimSpace(label[:i])
		account = strings.TrimSpace(label[i+1:])
	} else {
		account = label
	}

	q := u.Query()
	secret := q.Get("secret")
	if secret == "" {
		return nil, fmt.Errorf("otp: uri missing required secret")
	}
	if _, err := DecodeSecret(secret); err != nil {
		return nil, err
	}
	if qi := q.Get("issuer"); qi != "" {
		issuer = qi // query issuer is authoritative over the label prefix
	}

	p := Params{Type: kind}
	if alg := q.Get("algorithm"); alg != "" {
		p.Algorithm = Algorithm(strings.ToUpper(alg))
		if _, err := p.Algorithm.newHash(); err != nil {
			return nil, err
		}
	}
	if ds := q.Get("digits"); ds != "" {
		d, err := strconv.Atoi(ds)
		if err != nil || d < 6 || d > 10 {
			return nil, fmt.Errorf("otp: invalid digits %q", ds)
		}
		p.Digits = d
	}
	if ps := q.Get("period"); ps != "" {
		v, err := strconv.Atoi(ps)
		if err != nil || v <= 0 {
			return nil, fmt.Errorf("otp: invalid period %q", ps)
		}
		p.Period = v
	}
	if cs := q.Get("counter"); cs != "" {
		v, err := strconv.ParseUint(cs, 10, 64)
		if err != nil {
			return nil, fmt.Errorf("otp: invalid counter %q", cs)
		}
		p.Counter = v
	}

	return &Account{
		Issuer: issuer,
		Label:  account,
		Secret: secret,
		Params: p.normalize(),
	}, nil
}

// ToURI embeds the secret in plaintext; callers must treat the result as
// sensitive (e.g. QR export) and never log it.
func ToURI(a Account) string {
	p := a.Params.normalize()
	label := a.Label
	if a.Issuer != "" {
		label = a.Issuer + ":" + a.Label
	}

	q := url.Values{}
	q.Set("secret", a.Secret)
	if a.Issuer != "" {
		q.Set("issuer", a.Issuer)
	}
	q.Set("algorithm", string(p.Algorithm))
	q.Set("digits", strconv.Itoa(p.Digits))
	if p.Type == HOTP {
		q.Set("counter", strconv.FormatUint(p.Counter, 10))
	} else {
		q.Set("period", strconv.Itoa(p.Period))
	}

	u := url.URL{
		Scheme:   "otpauth",
		Host:     string(p.Type),
		Path:     "/" + label,
		RawQuery: q.Encode(),
	}
	return u.String()
}
