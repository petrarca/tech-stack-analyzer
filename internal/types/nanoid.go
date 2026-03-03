package types

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"math/big"
)

// Alphabet used for nanoid generation (alphanumeric)
const alphabet = "0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz"

// GenerateRootID generates a unique 12-character random ID for root/main components.
// Each scan gets a unique root ID, ensuring all component IDs within that scan are unique.
func GenerateRootID() string {
	const length = 12
	const alphabetLen = 62 // len(alphabet)

	result := make([]byte, length)
	for i := range result {
		n, _ := rand.Int(rand.Reader, big.NewInt(int64(alphabetLen)))
		result[i] = alphabet[n.Int64()]
	}

	return string(result)
}

// GenerateComponentID generates a deterministic 20-character ID for child components.
// It combines the root ID, component name, and relative path to create unique, reproducible IDs
// within a scan context. Components with different names at the same path will have different IDs.
func GenerateComponentID(rootID, name, relativePath string) string {
	content := fmt.Sprintf("%s:%s:%s", rootID, name, relativePath)
	hash := sha256.Sum256([]byte(content))
	return hex.EncodeToString(hash[:])[:20]
}
