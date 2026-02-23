package folder

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strings"
)

const (
	// UIDPrefix is the prefix for all operator-managed folder UIDs.
	// This follows the same convention as datasource UIDs (gs- prefix).
	UIDPrefix = "gs-"

	// Description is set on operator-managed folders for human readability.
	Description = "managed-by: observability-operator"

	// MaxTitleLength is the maximum length of a Grafana folder title.
	MaxTitleLength = 189

	// MaxDepth is the maximum nesting depth for folder hierarchies.
	MaxDepth = 4
)

// Folder represents a single Grafana folder managed by the operator.
type Folder struct {
	uid       string
	title     string
	parentUID string
	fullPath  string
}

// New creates a new Folder domain object.
func New(fullPath string, title string, parentUID string) *Folder {
	return &Folder{
		uid:       GenerateUID(fullPath),
		title:     title,
		parentUID: parentUID,
		fullPath:  fullPath,
	}
}

func (f *Folder) UID() string       { return f.uid }
func (f *Folder) Title() string     { return f.title }
func (f *Folder) ParentUID() string { return f.parentUID }
func (f *Folder) FullPath() string  { return f.fullPath }

// GenerateUID produces a deterministic UID from the full folder path.
// Uses gs- prefix + first 12 hex chars of SHA256(fullPath).
func GenerateUID(fullPath string) string {
	hash := sha256.Sum256([]byte(fullPath))
	return fmt.Sprintf("%s%s", UIDPrefix, hex.EncodeToString(hash[:12]))
}

// IsOperatorManaged checks if a folder UID belongs to the operator.
func IsOperatorManaged(uid string) bool {
	return strings.HasPrefix(uid, UIDPrefix)
}

// ParsePath splits a slash-separated path into individual Folder domain objects
// representing the hierarchy from root to leaf.
// E.g., "team-a/networking/alerts" -> [Folder{team-a}, Folder{team-a/networking}, Folder{team-a/networking/alerts}]
func ParsePath(path string) []*Folder {
	if path == "" {
		return nil
	}

	segments := strings.Split(path, "/")
	folders := make([]*Folder, 0, len(segments))

	for i, segment := range segments {
		fullPath := strings.Join(segments[:i+1], "/")
		parentUID := ""
		if i > 0 {
			parentUID = GenerateUID(strings.Join(segments[:i], "/"))
		}
		folders = append(folders, New(fullPath, segment, parentUID))
	}

	return folders
}

// ValidatePath checks that a folder path is well-formed.
// Returns an error if the path has leading/trailing slashes or empty segments.
func ValidatePath(path string) error {
	if path == "" {
		return nil
	}

	if strings.HasPrefix(path, "/") || strings.HasSuffix(path, "/") {
		return ErrInvalidFolderPath
	}

	segments := strings.Split(path, "/")

	if len(segments) > MaxDepth {
		return ErrFolderPathTooDeep
	}

	for _, segment := range segments {
		if segment == "" {
			return ErrInvalidFolderPath
		}
		if len(segment) > MaxTitleLength {
			return ErrFolderNameTooLong
		}
	}

	return nil
}
