package server

import (
	"context"
	"testing"

	"github.com/snipspin/jig-mcp/internal/auth"
)

func TestCallerIdentityRoundTrip(t *testing.T) {
	ctx := context.Background()
	id := auth.CallerIdentity{Name: "test-caller", Transport: "stdio"}
	newCtx := auth.WithCaller(ctx, id)

	retrieved := auth.CallerFrom(newCtx)
	if retrieved.Name != "test-caller" {
		t.Errorf("expected 'test-caller', got %q", retrieved.Name)
	}
}

func TestCallerFrom_EmptyContext(t *testing.T) {
	ctx := context.Background()
	caller := auth.CallerFrom(ctx)
	if caller.Name != "unknown" {
		t.Errorf("expected 'unknown', got %q", caller.Name)
	}
}
