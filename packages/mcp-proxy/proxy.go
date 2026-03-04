package mcpproxy

import (
	"crypto/sha256"
	"encoding/hex"
	"sync"
	"time"
)

const defaultTTL = 15 * time.Minute

type cacheEntry struct {
	payload   []byte
	expiresAt time.Time
}

var (
	cache   sync.Map
	stopTTL chan struct{}
)

func init() {
	stopTTL = make(chan struct{})
	go evictExpired()
}

// evictExpired runs in the background and removes entries older than defaultTTL.
func evictExpired() {
	ticker := time.NewTicker(time.Minute)
	defer ticker.Stop()
	for {
		select {
		case <-stopTTL:
			return
		case now := <-ticker.C:
			cache.Range(func(key, value interface{}) bool {
				if e, ok := value.(*cacheEntry); ok && e.expiresAt.Before(now) {
					cache.Delete(key)
				}
				return true
			})
		}
	}
}

// RegisterContext computes SHA-256 of payload, stores payload in the TTL cache under the hex-encoded hash, and returns the hash.
func RegisterContext(payload []byte) (contextHash string) {
	h := sha256.Sum256(payload)
	contextHash = hex.EncodeToString(h[:])
	payloadCopy := make([]byte, len(payload))
	copy(payloadCopy, payload)
	cache.Store(contextHash, &cacheEntry{payload: payloadCopy, expiresAt: time.Now().Add(defaultTTL)})
	return contextHash
}

// ValidateContext returns true if the contextHash exists in the cache (and has not expired).
func ValidateContext(contextHash string) bool {
	v, ok := cache.Load(contextHash)
	if !ok {
		return false
	}
	e, ok := v.(*cacheEntry)
	if !ok || e.expiresAt.Before(time.Now()) {
		cache.Delete(contextHash)
		return false
	}
	return true
}
