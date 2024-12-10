package alertmanager

type APIError struct {
	Code    int
	Message string
}

func (e APIError) Error() string {
	return e.Message
}
