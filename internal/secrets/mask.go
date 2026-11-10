// Package secrets provides deterministic, rule-based masking of common secret
// material (API keys, bearer tokens, JWTs, database credentials) so that values
// never leak into client-facing error messages, structured logs, or the event
// log. Detection is pure and regex-based — it never calls an external service —
// and is only applied at output boundaries (error responses and logging), so it
// has no effect on the fast routing path.
package secrets

import (
	"regexp"
	"sort"
	"strings"
)

// Redaction is the placeholder substituted for a detected secret. The detected
// secret type is embedded so logs remain useful without exposing the value.
func redaction(kind string) string { return "[REDACTED:" + kind + "]" }

// Result is the outcome of masking a string.
type Result struct {
	// Text is the input with every detected secret replaced by a redaction.
	Text string
	// Kinds lists the secret types that were masked, in order of first
	// detection. Duplicate types appear once. Empty when nothing matched.
	kinds []string
}

// Masked reports whether at least one secret was detected and replaced.
func (r Result) Masked() bool { return len(r.kinds) > 0 }

// Count returns the number of distinct secret types masked.
func (r Result) Count() int { return len(r.kinds) }

// Types returns the distinct secret types masked, sorted for stable logging.
func (r Result) Types() []string {
	out := append([]string(nil), r.kinds...)
	sort.Strings(out)
	return out
}

// rule is one detection pattern. replace receives the full match and returns the
// masked replacement; it may preserve a non-secret prefix (e.g. a DB user or the
// "Bearer " keyword) while redacting only the sensitive portion.
type rule struct {
	kind    string
	re      *regexp.Regexp
	replace func(re *regexp.Regexp, match string) string
	// group is the capture group that holds the secret value when only part
	// of the match is the secret (a "Bearer " keyword, a DB user); 0 = all.
	group int
	// hints are lower-case substrings at least one of which must occur for
	// the pattern to possibly match — a cheap check before the regex, so
	// classifying a large prompt stays fast (ISSUE-157).
	hints []string
}

// full replaces the entire match with a redaction for the given kind.
func full(kind string) func(*regexp.Regexp, string) string {
	return func(*regexp.Regexp, string) string { return redaction(kind) }
}

// redactGroup replaces only capture group n within the match, leaving every
// other character of the match (a "Bearer " keyword, a DB scheme/user, a
// trailing "@", a closing quote) intact. This lets logs keep useful context
// while the secret value itself is redacted.
func redactGroup(kind string, n int) func(*regexp.Regexp, string) string {
	return func(re *regexp.Regexp, match string) string {
		loc := re.FindStringSubmatchIndex(match)
		if loc == nil || 2*n+1 >= len(loc) {
			return redaction(kind)
		}
		start, end := loc[2*n], loc[2*n+1]
		if start < 0 || end < 0 {
			return redaction(kind)
		}
		return match[:start] + redaction(kind) + match[end:]
	}
}

// valueClass is the character class for a captured secret value. It deliberately
// excludes "[" and "]" so a rule can never re-match a "[REDACTED:...]" token that
// an earlier rule already inserted.
const valueClass = `[^\s"',&)\[\]]+`

