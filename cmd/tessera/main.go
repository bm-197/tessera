package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/bm-197/tessera/core/api"
	"github.com/bm-197/tessera/core/otp"
	"github.com/bm-197/tessera/core/sync"
	"github.com/bm-197/tessera/core/vault"
	"golang.org/x/term"
)

const usage = `tessera — end-to-end encrypted 2FA authenticator

Usage:
  tessera [--profile NAME] <command> [args]

Commands:
  init                          Create a new encrypted vault for the profile
  add <otpauth://...>           Add an account from an otpauth URI
  add --issuer I --secret S [--label L] [--digits N] [--period N] [--algo A]
  list                          List accounts with their current codes
  get <id|issuer>               Print the current code for one account
  recovery set <id> <code>...   Store recovery codes for an account
  recovery get <id>             Show stored recovery codes
  rm <id>                       Delete (tombstone) an account
  sync [--path FILE]            Merge with a filesystem backend (no-loss merge)

Global flags:
  --profile NAME   Vault profile / user (default "default")

Passphrase is read from $TESSERA_PASSPHRASE if set, otherwise prompted.
`

func main() {
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}

func run(args []string) error {
	profile := "default"

	var rest []string
	for i := 0; i < len(args); i++ {
		switch {
		case args[i] == "--profile" && i+1 < len(args):
			profile = args[i+1]
			i++
		case strings.HasPrefix(args[i], "--profile="):
			profile = strings.TrimPrefix(args[i], "--profile=")
		default:
			rest = append(rest, args[i])
		}
	}

	if len(rest) == 0 {
		fmt.Print(usage)
		return nil
	}

	cmd, cmdArgs := rest[0], rest[1:]
	switch cmd {
	case "help", "-h", "--help":
		fmt.Print(usage)
		return nil
	case "init":
		return cmdInit(profile)
	case "add":
		return cmdAdd(profile, cmdArgs)
	case "list", "ls":
		return cmdList(profile)
	case "get":
		return cmdGet(profile, cmdArgs)
	case "recovery":
		return cmdRecovery(profile, cmdArgs)
	case "rm", "delete":
		return cmdRemove(profile, cmdArgs)
	case "sync":
		return cmdSync(profile, cmdArgs)
	default:
		return fmt.Errorf("unknown command %q (try `tessera help`)", cmd)
	}
}

func profileDir(profile string) (string, error) {
	base, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(base, "tessera", profile), nil
}

func vaultPath(profile string) (string, error) {
	dir, err := profileDir(profile)
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "vault.json"), nil
}

type profileConfig struct {
	SyncPath string `json:"sync_path,omitempty"`
}

func configPath(profile string) (string, error) {
	dir, err := profileDir(profile)
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "config.json"), nil
}

func loadConfig(profile string) (profileConfig, error) {
	var c profileConfig
	p, err := configPath(profile)
	if err != nil {
		return c, err
	}
	data, err := os.ReadFile(p)
	if errors.Is(err, os.ErrNotExist) {
		return c, nil
	}
	if err != nil {
		return c, err
	}
	return c, json.Unmarshal(data, &c)
}

func saveConfig(profile string, c profileConfig) error {
	p, err := configPath(profile)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(p), 0o700); err != nil {
		return err
	}
	data, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(p, data, 0o600)
}

func readPassphrase(prompt string) ([]byte, error) {
	if env := os.Getenv("TESSERA_PASSPHRASE"); env != "" {
		return []byte(env), nil
	}
	if !term.IsTerminal(int(os.Stdin.Fd())) {
		return nil, errors.New("no terminal for passphrase prompt; set $TESSERA_PASSPHRASE")
	}
	fmt.Fprint(os.Stderr, prompt)
	pw, err := term.ReadPassword(int(os.Stdin.Fd()))
	fmt.Fprintln(os.Stderr)
	if err != nil {
		return nil, err
	}
	return pw, nil
}

func openSession(profile string) (*api.Session, []byte, error) {
	path, err := vaultPath(profile)
	if err != nil {
		return nil, nil, err
	}
	pass, err := readPassphrase("Passphrase: ")
	if err != nil {
		return nil, nil, err
	}
	s, err := api.Open(path, pass)
	if err != nil {
		return nil, nil, err
	}
	return s, pass, nil
}

func cmdInit(profile string) error {
	path, err := vaultPath(profile)
	if err != nil {
		return err
	}
	pass, err := readPassphrase("New passphrase: ")
	if err != nil {
		return err
	}
	confirm, err := readPassphrase("Confirm passphrase: ")
	if err != nil {
		return err
	}
	if string(pass) != string(confirm) {
		return errors.New("passphrases do not match")
	}
	s, err := api.Create(path, pass)
	if err != nil {
		return err
	}
	s.Lock()
	fmt.Printf("Initialized vault for profile %q at %s\n", profile, path)
	return nil
}

func cmdAdd(profile string, args []string) error {
	if len(args) == 0 {
		return errors.New("usage: tessera add <otpauth://...> | tessera add --issuer I --secret S [...]")
	}
	s, pass, err := openSession(profile)
	if err != nil {
		return err
	}
	defer func() { s.Lock(); zero(pass) }()

	var e *vault.Entry
	if strings.HasPrefix(args[0], "otpauth://") {
		e, err = s.AddURI(args[0], nil)
	} else {
		e, err = addManual(s, args)
	}
	if err != nil {
		return err
	}
	fmt.Printf("Added %s  (id %s)\n", displayName(e), e.ID)
	return nil
}

