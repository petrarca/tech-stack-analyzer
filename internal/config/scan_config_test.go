package config

import (
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
)

// ---- MergeWithSettings -----------------------------------------------------

func TestMergeWithSettings_NilGuards(t *testing.T) {
	// Neither nil receiver nor nil settings should panic.
	var nilCfg *ScanConfigFile
	nilCfg.MergeWithSettings(nil)
	nilCfg.MergeWithSettings(DefaultSettings())

	valid := &ScanConfigFile{}
	valid.MergeWithSettings(nil)
}

func TestMergeWithSettings_ScanOptions_OnlyWritesWhenTargetIsDefault(t *testing.T) {
	// Config sets SubsystemDepth=2; settings already has SubsystemDepth=1 (non-default).
	// Config value must NOT overwrite the explicit CLI value.
	cfg := &ScanConfigFile{Scan: ScanOptions{SubsystemDepth: 2}}
	s := DefaultSettings()
	s.SubsystemDepth = 1

	cfg.MergeWithSettings(s)

	if s.SubsystemDepth != 1 {
		t.Errorf("SubsystemDepth: got %d, want 1 (CLI value must not be overwritten)", s.SubsystemDepth)
	}
}

func TestMergeWithSettings_ScanOptions_WritesWhenTargetIsDefault(t *testing.T) {
	// Config sets SubsystemDepth=2; settings is at default (0). Config value must win.
	cfg := &ScanConfigFile{Scan: ScanOptions{SubsystemDepth: 2}}
	s := DefaultSettings()

	cfg.MergeWithSettings(s)

	if s.SubsystemDepth != 2 {
		t.Errorf("SubsystemDepth: got %d, want 2 (config value must fill default)", s.SubsystemDepth)
	}
}

func TestMergeWithSettings_ExcludePatterns(t *testing.T) {
	cfg := &ScanConfigFile{Exclude: []string{"vendor", "*.tmp"}}
	s := DefaultSettings()

	cfg.MergeWithSettings(s)

	want := []string{"vendor", "*.tmp"}
	if diff := cmp.Diff(want, s.ExcludePatterns); diff != "" {
		t.Errorf("ExcludePatterns mismatch (-want +got):\n%s", diff)
	}
}

func TestMergeWithSettings_ExcludePatterns_NotOverwrittenWhenSet(t *testing.T) {
	// Settings already has patterns (e.g. from CLI --exclude flags).
	// Config must not overwrite them — config only fills when settings is empty.
	cfg := &ScanConfigFile{Exclude: []string{"vendor"}}
	s := DefaultSettings()
	s.ExcludePatterns = []string{"build"}

	cfg.MergeWithSettings(s)

	want := []string{"build"}
	if diff := cmp.Diff(want, s.ExcludePatterns); diff != "" {
		t.Errorf("ExcludePatterns mismatch (-want +got):\n%s", diff)
	}
}

func TestMergeWithSettings_SubsystemGroups(t *testing.T) {
	cfg := &ScanConfigFile{
		SubsystemGroups: map[string]SubsystemGroup{
			"core": {Paths: []string{"/core"}, Description: "Core"},
		},
	}
	s := DefaultSettings()

	cfg.MergeWithSettings(s)

	if len(s.SubsystemGroups) != 1 {
		t.Fatalf("SubsystemGroups: got %d entries, want 1", len(s.SubsystemGroups))
	}
	if s.SubsystemGroups["core"].Description != "Core" {
		t.Errorf("SubsystemGroups[core].Description: got %q, want %q",
			s.SubsystemGroups["core"].Description, "Core")
	}
}

func TestMergeWithSettings_SubsystemGroups_NotOverwrittenWhenSet(t *testing.T) {
	// Settings already has groups (from a previous merge). Config must not replace them.
	cfg := &ScanConfigFile{
		SubsystemGroups: map[string]SubsystemGroup{
			"new": {Paths: []string{"/new"}},
		},
	}
	s := DefaultSettings()
	s.SubsystemGroups = map[string]SubsystemGroup{
		"existing": {Paths: []string{"/existing"}},
	}

	cfg.MergeWithSettings(s)

	if _, ok := s.SubsystemGroups["existing"]; !ok {
		t.Error("SubsystemGroups: existing entry was overwritten")
	}
	if _, ok := s.SubsystemGroups["new"]; ok {
		t.Error("SubsystemGroups: new entry was added despite settings already having groups")
	}
}

// ---- GetMergedConfig -------------------------------------------------------

