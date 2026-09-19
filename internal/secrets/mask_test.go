package secrets

import (
	"strings"
	"testing"
)

func TestMaskRedactsCommonSecrets(t *testing.T) {
	tests := []struct {
		name      string
		in        string
		leaked    string   // substring that must NOT survive masking
		wantKinds []string // distinct kinds expected (sorted)
		wantKept  []string // substrings that must remain (non-secret context)
	}{
		{
			name:      "openai api key",
			in:        "provider rejected key sk-proj-AbCdEf0123456789ghIJKLmnop with 401",
			leaked:    "sk-proj-AbCdEf0123456789",
			wantKinds: []string{"api_key"},
			wantKept:  []string{"provider rejected key", "with 401", "[REDACTED:api_key]"},
		},
		{
			name:      "anthropic api key",
			in:        "Authorization failed for sk-ant-api03-AbCdEf0123456789ghIJKLmnopqrst",
			leaked:    "sk-ant-api03-AbCdEf0123456789",
			wantKinds: []string{"api_key"},
		},
		{
			name:      "bearer jwt keeps keyword and jwt wins",
			in:        "upstream 403: Bearer eyJhbGciOi.eyJzdWIabc.ghi789xyzAB refused",
			leaked:    "eyJhbGciOi.eyJzdWIabc.ghi789xyzAB",
			wantKinds: []string{"jwt"}, // jwt pattern wins over generic bearer
			wantKept:  []string{"Bearer", "refused"},
		},
		{
			name:      "opaque bearer token",
			in:        "header was Bearer abcDEF123456ghiJKL789 here",
			leaked:    "abcDEF123456ghiJKL789",
			wantKinds: []string{"token"},
			wantKept:  []string{"Bearer ", "[REDACTED:token]", "here"},
		},
		{
			name:      "jwt standalone",
			in:        "token=eyJhbGciOiJIUzI1NiJ9.eyJzdWIiOiIxMjM0NTYifQ.SflKxwRJSMeKKF2QT4",
			leaked:    "eyJhbGciOiJIUzI1NiJ9.eyJzdWIiOiIxMjM0NTYifQ.SflKxwRJSMeKKF2QT4",
			wantKinds: []string{"jwt"},
		},
		{
			name:      "postgres url password",
			in:        "dial error: postgres://app_user:s3cr3tP@ss@db.internal:5432/tokenizer failed",
			leaked:    "s3cr3tP@ss",
			wantKinds: []string{"db_password"},
			wantKept:  []string{"postgres://app_user:", "@db.internal:5432/tokenizer", "[REDACTED:db_password]"},
		},
		{
			name:      "redis url password",
			in:        "redis://default:hunter2hunter2@cache:6379/0 timeout",
			leaked:    "hunter2hunter2",
			wantKinds: []string{"db_password"},
			wantKept:  []string{"redis://default:", "@cache:6379/0"},
		},
		{
			name:      "key value password",
			in:        "config invalid: password=Sup3rSecret! and host=localhost",
			leaked:    "Sup3rSecret!",
			wantKinds: []string{"secret_kv"},
			wantKept:  []string{"password=", "host=localhost"},
		},
		{
			name:      "api_key kv",
			in:        `{"api_key":"abcd1234efgh5678","model":"auto"}`,
			leaked:    "abcd1234efgh5678",
			wantKinds: []string{"secret_kv"},
			wantKept:  []string{`"model":"auto"`},
		},
		{
			name:      "aws access key",
			in:        "denied for AKIAIOSFODNN7EXAMPLE in us-east-1",
			leaked:    "AKIAIOSFODNN7EXAMPLE",
			wantKinds: []string{"aws_key"},
			wantKept:  []string{"in us-east-1"},
		},
		{
			name:      "github token",
			in:        "clone failed using ghp_abcdefghijklmnopqrstuvwxyz0123456789 token",
			leaked:    "ghp_abcdefghijklmnopqrstuvwxyz0123456789",
			wantKinds: []string{"github_token"},
		},
		{
			name:      "multiple secrets in one string",
			in:        "postgres://u:pw_secret_value@h/db and api_key=KEYVALUE123456 plus Bearer tok12345678",
			leaked:    "pw_secret_value",
			wantKinds: []string{"db_password", "secret_kv", "token"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := Mask(tt.in)
			if !got.Masked() {
				t.Fatalf("expected secret to be masked, got %q", got.Text)
			}
			if tt.leaked != "" && strings.Contains(got.Text, tt.leaked) {
				t.Fatalf("secret leaked through masking: %q still contains %q", got.Text, tt.leaked)
			}
			if !strings.Contains(got.Text, "[REDACTED:") {
				t.Fatalf("expected a redaction marker, got %q", got.Text)
			}
			for _, keep := range tt.wantKept {
				if !strings.Contains(got.Text, keep) {
					t.Fatalf("expected non-secret context %q to be preserved, got %q", keep, got.Text)
				}
			}
			if len(tt.wantKinds) > 0 {
				assertEqual(t, got.Types(), tt.wantKinds)
			}
		})
	}
}

func TestMaskLeavesBenignTextUnchanged(t *testing.T) {
	benign := []string{
		"messages cannot be empty",
		"provider_timeout: upstream did not respond within 10s",
		"no route matched task_type=hard_code_debugging risk=medium",
		"invalid_request_error: model \"auto\" requires at least one message",
		"the token bucket is empty", // "token" not followed by = or :
	}
	for _, in := range benign {
		got := Mask(in)
		if got.Masked() {
			t.Fatalf("benign text was masked: in=%q out=%q kinds=%v", in, got.Text, got.Types())
		}
		if got.Text != in {
			t.Fatalf("benign text mutated: in=%q out=%q", in, got.Text)
		}
	}
}

func TestMaskEmptyString(t *testing.T) {
	got := Mask("")
	if got.Masked() || got.Text != "" || got.Count() != 0 {
		t.Fatalf("empty input should be a no-op, got %+v", got)
	}
}

func assertEqual(t *testing.T, got, want []string) {
	t.Helper()
	if len(got) != len(want) {
		t.Fatalf("length mismatch: got %v want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("mismatch at %d: got %v want %v", i, got, want)
		}
	}
}
