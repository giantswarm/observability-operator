package grafana

import (
	"maps"
)

// Datasource represents a Grafana datasource.
type Datasource struct {
	ID             int64
	UID            string
	Name           string
	Type           string
	URL            string
	IsDefault      bool
	Access         string
	JSONData       map[string]any
	SecureJSONData map[string]string
}

// Merge merges the non-zero fields from src into d and returns the result.
// For JSONData and SecureJSONData maps, it merges the key-value pairs, with src's values taking precedence in case of key conflicts.
func (d Datasource) Merge(src Datasource) Datasource {
	if !IsZeroVar(src.ID) {
		d.ID = src.ID
	}
	if !IsZeroVar(src.UID) {
		d.UID = src.UID
	}
	if !IsZeroVar(src.Name) {
		d.Name = src.Name
	}
	if !IsZeroVar(src.Type) {
		d.Type = src.Type
	}
	if !IsZeroVar(src.URL) {
		d.URL = src.URL
	}
	if d.IsDefault != src.IsDefault {
		d.IsDefault = src.IsDefault
	}
	if !IsZeroVar(src.Access) {
		d.Access = src.Access
	}
	if src.JSONData != nil {
		if d.JSONData == nil {
			d.JSONData = make(map[string]any)
		}
		maps.Copy(d.JSONData, src.JSONData)
	}
	if src.SecureJSONData != nil {
		if d.SecureJSONData == nil {
			d.SecureJSONData = make(map[string]string)
		}
		maps.Copy(d.SecureJSONData, src.SecureJSONData)
	}

	return d
}

// IsZeroVar reports whether v is the zero value for its type.
func IsZeroVar[T comparable](v T) bool {
	var zero T
	return v == zero
}
