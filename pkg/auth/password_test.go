package auth

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/crypto/bcrypt"
)

func TestPasswordGenerator(t *testing.T) {
	generator := NewPasswordGenerator()

	t.Run("GeneratePassword", func(t *testing.T) {
		t.Run("should generate password of correct length", func(t *testing.T) {
			password, err := generator.GeneratePassword(32)
			require.NoError(t, err)
			assert.NotEmpty(t, password)
			// Hex encoding doubles the length
			assert.Equal(t, 64, len(password))
		})
		t.Run("should generate different passwords", func(t *testing.T) {
			password1, err := generator.GeneratePassword(16)
			require.NoError(t, err)

			password2, err := generator.GeneratePassword(16)
			require.NoError(t, err)

			assert.NotEqual(t, password1, password2)
		})

		t.Run("should handle zero length", func(t *testing.T) {
			password, err := generator.GeneratePassword(0)
			require.NoError(t, err)
			assert.Empty(t, password)
		})
	})

	t.Run("GenerateHtpasswd", func(t *testing.T) {
		t.Run("should generate valid htpasswd entry", func(t *testing.T) {
			username := "test-cluster"
			password := "test-password"

			htpasswd, err := generator.GenerateHtpasswd(username, password)
			require.NoError(t, err)

			// Should be in format username:encrypted_password
			parts := splitHtpasswd(htpasswd)
			require.Len(t, parts, 2)
			assert.Equal(t, username, parts[0])

			// Verify the password can be verified with bcrypt
			err = bcrypt.CompareHashAndPassword([]byte(parts[1]), []byte(password))
			assert.NoError(t, err)
		})

		t.Run("should generate different hashes for same password", func(t *testing.T) {
			username := "test-cluster"
			password := "test-password"

			htpasswd1, err := generator.GenerateHtpasswd(username, password)
			require.NoError(t, err)

			htpasswd2, err := generator.GenerateHtpasswd(username, password)
			require.NoError(t, err)

			// Hashes should be different due to salt
			assert.NotEqual(t, htpasswd1, htpasswd2)

			// But both should verify the same password
			parts1 := splitHtpasswd(htpasswd1)
			parts2 := splitHtpasswd(htpasswd2)

			err = bcrypt.CompareHashAndPassword([]byte(parts1[1]), []byte(password))
			assert.NoError(t, err)
			err = bcrypt.CompareHashAndPassword([]byte(parts2[1]), []byte(password))
			assert.NoError(t, err)
		})

		t.Run("should handle empty username", func(t *testing.T) {
			htpasswd, err := generator.GenerateHtpasswd("", "password")
			require.NoError(t, err)

			parts := splitHtpasswd(htpasswd)
			require.Len(t, parts, 2)
			assert.Empty(t, parts[0])
		})

		t.Run("should handle empty password", func(t *testing.T) {
			htpasswd, err := generator.GenerateHtpasswd("username", "")
			require.NoError(t, err)

			parts := splitHtpasswd(htpasswd)
			require.Len(t, parts, 2)
			assert.Equal(t, "username", parts[0])

			// Should be able to verify empty password
			err = bcrypt.CompareHashAndPassword([]byte(parts[1]), []byte(""))
			assert.NoError(t, err)
		})
	})
}

// Helper function to split htpasswd entry into username and hash
func splitHtpasswd(htpasswd string) []string {
	return strings.SplitN(htpasswd, ":", 2)
}
