package folder_test

import (
	"strings"
	"testing"

	"github.com/giantswarm/observability-operator/pkg/domain/folder"
)

func TestGenerateUID(t *testing.T) {
	tests := []struct {
		name     string
		path     string
		wantPfx  string
		wantLen  int
		wantSame bool // if true, compare with second call for determinism
	}{
		{
			name:     "simple path",
			path:     "team-a",
			wantPfx:  "gs-",
			wantLen:  15, // "gs-" (3) + 12 hex chars
			wantSame: true,
		},
		{
			name:     "nested path",
			path:     "team-a/networking/alerts",
			wantPfx:  "gs-",
			wantLen:  15,
			wantSame: true,
		},
		{
			name:     "empty path",
			path:     "",
			wantPfx:  "gs-",
			wantLen:  15,
			wantSame: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			uid := folder.GenerateUID(tt.path)

			if len(uid) != tt.wantLen {
				t.Errorf("GenerateUID(%q) length = %d, want %d (uid=%q)", tt.path, len(uid), tt.wantLen, uid)
			}

			if uid[:3] != tt.wantPfx {
				t.Errorf("GenerateUID(%q) prefix = %q, want %q", tt.path, uid[:3], tt.wantPfx)
			}

			if tt.wantSame {
				uid2 := folder.GenerateUID(tt.path)
				if uid != uid2 {
					t.Errorf("GenerateUID(%q) not deterministic: %q != %q", tt.path, uid, uid2)
				}
			}
		})
	}
}

func TestGenerateUID_Uniqueness(t *testing.T) {
	paths := []string{
		"team-a",
		"team-b",
		"team-a/networking",
		"team-b/networking",
		"team-a/networking/alerts",
		"team-a/monitoring/alerts",
	}

	uids := make(map[string]string) // uid -> path
	for _, path := range paths {
		uid := folder.GenerateUID(path)
		if existingPath, exists := uids[uid]; exists {
			t.Errorf("UID collision: %q and %q both produce %q", existingPath, path, uid)
		}
		uids[uid] = path
	}
}

func TestIsOperatorManaged(t *testing.T) {
	tests := []struct {
		name string
		uid  string
		want bool
	}{
		{
			name: "operator-managed folder",
			uid:  folder.GenerateUID("team-a"),
			want: true,
		},
		{
			name: "user-created folder with random UID",
			uid:  "abc123def456",
			want: false,
		},
		{
			name: "user-created folder with similar prefix",
			uid:  "gsx-something",
			want: false,
		},
		{
			name: "empty UID",
			uid:  "",
			want: false,
		},
		{
			name: "just the prefix",
			uid:  "gs-",
			want: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := folder.IsOperatorManaged(tt.uid); got != tt.want {
				t.Errorf("IsOperatorManaged(%q) = %v, want %v", tt.uid, got, tt.want)
			}
		})
	}
}

func TestParsePath(t *testing.T) {
	tests := []struct {
		name          string
		path          string
		wantCount     int
		wantTitles    []string
		wantPaths     []string
		wantParentNil []bool // true = parentUID should be empty
	}{
		{
			name:          "empty path",
			path:          "",
			wantCount:     0,
			wantTitles:    nil,
			wantPaths:     nil,
			wantParentNil: nil,
		},
		{
			name:          "single segment",
			path:          "team-a",
			wantCount:     1,
			wantTitles:    []string{"team-a"},
			wantPaths:     []string{"team-a"},
			wantParentNil: []bool{true},
		},
		{
			name:          "two segments",
			path:          "team-a/networking",
			wantCount:     2,
			wantTitles:    []string{"team-a", "networking"},
			wantPaths:     []string{"team-a", "team-a/networking"},
			wantParentNil: []bool{true, false},
		},
		{
			name:          "three segments",
			path:          "team-a/networking/alerts",
			wantCount:     3,
			wantTitles:    []string{"team-a", "networking", "alerts"},
			wantPaths:     []string{"team-a", "team-a/networking", "team-a/networking/alerts"},
			wantParentNil: []bool{true, false, false},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			folders := folder.ParsePath(tt.path)

			if len(folders) != tt.wantCount {
				t.Fatalf("ParsePath(%q) returned %d folders, want %d", tt.path, len(folders), tt.wantCount)
			}

			for i, f := range folders {
				if f.Title() != tt.wantTitles[i] {
					t.Errorf("folder[%d].Title() = %q, want %q", i, f.Title(), tt.wantTitles[i])
				}
				if f.FullPath() != tt.wantPaths[i] {
					t.Errorf("folder[%d].FullPath() = %q, want %q", i, f.FullPath(), tt.wantPaths[i])
				}
				if tt.wantParentNil[i] && f.ParentUID() != "" {
					t.Errorf("folder[%d].ParentUID() = %q, want empty (root)", i, f.ParentUID())
				}
				if !tt.wantParentNil[i] && f.ParentUID() == "" {
					t.Errorf("folder[%d].ParentUID() should not be empty", i)
				}
			}
		})
	}
}

