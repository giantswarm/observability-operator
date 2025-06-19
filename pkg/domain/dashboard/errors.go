package dashboard

import "errors"

var (
	ErrMissingUID         = errors.New("dashboard UID not found in configmap")
	ErrMissingOrganization = errors.New("organization label not found in configmap")
	ErrInvalidJSON        = errors.New("failed converting dashboard to json")
)
