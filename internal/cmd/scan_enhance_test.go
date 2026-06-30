package cmd

import (
	"testing"

	"github.com/petrarca/tech-stack-analyzer/internal/config"
	"github.com/petrarca/tech-stack-analyzer/internal/types"
)

func hasTech(p *types.Payload, tech string) bool {
	for _, t := range p.Techs {
		if t == tech {
			return true
		}
	}
	return false
}

// TestEnhanceSinglePayload characterizes config-based payload enhancement
// (custom properties + configured techs) before refactoring.
func TestEnhanceSinglePayload(t *testing.T) {
	t.Run("nil config is a no-op", func(t *testing.T) {
		p := types.NewPayload("root", nil)
		enhanceSinglePayload(p, nil)
		if len(p.Techs) != 0 || len(p.Properties) != 0 {
			t.Fatalf("nil config should not modify payload, got techs=%v props=%v", p.Techs, p.Properties)
		}
	})

	t.Run("non-payload input is ignored", func(t *testing.T) {
		cfg := &config.ScanConfig{Properties: map[string]interface{}{"team": "x"}}
		// Must not panic on a non-*types.Payload value.
		enhanceSinglePayload("not a payload", cfg)
	})

	t.Run("merges custom properties", func(t *testing.T) {
		p := types.NewPayload("root", nil)
		cfg := &config.ScanConfig{Properties: map[string]interface{}{"team": "platform", "owner": "eng"}}
		enhanceSinglePayload(p, cfg)
		if p.Properties["team"] != "platform" || p.Properties["owner"] != "eng" {
			t.Fatalf("expected custom properties merged, got %v", p.Properties)
		}
	})

	t.Run("adds known configured tech", func(t *testing.T) {
		p := types.NewPayload("root", nil)
		cfg := &config.ScanConfig{Techs: []config.ConfigTech{{Tech: "nodejs", Reason: "declared"}}}
		enhanceSinglePayload(p, cfg)
		if !hasTech(p, "nodejs") {
			t.Fatalf("expected known tech nodejs added, got %v", p.Techs)
		}
	})

	t.Run("unknown configured tech maps to unmapped_unknown", func(t *testing.T) {
		p := types.NewPayload("root", nil)
		cfg := &config.ScanConfig{Techs: []config.ConfigTech{{Tech: "totally-made-up-tech"}}}
		enhanceSinglePayload(p, cfg)
		if !hasTech(p, "unmapped_unknown") {
			t.Fatalf("expected unknown tech mapped to unmapped_unknown, got %v", p.Techs)
		}
	})
}
