package classifier_test

import (
	"testing"

	"github.com/magnusfroste/sluss/internal/classifier"
	"github.com/magnusfroste/sluss/internal/openai"
	"github.com/magnusfroste/sluss/internal/router"
)

// A prompt that CONTAINS a key or a protective-security marking is classified
// as such even when it never names it (ISSUE-157) — the first thing a CISO
// pastes into a gateway demo.
func TestSecretValuesAndProtectiveMarkings(t *testing.T) {
	cases := []struct {
		name, prompt string
		want         router.Sensitivity
	}{
		{"aws access key id", "why does the deploy fail with AKIAQ3EGTRWKX5VL7P2N", router.SensitivitySecretsPossible},
		{"aws secret in config", "aws_secret_access_key = wJalrXUtnFEMI/K7MDENG/bPxRfiCYEXAMPLEKEY fails", router.SensitivitySecretsPossible},
		{"openai-style key", "use sk-proj-abcdefghijklmnopqrstuv1234 in the client", router.SensitivitySecretsPossible},
		{"github token", "push with ghp_abcdefghijklmnopqrstuvwxyz0123456789", router.SensitivitySecretsPossible},
		{"pem private key", "-----BEGIN RSA PRIVATE KEY-----\nMIIEow\n-----END RSA PRIVATE KEY-----", router.SensitivitySecretsPossible},
		{"swedish protective security", "HEMLIG enligt säkerhetsskyddslagen: sammanfatta segmenteringsplanen", router.SensitivitySecurityClassified},
		{"swedish restricted", "Dokumentet är BEGRÄNSAT HEMLIG, gör en sammanfattning", router.SensitivitySecurityClassified},
		{"plain text stays plain", "Summarise the NIS2 incident reporting deadlines for an essential entity", router.SensitivityNone},
		{"a normal swedish sentence", "Hej, mötet flyttas till torsdag klockan tio", router.SensitivityNone},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			f := classifier.ExtractFromMessages([]openai.Message{{Role: "user", Content: c.prompt}}, classifier.RequestHints{})
			got := classifier.ClassifyRisk(f, "summarization", "")
			want := string(c.want)
			if c.want == router.SensitivityNone {
				want = ""
			}
			if got.Sensitivity != want && !(c.want == router.SensitivityNone && got.Sensitivity == "none") {
				t.Fatalf("sensitivity = %q, want %q", got.Sensitivity, c.want)
			}
		})
	}
}
