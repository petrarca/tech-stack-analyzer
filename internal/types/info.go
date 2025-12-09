package types

// TechInfo holds information about a technology
type TechInfo struct {
	Name        string                 `json:"name"`
	Tech        string                 `json:"tech"`
	Category    string                 `json:"category"`
	Description string                 `json:"description,omitempty"`
	Properties  map[string]interface{} `json:"properties,omitempty"`
}

// CategoryInfo represents a single category entry
type CategoryInfo struct {
	Name        string `json:"name"`
	IsComponent bool   `json:"is_component"`
	Description string `json:"description"`
}