// rules are applied in order. More specific / higher-entropy patterns run first
// so they win over the generic key=value catch-all.
var rules = []rule{
	// JSON Web Tokens: three base64url segments separated by dots.
	{kind: "jwt", re: regexp.MustCompile(`eyJ[A-Za-z0-9_-]{6,}\.[A-Za-z0-9_-]{6,}\.[A-Za-z0-9_-]{6,}`), replace: full("jwt"), hints: []string{"eyj"}},
	// Anthropic / OpenAI-style keys (sk-, sk-ant-, sk-proj-...).
	{kind: "api_key", re: regexp.MustCompile(`sk-(?:ant-|proj-)?[A-Za-z0-9_-]{16,}`), replace: full("api_key"), hints: []string{"sk-"}},
	// More vendor formats (ISSUE-168): Stripe, GitLab, npm, Hugging Face,
	// SendGrid, Twilio, DigitalOcean, Shopify, Azure storage, and webhook
	// URLs whose path is the secret.
	{kind: "stripe_key", re: regexp.MustCompile(`(?:sk|rk|pk)_(?:live|test)_[A-Za-z0-9]{16,}`), replace: full("stripe_key"), hints: []string{"_live_", "_test_"}},
	{kind: "gitlab_token", re: regexp.MustCompile(`glpat-[A-Za-z0-9_-]{20,}`), replace: full("gitlab_token"), hints: []string{"glpat-"}},
	{kind: "npm_token", re: regexp.MustCompile(`npm_[A-Za-z0-9]{36}`), replace: full("npm_token"), hints: []string{"npm_"}},
	{kind: "hf_token", re: regexp.MustCompile(`hf_[A-Za-z0-9]{30,}`), replace: full("hf_token"), hints: []string{"hf_"}},
	{kind: "sendgrid_key", re: regexp.MustCompile(`SG\.[A-Za-z0-9_-]{16,}\.[A-Za-z0-9_-]{16,}`), replace: full("sendgrid_key"), hints: []string{"sg."}},
	{kind: "twilio_key", re: regexp.MustCompile(`\bSK[0-9a-f]{32}\b`), replace: full("twilio_key"), hints: []string{"sk"}},
	{kind: "do_token", re: regexp.MustCompile(`dop_v1_[a-f0-9]{64}`), replace: full("do_token"), hints: []string{"dop_v1_"}},
	{kind: "shopify_token", re: regexp.MustCompile(`shp(?:at|ca|pa|ss)_[a-fA-F0-9]{32}`), replace: full("shopify_token"), hints: []string{"shp"}},
	{kind: "azure_key", re: regexp.MustCompile(`(?i)(?:AccountKey|SharedAccessKey)=([A-Za-z0-9+/]{40,}={0,2})`), replace: redactGroup("azure_key", 1), group: 1, hints: []string{"accountkey=", "sharedaccesskey="}},
	{kind: "webhook_url", re: regexp.MustCompile(`https://hooks\.slack\.com/services/[A-Za-z0-9/]{20,}|https://discord(?:app)?\.com/api/webhooks/\d+/[A-Za-z0-9_-]{20,}|https://[a-z0-9-]+\.webhook\.office\.com/webhookb2/[A-Za-z0-9@/-]{20,}`), replace: full("webhook_url"), hints: []string{"hooks.slack.com", "/api/webhooks/", "webhook.office.com"}},
	// AWS access key IDs.
	{kind: "aws_key", re: regexp.MustCompile(`AKIA[0-9A-Z]{16}`), replace: full("aws_key"), hints: []string{"akia"}},
	// AWS secret access keys in their usual config form (ISSUE-157).
	{kind: "aws_secret", re: regexp.MustCompile(`(?i)aws_secret_access_key["']?\s*[=:]\s*["']?([A-Za-z0-9/+=]{30,})`), replace: redactGroup("aws_secret", 1), group: 1, hints: []string{"aws_secret_access_key"}},
	// Private keys in PEM form.
	{kind: "private_key", re: regexp.MustCompile(`-----BEGIN (?:[A-Z]+ )*PRIVATE KEY-----`), replace: full("private_key"), hints: []string{"private key-----"}},
	// Slack tokens.
	{kind: "slack_token", re: regexp.MustCompile(`xox[abprs]-[A-Za-z0-9-]{10,}`), replace: full("slack_token"), hints: []string{"xox"}},
	// Google API keys.
	{kind: "google_key", re: regexp.MustCompile(`AIza[0-9A-Za-z_-]{35}`), replace: full("google_key"), hints: []string{"aiza"}},
	// GitHub tokens (ghp_, gho_, ghu_, ghs_, ghr_).
	{kind: "github_token", re: regexp.MustCompile(`gh[pousr]_[A-Za-z0-9]{30,}`), replace: full("github_token"), hints: []string{"gh"}},
	// Bearer authorization values — keep the keyword, redact the token.
	{kind: "token", re: regexp.MustCompile(`(?i)bearer\s+([A-Za-z0-9._~+/=-]{8,})`), replace: redactGroup("token", 1), group: 1, hints: []string{"bearer"}},
	// Database / broker connection URIs with inline credentials — keep the
	// scheme and user, redact only the password.
	{kind: "db_password", re: regexp.MustCompile(`(?i)[a-z][a-z0-9+.-]*://[^:@/\s]+:(` + valueClass + `)@`), replace: redactGroup("db_password", 1), group: 1, hints: []string{"://"}},
	// Generic secret-named key/value pairs (incl. JSON "key":"value") — keep the
	// key, redact the value.
	{kind: "secret_kv", re: regexp.MustCompile(`(?i)(?:\b|_)(?:password|passwd|pwd|secret|api[_-]?key|apikey|access[_-]?token|auth[_-]?token|client[_-]?secret|token)\b["']?\s*[=:]\s*["']?(` + valueClass + `)`), replace: redactGroup("secret", 1), group: 1, hints: []string{"password", "passwd", "pwd", "secret", "key", "token"}},
}

