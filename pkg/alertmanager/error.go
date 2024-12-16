package alertmanager

import "fmt"

type APIError struct {
	Code    int
	Message string
}

func (e APIError) Error() string {
	return fmt.Sprintf("%d: %s", e.Code, e.Message)
}
