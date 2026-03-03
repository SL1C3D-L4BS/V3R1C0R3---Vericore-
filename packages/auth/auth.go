package auth

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"strings"
	"sync"
	"time"
)

// contextKey is a private type for request context keys to avoid collisions.
type contextKey string

const tenantIDContextKey contextKey = "tenant_id"

// TenantIDFromContext returns the TenantID stored in the request context, or "" if not set.
func TenantIDFromContext(ctx context.Context) string {
	if v := ctx.Value(tenantIDContextKey); v != nil {
		if s, ok := v.(string); ok {
			return s
		}
	}
	return ""
}

// KeyValidator looks up a tenant by the SHA-256 hash of an API key.
// Implementations typically query the api_keys table for an unrevoked key_hash.
type KeyValidator interface {
	LookupTenantByKeyHash(ctx context.Context, keyHash string) (tenantID string, ok bool)
}

// HashAPIKey returns the hex-encoded SHA-256 hash of the raw API key.
// Exported so API key provisioning can hash keys before storing.
func HashAPIKey(raw string) string {
	h := sha256.Sum256([]byte(raw))
	return hex.EncodeToString(h[:])
}

type cacheEntry struct {
	tenantID  string
	expiresAt time.Time
}

const defaultCacheTTL = 5 * time.Minute

// TenantAuthMiddleware returns an http.Handler that requires Authorization: Bearer <API_KEY>.
// It uses the KeyValidator to resolve key_hash -> tenant_id and caches lookups in-memory
// with a TTL to avoid hitting the database on every request.
// If the key is valid, the TenantID is injected into the request context.
// If the key is missing or invalid, it responds with 401 Unauthorized.
func TenantAuthMiddleware(validator KeyValidator, next http.Handler) http.Handler {
	var cache sync.Map // keyHash -> *cacheEntry
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		auth := r.Header.Get("Authorization")
		if auth == "" {
			w.WriteHeader(http.StatusUnauthorized)
			_, _ = w.Write([]byte("missing authorization"))
			return
		}
		const prefix = "Bearer "
		if !strings.HasPrefix(auth, prefix) {
			w.WriteHeader(http.StatusUnauthorized)
			_, _ = w.Write([]byte("invalid authorization format"))
			return
		}
		rawKey := strings.TrimSpace(auth[len(prefix):])
		if rawKey == "" {
			w.WriteHeader(http.StatusUnauthorized)
			_, _ = w.Write([]byte("missing api key"))
			return
		}
		keyHash := HashAPIKey(rawKey)

		// Cache lookup with TTL
		if ent, ok := cache.Load(keyHash); ok {
			if e, _ := ent.(*cacheEntry); e != nil && time.Now().Before(e.expiresAt) {
				ctx := context.WithValue(r.Context(), tenantIDContextKey, e.tenantID)
				next.ServeHTTP(w, r.WithContext(ctx))
				return
			}
			cache.Delete(keyHash)
		}

		tenantID, ok := validator.LookupTenantByKeyHash(r.Context(), keyHash)
		if !ok {
			w.WriteHeader(http.StatusUnauthorized)
			_, _ = w.Write([]byte("invalid api key"))
			return
		}
		cache.Store(keyHash, &cacheEntry{tenantID: tenantID, expiresAt: time.Now().Add(defaultCacheTTL)})
		ctx := context.WithValue(r.Context(), tenantIDContextKey, tenantID)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}
