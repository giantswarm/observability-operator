package credential

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestValidateMode(t *testing.T) {
	tests := []struct {
		input   string
		want    Mode
		wantErr bool
	}{
		{input: "basicAuth", want: ModeBasicAuth},
		{input: "none", want: ModeNone},
		{input: "", wantErr: true},
		{input: "BasicAuth", wantErr: true}, // case-sensitive
		{input: "oauth", wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got, err := ValidateMode(tt.input)
			if tt.wantErr {
				assert.Error(t, err)
				return
			}
			assert.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}
