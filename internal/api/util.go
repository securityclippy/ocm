package api

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"time"
)

// generateID creates a unique ID with the given prefix.
func generateID(prefix string) string {
	b := make([]byte, 8)
	rand.Read(b)
	return fmt.Sprintf("%s_%s_%d", prefix, hex.EncodeToString(b), time.Now().UnixMilli()%10000)
}
