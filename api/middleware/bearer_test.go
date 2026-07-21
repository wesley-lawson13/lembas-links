package middleware

import "testing"

func TestExtractBearerToken(t *testing.T) {
	tests := []struct {
		name   string
		header string
		want   string
	}{
		{"with prefix", "Bearer abc123", "abc123"},
		{"without prefix", "abc123", "abc123"},
		{"empty string", "", ""},
		{"prefix with empty token", "Bearer ", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ExtractBearerToken(tt.header)
			if got != tt.want {
				t.Errorf("ExtractBearerToken(%q) = %q, want %q", tt.header, got, tt.want)
			}
		})
	}
}