func TestParsePath_ParentUIDChaining(t *testing.T) {
	folders := folder.ParsePath("a/b/c")

	if len(folders) != 3 {
		t.Fatalf("Expected 3 folders, got %d", len(folders))
	}

	// First folder has no parent
	if folders[0].ParentUID() != "" {
		t.Errorf("Root folder should have empty parentUID, got %q", folders[0].ParentUID())
	}

	// Second folder's parent should be first folder's UID
	if folders[1].ParentUID() != folders[0].UID() {
		t.Errorf("Second folder parentUID = %q, want %q (first folder UID)", folders[1].ParentUID(), folders[0].UID())
	}

	// Third folder's parent should be second folder's UID
	if folders[2].ParentUID() != folders[1].UID() {
		t.Errorf("Third folder parentUID = %q, want %q (second folder UID)", folders[2].ParentUID(), folders[1].UID())
	}
}

func TestNew(t *testing.T) {
	f := folder.New("team-a/networking", "networking", folder.GenerateUID("team-a"))

	if f.UID() != folder.GenerateUID("team-a/networking") {
		t.Errorf("UID = %q, want %q", f.UID(), folder.GenerateUID("team-a/networking"))
	}
	if f.Title() != "networking" {
		t.Errorf("Title = %q, want %q", f.Title(), "networking")
	}
	if f.ParentUID() != folder.GenerateUID("team-a") {
		t.Errorf("ParentUID = %q, want %q", f.ParentUID(), folder.GenerateUID("team-a"))
	}
	if f.FullPath() != "team-a/networking" {
		t.Errorf("FullPath = %q, want %q", f.FullPath(), "team-a/networking")
	}
}

func TestValidatePath(t *testing.T) {
	tests := []struct {
		name    string
		path    string
		wantErr bool
	}{
		{
			name:    "empty path is valid",
			path:    "",
			wantErr: false,
		},
		{
			name:    "simple path is valid",
			path:    "team-a",
			wantErr: false,
		},
		{
			name:    "nested path is valid",
			path:    "team-a/networking/alerts",
			wantErr: false,
		},
		{
			name:    "leading slash is invalid",
			path:    "/team-a",
			wantErr: true,
		},
		{
			name:    "trailing slash is invalid",
			path:    "team-a/",
			wantErr: true,
		},
		{
			name:    "double slash is invalid",
			path:    "team-a//alerts",
			wantErr: true,
		},
		{
			name:    "leading and trailing slashes",
			path:    "/team-a/",
			wantErr: true,
		},
		{
			name:    "segment exceeds max title length",
			path:    "team-a/" + strings.Repeat("a", folder.MaxTitleLength+1),
			wantErr: true,
		},
		{
			name:    "segment at max title length is valid",
			path:    strings.Repeat("a", folder.MaxTitleLength),
			wantErr: false,
		},
		{
			name:    "path exceeds max depth",
			path:    "a/b/c/d/e",
			wantErr: true,
		},
		{
			name:    "path at max depth is valid",
			path:    "a/b/c/d",
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := folder.ValidatePath(tt.path)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidatePath(%q) error = %v, wantErr %v", tt.path, err, tt.wantErr)
			}
		})
	}
}
