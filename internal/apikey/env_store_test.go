package apikey

import (
	"context"
	"strings"
	"testing"
)

func validHash() string { return strings.Repeat("ab", 32) } // 64 valid hex chars

func TestNewEnvStore_ValidEntry(t *testing.T) {
	raw := validHash() + ":sys-a:System A"
	s, err := NewEnvStore(raw)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	id, name, err := s.Lookup(context.Background(), validHash())
	if err != nil {
		t.Fatalf("lookup failed: %v", err)
	}
	if id != "sys-a" || name != "System A" {
		t.Fatalf("got id=%q name=%q", id, name)
	}
}

func TestNewEnvStore_HashTooShort(t *testing.T) {
	_, err := NewEnvStore("deadbeef:sys-a:System A")
	if err == nil || !strings.Contains(err.Error(), "64 hex chars") {
		t.Fatalf("expected length error, got: %v", err)
	}
}

func TestNewEnvStore_HashTooLong(t *testing.T) {
	_, err := NewEnvStore(strings.Repeat("a", 65) + ":sys-a:System A")
	if err == nil || !strings.Contains(err.Error(), "64 hex chars") {
		t.Fatalf("expected length error, got: %v", err)
	}
}

func TestNewEnvStore_HashNotHex(t *testing.T) {
	nonHex := strings.Repeat("z", 64)
	_, err := NewEnvStore(nonHex + ":sys-a:System A")
	if err == nil || !strings.Contains(err.Error(), "not valid hex") {
		t.Fatalf("expected hex error, got: %v", err)
	}
}

func TestNewEnvStore_MissingFields(t *testing.T) {
	_, err := NewEnvStore(validHash() + ":sys-a")
	if err == nil || !strings.Contains(err.Error(), "expected sha256hex:id:name") {
		t.Fatalf("expected format error, got: %v", err)
	}
}

func TestNewEnvStore_Empty(t *testing.T) {
	s, err := NewEnvStore("")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	_, _, err = s.Lookup(context.Background(), validHash())
	if err != ErrNotFound {
		t.Fatalf("expected ErrNotFound, got: %v", err)
	}
}

func TestNewEnvStore_MultipleEntries(t *testing.T) {
	h1, h2 := strings.Repeat("11", 32), strings.Repeat("22", 32)
	raw := h1 + ":a:Alpha," + h2 + ":b:Beta"
	s, err := NewEnvStore(raw)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	id, _, _ := s.Lookup(context.Background(), h1)
	if id != "a" {
		t.Fatalf("expected 'a', got %q", id)
	}
	id, _, _ = s.Lookup(context.Background(), h2)
	if id != "b" {
		t.Fatalf("expected 'b', got %q", id)
	}
}
