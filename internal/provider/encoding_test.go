package provider

import (
	"encoding/binary"
	"testing"
)

func TestNormalizeToUTF8_PlainUTF8(t *testing.T) {
	input := []byte("hello world\nline two")
	got := NormalizeToUTF8(input)
	if string(got) != "hello world\nline two" {
		t.Errorf("Plain UTF-8 should pass through unchanged, got %q", got)
	}
}

func TestNormalizeToUTF8_UTF8BOM(t *testing.T) {
	input := append([]byte{0xEF, 0xBB, 0xBF}, []byte("hello world")...)
	got := NormalizeToUTF8(input)
	if string(got) != "hello world" {
		t.Errorf("UTF-8 BOM should be stripped, got %q", got)
	}
}

func TestNormalizeToUTF8_UTF16LE(t *testing.T) {
	// Encode "gitdb==4.0.9\nsmmap==5.0.0" as UTF-16LE with BOM
	text := "gitdb==4.0.9\nsmmap==5.0.0"
	data := encodeUTF16LE(text)
	got := NormalizeToUTF8(data)
	if string(got) != text {
		t.Errorf("UTF-16LE should be decoded, got %q, want %q", got, text)
	}
}

func TestNormalizeToUTF8_UTF16LEWithCRLF(t *testing.T) {
	// Simulates requirements.txt with CRLF line endings (common on Windows)
	text := "requests==2.31.0\r\nnumpy==1.24.0\r\nlxml==4.8.0"
	data := encodeUTF16LE(text)
	got := NormalizeToUTF8(data)
	if string(got) != text {
		t.Errorf("UTF-16LE with CRLF should be decoded, got %q, want %q", got, text)
	}
}

func TestNormalizeToUTF8_UTF16BE(t *testing.T) {
	text := "hello==1.0.0"
	data := encodeUTF16BE(text)
	got := NormalizeToUTF8(data)
	if string(got) != text {
		t.Errorf("UTF-16BE should be decoded, got %q, want %q", got, text)
	}
}

func TestNormalizeToUTF8_EmptyInput(t *testing.T) {
	got := NormalizeToUTF8([]byte{})
	if len(got) != 0 {
		t.Errorf("Empty input should return empty output, got %q", got)
	}
}

func TestNormalizeToUTF8_OddByteUTF16(t *testing.T) {
	// UTF-16LE BOM + odd number of content bytes (trailing byte dropped)
	data := []byte{0xFF, 0xFE, 0x41, 0x00, 0x42} // 'A' + trailing odd byte
	got := NormalizeToUTF8(data)
	if string(got) != "A" {
		t.Errorf("Odd byte UTF-16 should handle gracefully, got %q", got)
	}
}

// encodeUTF16LE encodes a string as UTF-16LE with BOM.
// Only safe for BMP codepoints (U+0000–U+FFFF); test data must stay within that range.
func encodeUTF16LE(s string) []byte {
	result := []byte{0xFF, 0xFE} // BOM
	for _, r := range s {
		var buf [2]byte
		binary.LittleEndian.PutUint16(buf[:], uint16(r))
		result = append(result, buf[:]...)
	}
	return result
}

// encodeUTF16BE encodes a string as UTF-16BE with BOM.
// Only safe for BMP codepoints (U+0000–U+FFFF); test data must stay within that range.
func encodeUTF16BE(s string) []byte {
	result := []byte{0xFE, 0xFF} // BOM
	for _, r := range s {
		var buf [2]byte
		binary.BigEndian.PutUint16(buf[:], uint16(r))
		result = append(result, buf[:]...)
	}
	return result
}
