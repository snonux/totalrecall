package audio

import (
	"strings"
	"testing"
)

func TestValidateBulgarianText(t *testing.T) {
	tests := []struct {
		name    string
		text    string
		wantErr bool
		errMsg  string
	}{
		{
			name:    "valid Bulgarian word",
			text:    "ябълка",
			wantErr: false,
		},
		{
			name:    "valid Bulgarian sentence",
			text:    "Здравей, как си?",
			wantErr: false,
		},
		{
			name:    "empty text",
			text:    "",
			wantErr: true,
			errMsg:  "text cannot be empty",
		},
		{
			name:    "whitespace only",
			text:    "   \t\n",
			wantErr: true,
			errMsg:  "text cannot be empty",
		},
		{
			name:    "English text",
			text:    "Hello world",
			wantErr: true,
			errMsg:  "text must contain Cyrillic characters",
		},
		{
			name:    "numbers only",
			text:    "12345",
			wantErr: true,
			errMsg:  "text must contain Cyrillic characters",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateBulgarianText(tt.text)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateBulgarianText() error = %v, wantErr %v", err, tt.wantErr)
			}
			if tt.wantErr && err != nil {
				if !strings.Contains(err.Error(), tt.errMsg) {
					t.Errorf("ValidateBulgarianText() error = %v, want error containing %v", err.Error(), tt.errMsg)
				}
			}
		})
	}
}
