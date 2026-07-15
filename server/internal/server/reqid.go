package server

import (
	"crypto/rand"
	"encoding/hex"
)

func randomID() string {
	b := make([]byte, 8)
	if _, err := rand.Read(b); err != nil {
		return "anon"
	}
	return hex.EncodeToString(b)
}
