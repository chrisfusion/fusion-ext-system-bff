package auth

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"testing"

	"github.com/fusion-platform/fusion-ext-system-bff/internal/apikey"
)

type stubStore struct {
	id, name string
	err      error
}

func (s *stubStore) Lookup(_ context.Context, _ string) (string, string, error) {
	return s.id, s.name, s.err
}

func TestAPIKeyAuthenticator_NoHeader(t *testing.T) {
	a := NewAPIKeyAuthenticator("X-Api-Key", &stubStore{})
	r, _ := http.NewRequest("GET", "/", nil)
	_, err := a.Authenticate(context.Background(), r)
	if !errors.Is(err, ErrNotApplicable) {
		t.Fatalf("expected ErrNotApplicable, got %v", err)
	}
}

func TestAPIKeyAuthenticator_UnknownKey_IncludesHashPrefix(t *testing.T) {
	a := NewAPIKeyAuthenticator("X-Api-Key", &stubStore{err: apikey.ErrNotFound})
	r, _ := http.NewRequest("GET", "/", nil)
	r.Header.Set("X-Api-Key", "rawsecret")
	_, err := a.Authenticate(context.Background(), r)
	if err == nil {
		t.Fatal("expected error for unknown key")
	}
	if !strings.Contains(err.Error(), "prefix:") {
		t.Fatalf("error should contain hash prefix for audit, got: %v", err)
	}
}

func TestAPIKeyAuthenticator_ValidKey(t *testing.T) {
	a := NewAPIKeyAuthenticator("X-Api-Key", &stubStore{id: "sys-a", name: "System A"})
	r, _ := http.NewRequest("GET", "/", nil)
	r.Header.Set("X-Api-Key", "rawsecret")
	id, err := a.Authenticate(context.Background(), r)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if id.ID != "sys-a" || id.Name != "System A" {
		t.Fatalf("unexpected identity: %+v", id)
	}
}

func TestAPIKeyAuthenticator_LookupError_Wrapped(t *testing.T) {
	infra := errors.New("db connection refused")
	a := NewAPIKeyAuthenticator("X-Api-Key", &stubStore{err: infra})
	r, _ := http.NewRequest("GET", "/", nil)
	r.Header.Set("X-Api-Key", "rawsecret")
	_, err := a.Authenticate(context.Background(), r)
	if err == nil || !strings.Contains(err.Error(), "lookup") {
		t.Fatalf("expected wrapped lookup error, got: %v", err)
	}
}
