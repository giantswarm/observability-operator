package auth

import (
	"crypto/rand"
	"crypto/sha1"
	"encoding/base64"
	"encoding/hex"
	"fmt"
)

// PasswordGenerator generates passwords and htpasswd entries
type PasswordGenerator interface {
	GeneratePassword(length int) (string, error)
	GenerateHtpasswd(username, password string) (string, error)
}

type simplePasswordGenerator struct{}

// NewPasswordGenerator creates a new password generator
func NewPasswordGenerator() PasswordGenerator {
	return &simplePasswordGenerator{}
}

func (g *simplePasswordGenerator) GeneratePassword(length int) (string, error) {
	bytes := make([]byte, length)
	if _, err := rand.Read(bytes); err != nil {
		return "", fmt.Errorf("failed to generate random bytes: %w", err)
	}
	return hex.EncodeToString(bytes), nil
}

func (g *simplePasswordGenerator) GenerateHtpasswd(username, password string) (string, error) {
	hash := sha1.Sum([]byte(password))
	encryptedPassword := base64.StdEncoding.EncodeToString(hash[:])
	return fmt.Sprintf("%s:{SHA}%s", username, encryptedPassword), nil
}