func addManual(s *api.Session, args []string) (*vault.Entry, error) {
	var issuer, label, secret, algo string
	params := otp.Defaults()
	for i := 0; i < len(args); i++ {
		next := func() (string, bool) {
			if i+1 < len(args) {
				i++
				return args[i], true
			}
			return "", false
		}
		switch args[i] {
		case "--issuer":
			issuer, _ = next()
		case "--label":
			label, _ = next()
		case "--secret":
			secret, _ = next()
		case "--algo":
			algo, _ = next()
		case "--digits":
			v, _ := next()
			fmt.Sscanf(v, "%d", &params.Digits)
		case "--period":
			v, _ := next()
			fmt.Sscanf(v, "%d", &params.Period)
		default:
			return nil, fmt.Errorf("unknown flag %q", args[i])
		}
	}
	if secret == "" {
		return nil, errors.New("--secret is required")
	}
	if algo != "" {
		params.Algorithm = otp.Algorithm(strings.ToUpper(algo))
	}
	return s.AddManual(issuer, label, secret, params, nil)
}

func cmdList(profile string) error {
	s, pass, err := openSession(profile)
	if err != nil {
		return err
	}
	defer func() { s.Lock(); zero(pass) }()

	codes, err := s.Codes()
	if err != nil {
		return err
	}
	if len(codes) == 0 {
		fmt.Println("No accounts. Add one with `tessera add otpauth://...`")
		return nil
	}
	for _, c := range codes {
		if c.Err != nil {
			fmt.Printf("%-28s  ERROR: %v\n", displayName(c.Entry), c.Err)
			continue
		}
		exp := ""
		if c.ExpiresIn > 0 {
			exp = fmt.Sprintf("  (%ds)", int(c.ExpiresIn/time.Second))
		}
		recov := ""
		if n := len(c.Entry.RecoveryCodes); n > 0 {
			recov = fmt.Sprintf("  [%d recovery codes]", n)
		}
		fmt.Printf("%-28s  %s%s%s\n", displayName(c.Entry), c.Code, exp, recov)
		fmt.Printf("%-28s  id %s\n", "", c.Entry.ID)
	}
	return nil
}

func cmdGet(profile string, args []string) error {
	if len(args) != 1 {
		return errors.New("usage: tessera get <id|issuer>")
	}
	s, pass, err := openSession(profile)
	if err != nil {
		return err
	}
	defer func() { s.Lock(); zero(pass) }()

	codes, err := s.Codes()
	if err != nil {
		return err
	}
	query := strings.ToLower(args[0])
	for _, c := range codes {
		if c.Entry.ID == args[0] || strings.ToLower(c.Entry.Issuer) == query {
			if c.Err != nil {
				return c.Err
			}
			fmt.Println(c.Code)
			return nil
		}
	}
	return fmt.Errorf("no account matching %q", args[0])
}

func cmdRecovery(profile string, args []string) error {
	if len(args) < 2 {
		return errors.New("usage: tessera recovery set <id> <code>... | tessera recovery get <id>")
	}
	s, pass, err := openSession(profile)
	if err != nil {
		return err
	}
	defer func() { s.Lock(); zero(pass) }()

	switch args[0] {
	case "set":
		id, codes := args[1], args[2:]
		if len(codes) == 0 {
			return errors.New("provide at least one recovery code")
		}
		if err := s.SetRecoveryCodes(id, codes); err != nil {
			return err
		}
		fmt.Printf("Stored %d recovery codes for %s\n", len(codes), id)
		return nil
	case "get":
		codes, err := s.RecoveryCodes(args[1])
		if err != nil {
			return err
		}
		if len(codes) == 0 {
			fmt.Println("(no recovery codes stored)")
			return nil
		}
		for _, c := range codes {
			fmt.Println(c)
		}
		return nil
	default:
		return fmt.Errorf("unknown recovery subcommand %q", args[0])
	}
}

func cmdRemove(profile string, args []string) error {
	if len(args) != 1 {
		return errors.New("usage: tessera rm <id>")
	}
	s, pass, err := openSession(profile)
	if err != nil {
		return err
	}
	defer func() { s.Lock(); zero(pass) }()

	if err := s.Delete(args[0]); err != nil {
		return err
	}
	fmt.Printf("Deleted %s\n", args[0])
	return nil
}

func cmdSync(profile string, args []string) error {
	cfg, err := loadConfig(profile)
	if err != nil {
		return err
	}
	syncPath := cfg.SyncPath
	for i := 0; i < len(args); i++ {
		if args[i] == "--path" && i+1 < len(args) {
			syncPath = args[i+1]
			i++
		} else if strings.HasPrefix(args[i], "--path=") {
			syncPath = strings.TrimPrefix(args[i], "--path=")
		}
	}
	if syncPath == "" {
		return errors.New("no sync path; pass --path FILE (it will be remembered)")
	}

	s, pass, err := openSession(profile)
	if err != nil {
		return err
	}
	defer func() { s.Lock(); zero(pass) }()

	backend := sync.NewFS(syncPath)
	if err := s.Sync(context.Background(), backend, pass); err != nil {
		return err
	}
	cfg.SyncPath = syncPath
	if err := saveConfig(profile, cfg); err != nil {
		return err
	}
	fmt.Printf("Synced with %s\n", syncPath)
	return nil
}

func displayName(e *vault.Entry) string {
	switch {
	case e.Issuer != "" && e.Label != "":
		return e.Issuer + " (" + e.Label + ")"
	case e.Issuer != "":
		return e.Issuer
	case e.Label != "":
		return e.Label
	default:
		return "(unnamed)"
	}
}

func zero(b []byte) {
	for i := range b {
		b[i] = 0
	}
}
