//go:build online

// Live deps.dev integration test for currency resolution. Excluded from the
// default suite (it calls api.deps.dev). Run with:
//
//	go test -tags online ./internal/currency/ -run Live
package currency

import (
	"errors"
	"testing"
)

func TestLiveDepsDevResolver(t *testing.T) {
	r := NewDepsDevResolver("") // public deps.dev

	// A well-known package across the dominant ecosystems must resolve.
	cases := []struct{ system, name string }{
		{"npm", "lodash"},
		{"maven", "org.springframework:spring-core"},
		{"nuget", "newtonsoft.json"},
		{"pypi", "requests"},
	}
	for _, c := range cases {
		info, err := r.LatestVersion(c.system, c.name)
		if err != nil {
			t.Errorf("%s/%s: unexpected error %v", c.system, c.name, err)
			continue
		}
		if info.Latest == "" {
			t.Errorf("%s/%s: empty latest version", c.system, c.name)
		}
		t.Logf("%s/%s -> latest=%s deprecated=%v", c.system, c.name, info.Latest, info.IsDeprecated)
	}

	// A definitely-nonexistent package must return ErrNotFound.
	if _, err := r.LatestVersion("npm", "this-package-should-not-exist-xyzzy-42"); !errors.Is(err, ErrNotFound) {
		t.Errorf("nonexistent package: err=%v, want ErrNotFound", err)
	}
}
