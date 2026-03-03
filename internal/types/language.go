package types

import "github.com/go-enry/go-enry/v2"

// LanguageTypeToString converts enry.Type to string (programming, data, markup, prose)
func LanguageTypeToString(t enry.Type) string {
	switch t {
	case enry.Programming:
		return "programming"
	case enry.Data:
		return "data"
	case enry.Markup:
		return "markup"
	case enry.Prose:
		return "prose"
	default:
		return "unknown"
	}
}
