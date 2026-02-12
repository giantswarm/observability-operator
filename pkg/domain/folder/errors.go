package folder

import "errors"

var (
	// ErrInvalidFolderPath is returned when the folder path has leading/trailing slashes or empty segments.
	ErrInvalidFolderPath = errors.New("invalid folder path: must not have leading/trailing slashes or empty segments")
)
