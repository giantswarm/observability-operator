package dashboard

import "errors"

var (
	// ErrInvalidJSON is returned when dashboard JSON cannot be parsed
	ErrInvalidJSON = errors.New("invalid JSON format")

	// ErrMissingUID is returned when dashboard doesn't have a UID
	ErrMissingUID = errors.New("dashboard UID is missing")

	// ErrMissingOrganization is returned when organization is not specified
	ErrMissingOrganization = errors.New("dashboard organization is missing")

	// ErrUnsupportedAPIVersion is returned when the dashboard apiVersion is not supported
	ErrUnsupportedAPIVersion = errors.New("unsupported dashboard apiVersion")
)
