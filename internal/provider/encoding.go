package provider

import (
	"bytes"
	"encoding/binary"
	"unicode/utf16"
)

// Byte Order Marks for various encodings
var (
	bomUTF8    = []byte{0xEF, 0xBB, 0xBF}
	bomUTF16LE = []byte{0xFF, 0xFE}
	bomUTF16BE = []byte{0xFE, 0xFF}
)

// NormalizeToUTF8 detects BOM-marked encodings (UTF-16LE, UTF-16BE, UTF-8 with BOM)
// and converts the content to plain UTF-8. Returns the input unchanged if no BOM is found.
func NormalizeToUTF8(data []byte) []byte {
	if len(data) >= 3 && bytes.HasPrefix(data, bomUTF8) {
		// Strip UTF-8 BOM
		return data[3:]
	}

	if len(data) >= 2 && bytes.HasPrefix(data, bomUTF16LE) {
		return decodeUTF16(data[2:], binary.LittleEndian)
	}

	if len(data) >= 2 && bytes.HasPrefix(data, bomUTF16BE) {
		return decodeUTF16(data[2:], binary.BigEndian)
	}

	return data
}

// decodeUTF16 converts UTF-16 encoded bytes to UTF-8
func decodeUTF16(data []byte, order binary.ByteOrder) []byte {
	// Need even number of bytes for UTF-16
	if len(data)%2 != 0 {
		data = data[:len(data)-1]
	}

	u16s := make([]uint16, len(data)/2)
	for i := range u16s {
		u16s[i] = order.Uint16(data[i*2 : i*2+2])
	}

	runes := utf16.Decode(u16s)

	var buf bytes.Buffer
	buf.Grow(len(runes) * 2) // Reasonable initial capacity
	for _, r := range runes {
		buf.WriteRune(r)
	}

	return buf.Bytes()
}
