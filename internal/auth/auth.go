// Package auth provides caller identity propagation and token-based authentication
// for jig-mcp transports.
package auth

import (
	"context"
	"crypto/subtle"
	"log/slog"
	"os"
	"strings"
	"sync"

	"github.com/snipspin/jig-mcp/internal/logging"
)

// CallerIdentity represents the authenticated caller making a request.
// For stdio transport, identity is always "local" (inherited OS permissions).
// For SSE transport, identity is resolved from the token registry.
type CallerIdentity struct {
	Name      string // Human-readable name (e.g. "homelab-agent", "anonymous")
	Transport string // "stdio", "sse"
}

// contextKey is an unexported type for context keys to avoid collisions.
type contextKey int

const callerKey contextKey = iota

// WithCaller attaches a CallerIdentity to a context.
func WithCaller(ctx context.Context, id CallerIdentity) context.Context {
	return context.WithValue(ctx, callerKey, id)
}

// CallerFrom extracts the callerIdentity from a context.
// Returns a zero-value identity with Name "unknown" if not present.
func CallerFrom(ctx context.Context) CallerIdentity {
	if id, ok := ctx.Value(callerKey).(CallerIdentity); ok {
		return id
	}
	return CallerIdentity{Name: "unknown"}
}

// tokenEntry maps a token to its owner name.
type tokenEntry struct {
	name  string
	token string
}

// TokenRegistry holds the set of valid tokens, loaded once at startup.
type TokenRegistry struct {
	mu      sync.RWMutex
	entries []tokenEntry // empty means auth disabled
}

// globalTokens is the singleton token registry, initialized by InitTokenRegistry.
var globalTokens TokenRegistry

// GlobalTokens returns the global token registry for use by other packages.
func GlobalTokens() *TokenRegistry {
	return &globalTokens
}

// InitTokenRegistry reads JIG_AUTH_TOKENS (plural) and JIG_AUTH_TOKEN (singular)
// and populates the global token registry. JIG_AUTH_TOKENS takes precedence.
//
// JIG_AUTH_TOKENS format: "name1:token1,name2:token2"
// JIG_AUTH_TOKEN format:  "token" (mapped to name "default")
func InitTokenRegistry() {
	if v := os.Getenv("JIG_AUTH_TOKENS"); v != "" {
		var entries []tokenEntry
		for _, pair := range strings.Split(v, ",") {
			pair = strings.TrimSpace(pair)
			if pair == "" {
				continue
			}
			parts := strings.SplitN(pair, ":", 2)
			if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
				slog.Warn("ignoring malformed JIG_AUTH_TOKENS entry (expected name:token)", logging.Sanitize("entry", pair))
				continue
			}
			entries = append(entries, tokenEntry{name: parts[0], token: parts[1]})
		}
		if len(entries) == 0 {
			slog.Warn("JIG_AUTH_TOKENS set but no valid entries parsed, auth disabled")
		} else {
			globalTokens.mu.Lock()
			globalTokens.entries = entries
			globalTokens.mu.Unlock()
			slog.Info("token registry initialized from JIG_AUTH_TOKENS", "count", len(entries))
		}
		return
	}

	if v := os.Getenv("JIG_AUTH_TOKEN"); v != "" {
		globalTokens.mu.Lock()
		globalTokens.entries = []tokenEntry{{name: "default", token: v}}
		globalTokens.mu.Unlock()
		slog.Info("token registry initialized from JIG_AUTH_TOKEN", "count", 1)
	}
}

// Lookup checks a candidate token against the registry using constant-time
// comparison. Returns the owner name and true if matched, or ("", false).
func (tr *TokenRegistry) Lookup(candidate string) (string, bool) {
	tr.mu.RLock()
	defer tr.mu.RUnlock()

	for _, e := range tr.entries {
		if subtle.ConstantTimeCompare([]byte(candidate), []byte(e.token)) == 1 {
			return e.name, true
		}
	}
	return "", false
}

// AuthRequired returns true if at least one token is configured.
func (tr *TokenRegistry) AuthRequired() bool {
	tr.mu.RLock()
	defer tr.mu.RUnlock()
	return len(tr.entries) > 0
}

// Clear resets the token registry. For testing purposes only.
func (tr *TokenRegistry) Clear() {
	tr.mu.Lock()
	defer tr.mu.Unlock()
	tr.entries = nil
}
