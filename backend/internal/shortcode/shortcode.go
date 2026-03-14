package shortcode

import (
	"crypto/rand"
)

const alphabet = "abcdefghijkmnopqrstuvwxyzABCDEFGHJKLMNPQRSTUVWXYZ23456789"

// Generate returns a cryptographically random URL-safe string of given length.
func Generate(length int) string {
	b := make([]byte, length)
	_, _ = rand.Read(b)
	for i, v := range b {
		b[i] = alphabet[int(v)%len(alphabet)]
	}
	return string(b)
}
