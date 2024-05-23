package password

import (
	"crypto/rand"
	"encoding/hex"
	"os/exec"
	"strings"
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
		return "", err
	}
	return hex.EncodeToString(bytes), nil
}

func (m SimpleManager) GenerateHtpasswd(username string, password string) (string, error) {
	htpasswd, err := exec.Command("htpasswd", "-bn", username, password).Output()
	if err != nil {
		return "", err
	}
	formattedHtpasswd := strings.TrimSpace(string(htpasswd))
	return formattedHtpasswd, nil
}
