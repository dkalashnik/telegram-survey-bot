package config

import (
	"fmt"
	"os"
	"strconv"
	"sync"
)

var (
	targetUserID int64
	targetMu     sync.RWMutex
)

// LoadTargetUserIDFromEnv reads TARGET_USER_ID env var and stores it for later retrieval.
func LoadTargetUserIDFromEnv() error {
	raw := os.Getenv("TARGET_USER_ID")
	if raw == "" {
		return fmt.Errorf("TARGET_USER_ID environment variable not set")
	}
	parsed, err := strconv.ParseInt(raw, 10, 64)
	if err != nil || parsed == 0 {
		return fmt.Errorf("invalid TARGET_USER_ID: %q", raw)
	}
	targetMu.Lock()
	targetUserID = parsed
	targetMu.Unlock()
	return nil
}

// GetTargetUserID returns the configured target user id (0 if unset).
func GetTargetUserID() int64 {
	targetMu.RLock()
	defer targetMu.RUnlock()
	return targetUserID
}

// SetTargetUserID is intended for tests.
func SetTargetUserID(id int64) {
	targetMu.Lock()
	targetUserID = id
	targetMu.Unlock()
}
