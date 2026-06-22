package folder

import (
	"errors"
	"fmt"
)

var (
	// ErrInvalidFolderPath is returned when the folder path has leading/trailing slashes or empty segments.
	ErrInvalidFolderPath = errors.New("invalid folder path: must not have leading/trailing slashes or empty segments")

	// ErrFolderNameTooLong is returned when a folder name segment exceeds the Grafana title limit.
	ErrFolderNameTooLong = fmt.Errorf("invalid folder path: segment exceeds maximum length of %d characters", MaxTitleLength)

	// ErrFolderPathTooDeep is returned when the folder path exceeds the maximum nesting depth.
	ErrFolderPathTooDeep = fmt.Errorf("invalid folder path: exceeds maximum nesting depth of %d", MaxDepth)
)
