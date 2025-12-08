package scanner

import (
	"testing"
	"time"
)

// TestScannerPerformance measures scanner creation and scan performance
func TestScannerPerformance(t *testing.T) {
	// Measure scanner creation time (includes rule loading)
	start := time.Now()
	s, err := NewScanner(".")
	if err != nil {
		t.Fatalf("Failed to create scanner: %v", err)
	}
	creationTime := time.Since(start)

	// Now measure the first scan
	start = time.Now()
	_, err = s.Scan()
	if err != nil {
		t.Fatalf("Failed to scan: %v", err)
	}
	firstScanTime := time.Since(start)

	// Second scan (should be similar to first scan)
	start = time.Now()
	_, err = s.Scan()
	if err != nil {
		t.Fatalf("Failed to scan: %v", err)
	}
	secondScanTime := time.Since(start)

	// Performance summary
	t.Logf("Scanner Creation (includes rule loading): %v", creationTime)
	t.Logf("First Scan: %v", firstScanTime)
	t.Logf("Second Scan: %v", secondScanTime)
}

// BenchmarkScannerCreationSimple measures scanner creation with simple interface
func BenchmarkScannerCreationSimple(b *testing.B) {
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := NewScanner(".")
		if err != nil {
			b.Fatalf("Failed to create scanner: %v", err)
		}
	}
}
