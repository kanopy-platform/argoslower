package iplist

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestValidateCIDRs(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		input   []string
		wantErr bool
	}{
		{
			name:    "valid",
			input:   []string{"1.2.3.4/32", "2001:db8:a0b:12f0::1/32"},
			wantErr: false,
		},
		{
			name:    "missing suffix",
			input:   []string{"1.2.3.4"},
			wantErr: true,
		},
		{
			name:    "invalid",
			input:   []string{"abc"},
			wantErr: true,
		},
	}

	for _, test := range tests {
		err := ValidateCIDRs(test.input)
		assert.Equal(t, test.wantErr, err != nil, test.name)
	}
}
