package sync

import (
	"bytes"
	"context"
	"errors"
	"path/filepath"
	"testing"
)

func TestFS_PutGetRoundTrip(t *testing.T) {
	path := filepath.Join(t.TempDir(), "sub", "blob")
	b := NewFS(path)
	ctx := context.Background()

	// Empty backend.
	data, ver, err := b.Get(ctx)
	if err != nil || data != nil || ver != "" {
		t.Fatalf("empty get: data=%v ver=%q err=%v", data, ver, err)
	}

	// First write expects empty version.
	v1, err := b.Put(ctx, []byte("blob-1"), "")
	if err != nil {
		t.Fatal(err)
	}
	got, ver, err := b.Get(ctx)
	if err != nil || !bytes.Equal(got, []byte("blob-1")) || ver != v1 {
		t.Fatalf("get after put: got=%q ver=%q err=%v", got, ver, err)
	}
}

func TestFS_ConflictDetection(t *testing.T) {
	path := filepath.Join(t.TempDir(), "blob")
	b := NewFS(path)
	ctx := context.Background()

	v1, err := b.Put(ctx, []byte("v1"), "")
	if err != nil {
		t.Fatal(err)
	}

	// A second writer overwrites using the current version.
	if _, err := b.Put(ctx, []byte("v2"), v1); err != nil {
		t.Fatal(err)
	}

	// The first writer still holds the stale version v1: its Put must conflict.
	if _, err := b.Put(ctx, []byte("v1-edit"), v1); !errors.Is(err, ErrConflict) {
		t.Fatalf("expected ErrConflict, got %v", err)
	}
}
