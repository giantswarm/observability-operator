package alertmanager

import "fmt"

// APIError represents an error response from the Mimir Alertmanager API,
// capturing the HTTP status code and response body for diagnostics.
type APIError struct {
	Code    int
	Message string
}

func (e APIError) Error() string {
	return fmt.Sprintf("%d: %s", e.Code, e.Message)
}
