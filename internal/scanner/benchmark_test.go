package scanner

import (
	"testing"

	"github.com/petrarca/tech-stack-analyzer/internal/types"
)

// BenchmarkPayloadCreation benchmarks payload creation and basic operations
func BenchmarkPayloadCreation(b *testing.B) {
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		payload := types.NewPayload("test", []string{"/test"})
		payload.AddTech("golang", "found go.mod")
		// Note: DetectLanguage is now handled by go-enry, not by Payload
	}
}

// BenchmarkTechAddition benchmarks adding multiple techs to a payload
func BenchmarkTechAddition(b *testing.B) {
	payload := types.NewPayload("test", []string{"/test"})
	techs := []string{"golang", "nodejs", "docker", "aws", "kubernetes"}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		for _, tech := range techs {
			payload.AddTech(tech, "detected")
		}
	}
}
