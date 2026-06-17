package java

import (
	"github.com/petrarca/tech-stack-analyzer/internal/scanner/components"
	"github.com/petrarca/tech-stack-analyzer/internal/scanner/parsers"
)

// mavenSourceIndexer indexes every pom.xml in the scanned tree by its
// "groupId:artifactId" coordinate, so imported BOMs (scope=import) can be
// located on disk and their managed versions read -- the offline equivalent of
// Maven resolving a BOM artifact from a repository.
type mavenSourceIndexer struct{}

func (mavenSourceIndexer) Ecosystem() string { return "maven" }

func (mavenSourceIndexer) Matches(fileName string) bool { return fileName == "pom.xml" }

// Coordinate returns the POM's "groupId:artifactId". groupId falls back to the
// parent's when the POM omits its own (Maven inheritance).
func (mavenSourceIndexer) Coordinate(content []byte) string {
	info := parsers.NewMavenParser().ExtractProjectInfo(string(content))
	groupID := info.GroupId
	if groupID == "" {
		groupID = info.Parent.GroupId
	}
	if groupID == "" || info.ArtifactId == "" {
		return ""
	}
	return groupID + ":" + info.ArtifactId
}

func init() {
	components.RegisterSourceIndexer(mavenSourceIndexer{})
}
