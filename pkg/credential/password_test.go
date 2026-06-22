package credential

import (
	"crypto/sha1"
	"encoding/base64"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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

			parts := strings.SplitN(htpasswd, ":", 2)
			require.Len(t, parts, 2)
			assert.Equal(t, username, parts[0])

			expectedHash := sha1.Sum([]byte(password))
			assert.Equal(t, "{SHA}"+base64.StdEncoding.EncodeToString(expectedHash[:]), parts[1])
		})

		t.Run("should be deterministic for same input", func(t *testing.T) {
			h1, err := generator.GenerateHtpasswd("u", "p")
			require.NoError(t, err)
			h2, err := generator.GenerateHtpasswd("u", "p")
			require.NoError(t, err)
			assert.Equal(t, h1, h2)
		})
	})
}
