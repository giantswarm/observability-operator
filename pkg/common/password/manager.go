package password

import (
	"crypto/rand"
	"encoding/hex"
)

type Manager interface {
	GeneratePassword(length int) (string, error)
}

type SimpleManager struct {
}

func (m SimpleManager) GeneratePassword(length int) (string, error) {
	bytes := make([]byte, length)
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}
	return hex.EncodeToString(bytes), nil
}
