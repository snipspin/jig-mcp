package auth

import (
	"context"
	"os"
	"testing"
)

func TestTokenRegistrySingleToken(t *testing.T) {
	os.Setenv("JIG_AUTH_TOKEN", "tok_single")
	defer os.Unsetenv("JIG_AUTH_TOKEN")
	os.Unsetenv("JIG_AUTH_TOKENS")
	InitTokenRegistry()
	defer func() {
		GlobalTokens().mu.Lock()
		GlobalTokens().entries = nil
		GlobalTokens().mu.Unlock()
	}()

	if !GlobalTokens().AuthRequired() {
		t.Fatal("expected auth required")
	}

	name, ok := GlobalTokens().Lookup("tok_single")
	if !ok {
		t.Fatal("expected token to match")
	}
	if name != "default" {
		t.Errorf("expected name 'default', got %q", name)
	}

	_, ok = GlobalTokens().Lookup("wrong")
	if ok {
		t.Error("expected wrong token to fail")
	}
}

func TestTokenRegistryMultipleTokens(t *testing.T) {
	os.Setenv("JIG_AUTH_TOKENS", "alpha:tok_a,beta:tok_b")
	defer os.Unsetenv("JIG_AUTH_TOKENS")
	InitTokenRegistry()
	defer func() {
		GlobalTokens().mu.Lock()
		GlobalTokens().entries = nil
		GlobalTokens().mu.Unlock()
	}()

	name, ok := GlobalTokens().Lookup("tok_a")
	if !ok || name != "alpha" {
		t.Errorf("expected alpha, got %q ok=%v", name, ok)
	}

	name, ok = GlobalTokens().Lookup("tok_b")
	if !ok || name != "beta" {
		t.Errorf("expected beta, got %q ok=%v", name, ok)
	}

	_, ok = GlobalTokens().Lookup("tok_c")
	if ok {
		t.Error("expected unknown token to fail")
	}
}

func TestTokenRegistryPrecedence(t *testing.T) {
	// JIG_AUTH_TOKENS should take precedence over JIG_AUTH_TOKEN.
	os.Setenv("JIG_AUTH_TOKEN", "tok_single")
	os.Setenv("JIG_AUTH_TOKENS", "multi:tok_multi")
	defer os.Unsetenv("JIG_AUTH_TOKEN")
	defer os.Unsetenv("JIG_AUTH_TOKENS")
	InitTokenRegistry()
	defer func() {
		GlobalTokens().mu.Lock()
		GlobalTokens().entries = nil
		GlobalTokens().mu.Unlock()
	}()

	// The multi-token should work.
	name, ok := GlobalTokens().Lookup("tok_multi")
	if !ok || name != "multi" {
		t.Errorf("expected multi, got %q ok=%v", name, ok)
	}

	// The single token should NOT work (it was overridden).
	_, ok = GlobalTokens().Lookup("tok_single")
	if ok {
		t.Error("JIG_AUTH_TOKEN should not work when JIG_AUTH_TOKENS is set")
	}
}

func TestTokenRegistryMalformedEntries(t *testing.T) {
	os.Setenv("JIG_AUTH_TOKENS", "good:tok_ok,,bad_no_colon,:empty_name,empty_tok:")
	defer os.Unsetenv("JIG_AUTH_TOKENS")
	InitTokenRegistry()
	defer func() {
		GlobalTokens().mu.Lock()
		GlobalTokens().entries = nil
		GlobalTokens().mu.Unlock()
	}()

	// Only the well-formed entry should survive.
	name, ok := GlobalTokens().Lookup("tok_ok")
	if !ok || name != "good" {
		t.Errorf("expected good entry to survive, got %q ok=%v", name, ok)
	}

	GlobalTokens().mu.RLock()
	count := len(GlobalTokens().entries)
	GlobalTokens().mu.RUnlock()
	if count != 1 {
		t.Errorf("expected 1 valid entry, got %d", count)
	}
}

func TestCallerFromContext(t *testing.T) {
	// Empty context returns "unknown".
	id := CallerFrom(context.Background())
	if id.Name != "unknown" {
		t.Errorf("expected 'unknown', got %q", id.Name)
	}

	// Context with caller.
	ctx := WithCaller(context.Background(), CallerIdentity{Name: "test-agent", Transport: "sse"})
	id = CallerFrom(ctx)
	if id.Name != "test-agent" {
		t.Errorf("expected 'test-agent', got %q", id.Name)
	}
	if id.Transport != "sse" {
		t.Errorf("expected 'sse', got %q", id.Transport)
	}
}

func TestTokenRegistryClear(t *testing.T) {
	os.Setenv("JIG_AUTH_TOKEN", "tok_clear")
	defer os.Unsetenv("JIG_AUTH_TOKEN")
	InitTokenRegistry()

	if !GlobalTokens().AuthRequired() {
		t.Fatal("expected auth required before clear")
	}

	GlobalTokens().Clear()

	if GlobalTokens().AuthRequired() {
		t.Error("expected auth not required after clear")
	}
}
