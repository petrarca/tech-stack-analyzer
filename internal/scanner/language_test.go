package scanner

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/petrarca/tech-stack-analyzer/internal/config"
)

func TestDetectLanguage_NoReclassify(t *testing.T) {
	d := NewLanguageDetector(nil, "")

	lang := d.DetectLanguage("main.cpp", []byte("#include <iostream>"))
	if lang != "C++" {
		t.Errorf("expected C++, got %q", lang)
	}

	lang = d.DetectLanguage("billing.e", []byte("EBM;01781;1;1-852"))
	if lang != "Eiffel" {
		t.Errorf("expected Eiffel (go-enry default), got %q", lang)
	}
}

func TestDetectLanguageWithType_LanguageAndType(t *testing.T) {
	dir := t.TempDir()
	rules := []config.ReclassifyRule{
		{Match: "**/*.e", Language: "CSV", Type: "data"},
	}
	d := NewLanguageDetector(rules, dir)

	result := d.DetectLanguageWithType(filepath.Join(dir, "data", "billing.e"), []byte("EBM;01781"))
	if result.Language != "CSV" {
		t.Errorf("language: expected CSV, got %q", result.Language)
	}
	if result.TypeOverride != "data" {
		t.Errorf("type: expected data, got %q", result.TypeOverride)
	}
}

func TestDetectLanguageWithType_TypeOnly(t *testing.T) {
	dir := t.TempDir()
	rules := []config.ReclassifyRule{
		{Match: "**/*.e", Type: "data"},
	}
	d := NewLanguageDetector(rules, dir)

	result := d.DetectLanguageWithType(filepath.Join(dir, "billing.e"), []byte("EBM;01781"))
	if result.Language != "" {
		t.Errorf("language: expected empty (type-only rule leaves no language label), got %q", result.Language)
	}
	if result.TypeOverride != "data" {
		t.Errorf("type: expected data, got %q", result.TypeOverride)
	}
}

func TestDetectLanguageWithType_LanguageOnly(t *testing.T) {
	dir := t.TempDir()
	rules := []config.ReclassifyRule{
		{Match: "**/*.e", Language: "CSV"},
	}
	d := NewLanguageDetector(rules, dir)

	result := d.DetectLanguageWithType(filepath.Join(dir, "billing.e"), []byte("EBM;01781"))
	if result.Language != "CSV" {
		t.Errorf("language: expected CSV, got %q", result.Language)
	}
	if result.TypeOverride != "" {
		t.Errorf("type: expected empty (no override), got %q", result.TypeOverride)
	}
}

func TestDetectLanguageWithType_NoMatch(t *testing.T) {
	dir := t.TempDir()
	rules := []config.ReclassifyRule{
		{Match: "**/*.e", Language: "CSV", Type: "data"},
	}
	d := NewLanguageDetector(rules, dir)

	result := d.DetectLanguageWithType(filepath.Join(dir, "main.cpp"), []byte("#include <iostream>"))
	if result.Language != "C++" {
		t.Errorf("language: expected C++, got %q", result.Language)
	}
	if result.TypeOverride != "" {
		t.Errorf("type: expected empty (no override), got %q", result.TypeOverride)
	}
}

func TestDetectLanguageWithType_FolderScoped(t *testing.T) {
	dir := t.TempDir()
	// Create subdirs so paths exist
	os.MkdirAll(filepath.Join(dir, "data", "EBM"), 0o755)
	os.MkdirAll(filepath.Join(dir, "src"), 0o755)

	rules := []config.ReclassifyRule{
		{Match: "data/**/*.e", Language: "CSV", Type: "data"},
	}
	d := NewLanguageDetector(rules, dir)

	// File in data/ — should match
	result := d.DetectLanguageWithType(filepath.Join(dir, "data", "EBM", "billing.e"), []byte("EBM;01781"))
	if result.Language != "CSV" {
		t.Errorf("data/EBM/billing.e: expected CSV, got %q", result.Language)
	}
	if result.TypeOverride != "data" {
		t.Errorf("data/EBM/billing.e: expected type data, got %q", result.TypeOverride)
	}

	// File in src/ — should NOT match, fall through to go-enry
	result = d.DetectLanguageWithType(filepath.Join(dir, "src", "code.e"), []byte("class MAIN"))
	if result.Language == "CSV" {
		t.Errorf("src/code.e: should not match folder-scoped rule, got CSV")
	}
	if result.TypeOverride != "" {
		t.Errorf("src/code.e: expected no type override, got %q", result.TypeOverride)
	}
}

func TestDetectLanguageWithType_InvalidGlobSilentlyIgnored(t *testing.T) {
	dir := t.TempDir()
	rules := []config.ReclassifyRule{
		{Match: "[invalid-glob", Language: "CSV", Type: "data"}, // malformed glob
	}
	d := NewLanguageDetector(rules, dir)

	// Invalid glob should not match anything — falls through to go-enry
	result := d.DetectLanguageWithType(filepath.Join(dir, "billing.e"), []byte("EBM;01781"))
	if result.Language == "CSV" {
		t.Errorf("invalid glob should not match, but got CSV")
	}
	if result.TypeOverride != "" {
		t.Errorf("invalid glob should not set type override, got %q", result.TypeOverride)
	}
	// go-enry default for .e is Eiffel
	if result.Language != "Eiffel" {
		t.Errorf("expected go-enry fallback (Eiffel), got %q", result.Language)
	}
}

func TestDetectLanguageWithType_FirstMatchWins(t *testing.T) {
	dir := t.TempDir()
	rules := []config.ReclassifyRule{
		{Match: "**/special/*.e", Language: "Text", Type: "prose"},
		{Match: "**/*.e", Language: "CSV", Type: "data"},
	}
	d := NewLanguageDetector(rules, dir)

	os.MkdirAll(filepath.Join(dir, "special"), 0o755)

	// Matches first rule
	result := d.DetectLanguageWithType(filepath.Join(dir, "special", "notes.e"), nil)
	if result.Language != "Text" {
		t.Errorf("expected Text (first rule), got %q", result.Language)
	}
	if result.TypeOverride != "prose" {
		t.Errorf("expected prose, got %q", result.TypeOverride)
	}

	// Matches second rule
	result = d.DetectLanguageWithType(filepath.Join(dir, "other", "billing.e"), nil)
	if result.Language != "CSV" {
		t.Errorf("expected CSV (second rule), got %q", result.Language)
	}
}
