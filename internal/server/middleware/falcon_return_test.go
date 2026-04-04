package middleware

import "testing"

func TestValidateFalconReturn(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{"empty", "", ""},
		{"valid orb.local", "https://nginx.falcon-backend.orb.local/processing/cases", "https://nginx.falcon-backend.orb.local/processing/cases"},
		{"valid localhost", "http://localhost:3000/dashboard", "http://localhost:3000/dashboard"},
		{"valid 127.0.0.1", "http://127.0.0.1:8080/page", "http://127.0.0.1:8080/page"},
		{"blocked external", "https://evil.com/phish", ""},
		{"blocked javascript", "javascript:alert(1)", ""},
		{"blocked data uri", "data:text/html,<script>alert(1)</script>", ""},
		{"blocked no scheme", "//evil.com/phish", ""},
		{"orb.local subdomain", "https://sf-mock.mock-salesforce.orb.local/lightning/o/Case/list", "https://sf-mock.mock-salesforce.orb.local/lightning/o/Case/list"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ValidateFalconReturn(tt.input)
			if got != tt.want {
				t.Errorf("ValidateFalconReturn(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}