func TestGetMergedConfig_NilScanConfig(t *testing.T) {
	// nil receiver returns the project config unchanged.
	var nilCfg *ScanConfigFile
	proj := &ScanConfig{Properties: map[string]interface{}{"env": "prod"}}

	got := nilCfg.GetMergedConfig(proj)

	if got != proj {
		t.Error("nil ScanConfigFile.GetMergedConfig must return projectConfig unchanged")
	}
}

func TestGetMergedConfig_NilProjectConfig(t *testing.T) {
	// nil projectConfig — scan config values are used.
	cfg := &ScanConfigFile{
		Properties: map[string]interface{}{"version": "1.0"},
		Exclude:    []string{"vendor"},
	}

	got := cfg.GetMergedConfig(nil)

	if got.Properties["version"] != "1.0" {
		t.Errorf("properties.version: got %v, want %q", got.Properties["version"], "1.0")
	}
	if diff := cmp.Diff([]string{"vendor"}, got.Exclude); diff != "" {
		t.Errorf("Exclude mismatch (-want +got):\n%s", diff)
	}
}

func TestGetMergedConfig_ProjectConfigOverridesProperties(t *testing.T) {
	// Project config properties override scan config for the same key.
	cfg := &ScanConfigFile{
		Properties: map[string]interface{}{"env": "staging", "version": "1.0"},
	}
	proj := &ScanConfig{
		Properties: map[string]interface{}{"env": "prod"},
	}

	got := cfg.GetMergedConfig(proj)

	if got.Properties["env"] != "prod" {
		t.Errorf("env: got %v, want %q (project config must override)", got.Properties["env"], "prod")
	}
	if got.Properties["version"] != "1.0" {
		t.Errorf("version: got %v, want %q (scan config value must survive)", got.Properties["version"], "1.0")
	}
}

func TestGetMergedConfig_ExcludesAreMerged(t *testing.T) {
	// Excludes from both configs are combined.
	cfg := &ScanConfigFile{Exclude: []string{"vendor", "dist"}}
	proj := &ScanConfig{Exclude: []string{"*.log"}}

	got := cfg.GetMergedConfig(proj)

	want := []string{"vendor", "dist", "*.log"}
	if diff := cmp.Diff(want, got.Exclude, cmpopts.SortSlices(func(a, b string) bool { return a < b })); diff != "" {
		t.Errorf("Exclude mismatch (-want +got):\n%s", diff)
	}
}

func TestGetMergedConfig_ReclassifyProjectRulesPrepended(t *testing.T) {
	// Project reclassify rules are prepended (first-match-wins = project takes priority).
	cfg := &ScanConfigFile{
		Reclassify: []ReclassifyRule{{Match: "scan-rule"}},
	}
	proj := &ScanConfig{
		Reclassify: []ReclassifyRule{{Match: "project-rule"}},
	}

	got := cfg.GetMergedConfig(proj)

	if len(got.Reclassify) != 2 {
		t.Fatalf("Reclassify: got %d rules, want 2", len(got.Reclassify))
	}
	if got.Reclassify[0].Match != "project-rule" {
		t.Errorf("Reclassify[0].Match: got %q, want %q (project rule must be first)",
			got.Reclassify[0].Match, "project-rule")
	}
	if got.Reclassify[1].Match != "scan-rule" {
		t.Errorf("Reclassify[1].Match: got %q, want %q", got.Reclassify[1].Match, "scan-rule")
	}
}

// ---- expandEnvVars ---------------------------------------------------------

func TestExpandEnvVars(t *testing.T) {
	t.Setenv("TEST_SCAN_ROOT", "/data/scans")

	cfg := &ScanConfigFile{
		Scan: ScanOptions{
			Paths: []string{"$TEST_SCAN_ROOT/repo1", "$TEST_SCAN_ROOT/repo2", "/absolute/path"},
		},
	}
	cfg.expandEnvVars()

	want := []string{"/data/scans/repo1", "/data/scans/repo2", "/absolute/path"}
	if diff := cmp.Diff(want, cfg.Scan.Paths); diff != "" {
		t.Errorf("Paths after expandEnvVars mismatch (-want +got):\n%s", diff)
	}
}

func TestExpandEnvVars_UnsetVar(t *testing.T) {
	// Unset env vars expand to empty string (standard os.ExpandEnv behaviour).
	cfg := &ScanConfigFile{
		Scan: ScanOptions{
			Paths: []string{"$DEFINITELY_UNSET_VAR_XYZ/repo"},
		},
	}
	cfg.expandEnvVars()

	if cfg.Scan.Paths[0] != "/repo" {
		t.Errorf("Paths[0]: got %q, want %q", cfg.Scan.Paths[0], "/repo")
	}
}
