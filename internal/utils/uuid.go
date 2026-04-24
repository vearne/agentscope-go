package utils

import (
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"time"
)

// ShortUUID generates a short unique identifier (22 chars) using crypto/rand.
func ShortUUID() string {
	b := make([]byte, 16)
	_, _ = rand.Read(b)
	return base64.RawURLEncoding.EncodeToString(b)
}

// TimestampID generates an ID from current timestamp + 6 random hex chars.
func TimestampID() string {
	prefix := time.Now().Format("20060102_150405")
	b := make([]byte, 3)
	_, _ = rand.Read(b)
	return fmt.Sprintf("%s_%x", prefix, b)
}
