package sync

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
)

// FS stores the encrypted blob in a single file, optionally inside a folder
// replicated by another tool (Dropbox, Syncthing, ...). The version token is
// the SHA-256 of the contents. Its compare-and-swap has a TOCTOU window and is
// not race-free against simultaneous writers on one host; strong concurrency is
// the server backend's job.
type FS struct {
	path string
}

func NewFS(path string) *FS {
	return &FS{path: path}
}

func (f *FS) Get(ctx context.Context) ([]byte, string, error) {
	data, err := os.ReadFile(f.path)
	if errors.Is(err, fs.ErrNotExist) {
		return nil, "", nil
	}
	if err != nil {
		return nil, "", fmt.Errorf("sync/fs: read %s: %w", f.path, err)
	}
	return data, version(data), nil
}

func (f *FS) Put(ctx context.Context, blob []byte, expectedVersion string) (string, error) {
	current, _, err := f.Get(ctx)
	if err != nil {
		return "", err
	}
	currentVersion := ""
	if len(current) > 0 {
		currentVersion = version(current)
	}
	if currentVersion != expectedVersion {
		return "", ErrConflict
	}

	if dir := filepath.Dir(f.path); dir != "" {
		if err := os.MkdirAll(dir, 0o700); err != nil {
			return "", fmt.Errorf("sync/fs: mkdir %s: %w", dir, err)
		}
	}
	tmp, err := os.CreateTemp(filepath.Dir(f.path), ".tessera-*.tmp")
	if err != nil {
		return "", fmt.Errorf("sync/fs: temp file: %w", err)
	}
	tmpName := tmp.Name()
	defer os.Remove(tmpName)

	if err := tmp.Chmod(0o600); err != nil {
		tmp.Close()
		return "", fmt.Errorf("sync/fs: chmod temp: %w", err)
	}
	if _, err := tmp.Write(blob); err != nil {
		tmp.Close()
		return "", fmt.Errorf("sync/fs: write temp: %w", err)
	}
	if err := tmp.Sync(); err != nil {
		tmp.Close()
		return "", fmt.Errorf("sync/fs: fsync temp: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return "", fmt.Errorf("sync/fs: close temp: %w", err)
	}
	if err := os.Rename(tmpName, f.path); err != nil {
		return "", fmt.Errorf("sync/fs: rename: %w", err)
	}
	return version(blob), nil
}

func version(b []byte) string {
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:])
}
