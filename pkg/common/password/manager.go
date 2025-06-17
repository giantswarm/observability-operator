package password

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"

	"golang.org/x/crypto/bcrypt"
)

type Manager interface {
	GeneratePassword(length int) (string, error)
	GenerateHtpasswd(username string, password string) (string, error)
}

type SimpleManager struct {
}

func (m SimpleManager) GeneratePassword(length int) (string, error) {
	bytes := make([]byte, length)
	if _, err := rand.Read(bytes); err != nil {
		return "", fmt.Errorf("failed to generate random bytes for password: %w", err)
	}
	return hex.EncodeToString(bytes), nil
}

func (m SimpleManager) GenerateHtpasswd(username, password string) (string, error) {
	encryptedPassword, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return "", fmt.Errorf("failed to generate bcrypt hash for username %s: %w", username, err)
	}
	formattedHtpasswd := fmt.Sprintf("%s:%s", username, string(encryptedPassword))
	return formattedHtpasswd, nil
}