func (r rule) mayMatch(lower string) bool {
	if len(r.hints) == 0 {
		return true
	}
	for _, h := range r.hints {
		if strings.Contains(lower, h) {
			return true
		}
	}
	return false
}

// Mask replaces every detected secret in s with a redaction placeholder and
// reports which secret types were found.
func Mask(s string) Result {
	res := Result{Text: s}
	if s == "" {
		return res
	}
	seen := make(map[string]struct{})
	for _, r := range rules {
		res.Text = r.re.ReplaceAllStringFunc(res.Text, func(match string) string {
			if _, ok := seen[r.kind]; !ok {
				seen[r.kind] = struct{}{}
				res.kinds = append(res.kinds, r.kind)
			}
			return r.replace(r.re, match)
		})
	}
	return res
}

// Detect reports which secret types s contains, without changing it. The
// classifier uses it so a prompt that CONTAINS a key is treated like one that
// talks about keys (ISSUE-157).
func Detect(s string) []string {
	if s == "" {
		return nil
	}
	var kinds []string
	lower := strings.ToLower(s)
	for _, r := range rules {
		if r.mayMatch(lower) && r.re.MatchString(s) {
			kinds = append(kinds, r.kind)
		}
	}
	return kinds
}

// MaskString is a convenience wrapper returning only the masked text.
func MaskString(s string) string { return Mask(s).Text }

// MaskWith replaces every detected secret value in s with what fn returns for
// it (ISSUE-151: a placeholder that is restored in the answer). Only the
// secret part of a match is replaced: a "Bearer " keyword or a DB user stays.
// It scans the whole string and returns the kinds it found.
func MaskWith(s string, fn func(kind, value string) string) (string, []string) {
	if s == "" {
		return s, nil
	}
	var kinds []string
	seen := map[string]bool{}
	lower := strings.ToLower(s)
	for _, r := range rules {
		if !r.mayMatch(lower) {
			continue
		}
		r := r
		s = r.re.ReplaceAllStringFunc(s, func(match string) string {
			start, end := 0, len(match)
			if r.group > 0 {
				loc := r.re.FindStringSubmatchIndex(match)
				if loc != nil && 2*r.group+1 < len(loc) && loc[2*r.group] >= 0 {
					start, end = loc[2*r.group], loc[2*r.group+1]
				}
			}
			if !seen[r.kind] {
				seen[r.kind] = true
				kinds = append(kinds, r.kind)
			}
			return match[:start] + fn(r.kind, match[start:end]) + match[end:]
		})
		lower = strings.ToLower(s)
	}
	return s, kinds
}
