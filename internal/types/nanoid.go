package types

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"math/big"
)

// Alphabet matches the TypeScript nanoid alphabet (kept for backward compatibility)
const alphabet = "0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz"

// GenerateID generates a stable 20-character hash-based ID using name and relative path
// This provides stable, reproducible IDs across scans while maintaining uniqueness
func GenerateID(name, relativePath string) string {
	content := fmt.Sprintf("%s:%s", name, relativePath)
	hash := sha256.Sum256([]byte(content))
	return hex.EncodeToString(hash[:])[:20]
}

// GenerateRandomID generates a 12-character nanoid-like string (legacy)
// Matches TypeScript: nid = customAlphabet(alphabet, maxSize)
// DEPRECATED: Use GenerateID for stable IDs
func GenerateRandomID() string {
	const length = 12
	const alphabetLen = 62 // len(alphabet)

	result := make([]byte, length)
	for i := range result {
		n, _ := rand.Int(rand.Reader, big.NewInt(int64(alphabetLen)))
		result[i] = alphabet[n.Int64()]
	}

	return string(result)
}
