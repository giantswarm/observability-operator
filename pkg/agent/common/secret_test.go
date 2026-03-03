package common

import (
	"testing"
)

func TestGenerateSecretData(t *testing.T) {
	tests := []struct {
		name      string
		secretEnv map[string]string
		wantErr   bool
	}{
		{
			name: "valid secret environment variables",
			secretEnv: map[string]string{
				"MIMIR_URL":      "https://mimir.example.com",
				"MIMIR_USERNAME": "test-cluster",
				"MIMIR_PASSWORD": "secret123",
			},
			wantErr: false,
		},
		{
			name:      "empty secret environment variables",
			secretEnv: map[string]string{},
			wantErr:   false,
		},
		{
			name:      "nil secret environment variables",
			secretEnv: nil,
			wantErr:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := GenerateSecretData(tt.secretEnv, "")
			if (err != nil) != tt.wantErr {
				t.Errorf("GenerateSecretData() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr && got == nil {
				t.Error("GenerateSecretData() returned nil data without error")
			}
		})
	}
}
