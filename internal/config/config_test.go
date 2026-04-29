package config

import (
	"os"
	"strings"
	"testing"
)

func TestParseUsersValid(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want map[string]string
	}{
		{"empty", "", map[string]string{}},
		{"whitespace only", "   ", map[string]string{}},
		{"single", "alice@example.com:secret", map[string]string{"alice@example.com": "secret"}},
		{"multiple", "a@x.com:p1,b@x.com:p2", map[string]string{"a@x.com": "p1", "b@x.com": "p2"}},
		{"trims whitespace", " a@x.com : p1 , b@x.com:p2 ", map[string]string{"a@x.com": "p1", "b@x.com": "p2"}},
		{"password with colon", "a@x.com:has:colon", map[string]string{"a@x.com": "has:colon"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParseUsers(tt.in)
			if err != nil {
				t.Fatalf("ParseUsers(%q) returned unexpected error: %v", tt.in, err)
			}
			if len(got) != len(tt.want) {
				t.Fatalf("ParseUsers(%q) len=%d want %d (got=%v)", tt.in, len(got), len(tt.want), got)
			}
			for k, v := range tt.want {
				if got[k] != v {
					t.Errorf("ParseUsers(%q)[%q] = %q, want %q", tt.in, k, got[k], v)
				}
			}
		})
	}
}

func TestParseUsersRejectsMalformed(t *testing.T) {
	tests := []struct {
		name      string
		in        string
		errSubstr string
	}{
		{"missing colon", "alice@example.com", "missing ':'"},
		{"empty email", ":secret", "empty email"},
		{"empty password", "alice@example.com:", "empty password"},
		{"whitespace-only email", "  :secret", "empty email"},
		{"whitespace-only password", "alice@example.com:   ", "empty password"},
		{"empty entry between commas", "a@x.com:p,,b@x.com:p", "empty"},
		{"trailing comma", "a@x.com:p,", "empty"},
		{"leading comma", ",a@x.com:p", "empty"},
		{"second entry malformed", "a@x.com:p,bogus", "missing ':'"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParseUsers(tt.in)
			if err == nil {
				t.Fatalf("ParseUsers(%q) succeeded with %v; want error", tt.in, got)
			}
			if got != nil {
				t.Errorf("ParseUsers(%q) returned non-nil map %v on error", tt.in, got)
			}
			if !strings.Contains(err.Error(), tt.errSubstr) {
				t.Errorf("ParseUsers(%q) error = %q, want substring %q", tt.in, err.Error(), tt.errSubstr)
			}
		})
	}
}

func TestFromEnvRejectsMalformedMockUsers(t *testing.T) {
	t.Setenv("MOCK_USERS", "alice@example.com")
	cfg, err := FromEnv()
	if err == nil {
		t.Fatalf("FromEnv() succeeded with cfg=%+v; want error for malformed MOCK_USERS", cfg)
	}
	if cfg != nil {
		t.Errorf("FromEnv() returned non-nil cfg %+v on error", cfg)
	}
	if !strings.Contains(err.Error(), "MOCK_USERS") {
		t.Errorf("FromEnv() error = %q, want substring %q", err.Error(), "MOCK_USERS")
	}
}

func TestFromEnvAcceptsValidMockUsers(t *testing.T) {
	t.Setenv("MOCK_USERS", "alice@example.com:secret,bob@example.com:hunter2")
	cfg, err := FromEnv()
	if err != nil {
		t.Fatalf("FromEnv() unexpected error: %v", err)
	}
	if cfg == nil {
		t.Fatal("FromEnv() returned nil cfg")
	}
	if cfg.MockUsers["alice@example.com"] != "secret" {
		t.Errorf("MockUsers[alice]=%q want secret", cfg.MockUsers["alice@example.com"])
	}
	if cfg.MockUsers["bob@example.com"] != "hunter2" {
		t.Errorf("MockUsers[bob]=%q want hunter2", cfg.MockUsers["bob@example.com"])
	}
}

func TestFromEnvUnsetMockUsers(t *testing.T) {
	os.Unsetenv("MOCK_USERS")
	cfg, err := FromEnv()
	if err != nil {
		t.Fatalf("FromEnv() unexpected error: %v", err)
	}
	if cfg == nil {
		t.Fatal("FromEnv() returned nil cfg")
	}
	if len(cfg.MockUsers) != 0 {
		t.Errorf("MockUsers=%v want empty", cfg.MockUsers)
	}
}
