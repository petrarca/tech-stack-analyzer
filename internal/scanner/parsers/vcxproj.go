package parsers

import (
	"encoding/xml"
	"path/filepath"
	"strings"
)

// VcxprojParser handles .vcxproj file parsing (MSBuild C++ projects)
type VcxprojParser struct{}

// VcxprojProject represents a parsed .vcxproj project
type VcxprojProject struct {
	Name                         string
	PlatformToolset              string // e.g. v100, v110, v141, v143
	ConfigurationType            string // Application, DynamicLibrary, StaticLibrary
	UseOfMfc                     string // Dynamic, Static, or empty
	CLRSupport                   string // true, Pure, Safe, or empty
	CharacterSet                 string // MultiByte, Unicode
	WindowsTargetPlatformVersion string
	AdditionalDependencies       []string // linked .lib files
	ProjectReferences            []string // referenced .vcxproj paths
}

// vcxprojRoot is the top-level MSBuild XML structure for .vcxproj files
type vcxprojRoot struct {
	XMLName              xml.Name                     `xml:"Project"`
	PropertyGroups       []vcxprojPropertyGroup       `xml:"PropertyGroup"`
	ItemGroups           []vcxprojItemGroup           `xml:"ItemGroup"`
	ItemDefinitionGroups []vcxprojItemDefinitionGroup `xml:"ItemDefinitionGroup"`
}

type vcxprojPropertyGroup struct {
	Label                        string `xml:"Label,attr"`
	Condition                    string `xml:"Condition,attr"`
	RootNamespace                string `xml:"RootNamespace"`
	ProjectName                  string `xml:"ProjectName"`
	ConfigurationType            string `xml:"ConfigurationType"`
	PlatformToolset              string `xml:"PlatformToolset"`
	UseOfMfc                     string `xml:"UseOfMfc"`
	CLRSupport                   string `xml:"CLRSupport"`
	CharacterSet                 string `xml:"CharacterSet"`
	WindowsTargetPlatformVersion string `xml:"WindowsTargetPlatformVersion"`
}

type vcxprojItemGroup struct {
	ProjectReferences []vcxprojProjectRef `xml:"ProjectReference"`
}

type vcxprojProjectRef struct {
	Include string `xml:"Include,attr"`
}

type vcxprojItemDefinitionGroup struct {
	Condition string         `xml:"Condition,attr"`
	Link      vcxprojLinkDef `xml:"Link"`
}

type vcxprojLinkDef struct {
	AdditionalDependencies string `xml:"AdditionalDependencies"`
}

// NewVcxprojParser creates a new VcxprojParser instance
func NewVcxprojParser() *VcxprojParser {
	return &VcxprojParser{}
}

// ParseVcxproj parses a .vcxproj file and extracts project information.
// For PlatformToolset, the Release configuration is preferred over Debug
// as it represents the shipping build. For all other fields, first non-empty
// value across PropertyGroups wins.
func (p *VcxprojParser) ParseVcxproj(content, filePath string) VcxprojProject {
	var project VcxprojProject

	var root vcxprojRoot
	if err := xml.Unmarshal([]byte(content), &root); err != nil {
		project.Name = p.nameFromPath(filePath)
		return project
	}

	p.extractPropertyGroups(&project, root.PropertyGroups)

	if project.Name == "" {
		project.Name = p.nameFromPath(filePath)
	}

	project.AdditionalDependencies = p.collectAdditionalDependencies(root.ItemDefinitionGroups)
	project.ProjectReferences = p.collectProjectReferences(root.ItemGroups)

	return project
}

// extractPropertyGroups scans all PropertyGroups using first-non-empty-wins
// semantics. For PlatformToolset, the Release configuration is preferred.
// For Name, ProjectName takes precedence over RootNamespace.
func (p *VcxprojParser) extractPropertyGroups(project *VcxprojProject, pgs []vcxprojPropertyGroup) {
	var releasePlatformToolset string
	var rootNamespace, projectName string

	for _, pg := range pgs {
		setFirstNonEmpty(&rootNamespace, pg.RootNamespace)
		setFirstNonEmpty(&projectName, pg.ProjectName)
		setFirstNonEmpty(&project.ConfigurationType, pg.ConfigurationType)
		setFirstNonEmpty(&project.UseOfMfc, pg.UseOfMfc)
		setFirstNonEmpty(&project.CLRSupport, pg.CLRSupport)
		setFirstNonEmpty(&project.CharacterSet, pg.CharacterSet)
		setFirstNonEmpty(&project.WindowsTargetPlatformVersion, pg.WindowsTargetPlatformVersion)

		if pg.PlatformToolset != "" {
			if strings.Contains(strings.ToLower(pg.Condition), "release") {
				releasePlatformToolset = pg.PlatformToolset
			}
			setFirstNonEmpty(&project.PlatformToolset, pg.PlatformToolset)
		}
	}

	if releasePlatformToolset != "" {
		project.PlatformToolset = releasePlatformToolset
	}

	// ProjectName takes precedence over RootNamespace.
	switch {
	case projectName != "":
		project.Name = projectName
	case rootNamespace != "":
		project.Name = rootNamespace
	}
}

// setFirstNonEmpty sets *target to value if *target is still empty and value is non-empty.
func setFirstNonEmpty(target *string, value string) {
	if *target == "" && value != "" {
		*target = value
	}
}

// collectProjectReferences extracts referenced .vcxproj paths from ItemGroups.
func (p *VcxprojParser) collectProjectReferences(itemGroups []vcxprojItemGroup) []string {
	var refs []string
	for _, ig := range itemGroups {
		for _, ref := range ig.ProjectReferences {
			if ref.Include != "" {
				refs = append(refs, ref.Include)
			}
		}
	}
	return refs
}

// collectAdditionalDependencies merges linked lib lists from all ItemDefinitionGroup/Link
// elements, deduplicating entries and skipping MSBuild variable references.
func (p *VcxprojParser) collectAdditionalDependencies(idgs []vcxprojItemDefinitionGroup) []string {
	seen := make(map[string]bool)
	var libs []string

	for _, idg := range idgs {
		raw := idg.Link.AdditionalDependencies
		if raw == "" {
			continue
		}
		for _, part := range strings.Split(raw, ";") {
			part = strings.TrimSpace(part)
			// Skip empty strings and MSBuild variable references like %(AdditionalDependencies)
			if part == "" || strings.Contains(part, "%(") {
				continue
			}
			if !seen[part] {
				seen[part] = true
				libs = append(libs, part)
			}
		}
	}

	return libs
}

// nameFromPath extracts a project name from the .vcxproj file path,
// stripping the extension and common Visual Studio year suffixes.
func (p *VcxprojParser) nameFromPath(filePath string) string {
	base := filepath.Base(filePath)
	name := strings.TrimSuffix(base, ".vcxproj")
	// Strip common VS year suffixes added to filenames (e.g. "HDApi 2010.vcxproj")
	for _, suffix := range []string{" 2010", " 2012", " 2013", " 2015", " 2017", " 2019", " 2022"} {
		name = strings.TrimSuffix(name, suffix)
	}
	return name
}

// PlatformToolsetToVSVersion maps PlatformToolset values to Visual Studio versions.
// Returns an empty string for unknown toolset values.
func PlatformToolsetToVSVersion(toolset string) string {
	switch toolset {
	case "v100":
		return "VS2010"
	case "v110":
		return "VS2012"
	case "v120":
		return "VS2013"
	case "v140":
		return "VS2015"
	case "v141":
		return "VS2017"
	case "v142":
		return "VS2019"
	case "v143":
		return "VS2022"
	default:
		return ""
	}
}
