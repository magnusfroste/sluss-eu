// Package pii implements rule-based detection of common personally identifiable
// information in prompt text (ISSUE-077). It is pure and deterministic — regexp
// plus checksum validation, no LLM, no network — so it fits inside the feature
// extractor's fast-path latency budget.
//
// Detection is an escalation signal, never a verdict: findings raise the
// request's sensitivity so policy can block, force residency, or escalate the
// tier. Hard blocking is always the policy's decision. Findings carry only the
// TYPE and COUNT of matches — never the matched values — so nothing sensitive
// leaks into logs, audit entries, or the event queue.
package pii

import (
	"regexp"
	"sort"
	"strings"
)

// Type identifies a category of detected PII.
type Type string

const (
	TypePersonnummer Type = "personnummer" // Swedish personal identity number
	TypeEmail        Type = "email"
	TypePhone        Type = "phone"
	TypeIBAN         Type = "iban"
	TypeCard         Type = "card" // payment card number (Luhn-valid)
)

// Finding is one detected PII category with its occurrence count. It never
// contains the matched text.
type Finding struct {
	Type  Type `json:"type"`
	Count int  `json:"count"`
}

// maxScanBytes bounds the per-text scan so a pathological prompt cannot blow
// the fast-path budget. PII in the first 64 KiB is more than enough signal.
const maxScanBytes = 64 * 1024

var (
	// Candidate patterns are intentionally loose; each match is then validated
	// (checksums, date rules, digit boundaries) before it counts.
	rePersonnummer = regexp.MustCompile(`\d{6,8}[-+]?\d{4}`)
	reEmail        = regexp.MustCompile(`[A-Za-z0-9._%+-]+@[A-Za-z0-9.-]+\.[A-Za-z]{2,}`)
	rePhone        = regexp.MustCompile(`\+\d{8,15}|0\d{1,3}[-\s]?\d{5,8}`)
	reIBAN         = regexp.MustCompile(`[A-Z]{2}\d{2}[A-Za-z0-9]{10,30}`)
	reCard         = regexp.MustCompile(`\d(?:[\d -]{11,24})\d`)
)

// Detect scans text and returns the detected PII categories with counts,
// sorted by type for determinism. It never returns matched values.
func Detect(text string) []Finding {
	if text == "" {
		return nil
	}
	if len(text) > maxScanBytes {
		text = text[:maxScanBytes]
	}

	counts := map[Type]int{}

	for _, loc := range rePersonnummer.FindAllStringIndex(text, -1) {
		if !digitBounded(text, loc[0], loc[1]) {
			continue
		}
		if validPersonnummer(text[loc[0]:loc[1]]) {
			counts[TypePersonnummer]++
		}
	}
	for range reEmail.FindAllString(text, -1) {
		counts[TypeEmail]++
	}
	for _, loc := range rePhone.FindAllStringIndex(text, -1) {
		if !digitBounded(text, loc[0], loc[1]) {
			continue
		}
		if validPhone(text[loc[0]:loc[1]]) {
			counts[TypePhone]++
		}
	}
	for _, loc := range reIBAN.FindAllStringIndex(text, -1) {
		if !alnumBounded(text, loc[0], loc[1]) {
			continue
		}
		if validIBAN(text[loc[0]:loc[1]]) {
			counts[TypeIBAN]++
		}
	}
	for _, loc := range reCard.FindAllStringIndex(text, -1) {
		if !digitBounded(text, loc[0], loc[1]) {
			continue
		}
		if validCard(text[loc[0]:loc[1]]) {
			counts[TypeCard]++
		}
	}

	if len(counts) == 0 {
		return nil
	}
	findings := make([]Finding, 0, len(counts))
	for t, n := range counts {
		findings = append(findings, Finding{Type: t, Count: n})
	}
	sort.Slice(findings, func(i, j int) bool { return findings[i].Type < findings[j].Type })
	return findings
}

// Merge combines two finding sets, summing counts per type. The result is
// sorted by type. Either argument may be nil.
func Merge(a, b []Finding) []Finding {
	if len(b) == 0 {
		return a
	}
	if len(a) == 0 {
		return b
	}
	counts := make(map[Type]int, len(a)+len(b))
	for _, f := range a {
		counts[f.Type] += f.Count
	}
	for _, f := range b {
		counts[f.Type] += f.Count
	}
	out := make([]Finding, 0, len(counts))
	for t, n := range counts {
		out = append(out, Finding{Type: t, Count: n})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Type < out[j].Type })
	return out
}

// Types returns just the type names of a finding set (sorted, as Detect sorts).
func Types(findings []Finding) []string {
	if len(findings) == 0 {
		return nil
	}
	out := make([]string, len(findings))
	for i, f := range findings {
		out[i] = string(f.Type)
	}
	return out
}

// digitBounded reports whether the match at [start,end) is not embedded in a
// longer digit run (e.g. a personnummer-shaped slice of a longer number).
func digitBounded(s string, start, end int) bool {
	if start > 0 && isDigit(s[start-1]) {
		return false
	}
	if end < len(s) && isDigit(s[end]) {
		return false
	}
	return true
}

// alnumBounded is digitBounded for alphanumeric contexts (IBAN letters).
func alnumBounded(s string, start, end int) bool {
	if start > 0 && isAlnum(s[start-1]) {
		return false
	}
	if end < len(s) && isAlnum(s[end]) {
		return false
	}
	return true
}

func isDigit(b byte) bool { return b >= '0' && b <= '9' }

func isAlnum(b byte) bool {
	return isDigit(b) || (b >= 'a' && b <= 'z') || (b >= 'A' && b <= 'Z')
}

// validPersonnummer validates a candidate Swedish personal identity number:
// ÅÅMMDD[-+]XXXX or ÅÅÅÅMMDD[-]XXXX, with date rules (samordningsnummer adds 60
// to the day) and the Luhn checksum over the 10-digit short form.
func validPersonnummer(candidate string) bool {
	digits := onlyDigits(candidate)
	switch len(digits) {
	case 10:
		// ÅÅMMDDXXXX — nothing more to strip.
	case 12:
		// ÅÅÅÅMMDDXXXX — century must be plausible; validate on the short form.
		century := digits[:2]
		if century != "18" && century != "19" && century != "20" {
			return false
		}
		digits = digits[2:]
	default:
		return false
	}

	month := int(digits[2]-'0')*10 + int(digits[3]-'0')
	day := int(digits[4]-'0')*10 + int(digits[5]-'0')
	if day > 60 {
		day -= 60 // samordningsnummer
	}
	if month < 1 || month > 12 || day < 1 || day > 31 {
		return false
	}
	return luhnValid(digits)
}

// validPhone accepts E.164 (+ and 8–15 digits) or Swedish national format
// (leading 0, 8–11 digits total).
func validPhone(candidate string) bool {
	digits := onlyDigits(candidate)
	if strings.HasPrefix(candidate, "+") {
		return len(digits) >= 8 && len(digits) <= 15
	}
	return len(digits) >= 8 && len(digits) <= 11
}

// validIBAN checks length and the ISO 13616 mod-97 checksum.
func validIBAN(candidate string) bool {
	if len(candidate) < 15 || len(candidate) > 34 {
		return false
	}
	// Move the first four characters to the end, then map letters to 10..35 and
	// compute mod 97 incrementally.
	rearranged := candidate[4:] + candidate[:4]
	rem := 0
	for i := 0; i < len(rearranged); i++ {
		c := rearranged[i]
		switch {
		case isDigit(c):
			rem = (rem*10 + int(c-'0')) % 97
		case c >= 'A' && c <= 'Z':
			v := int(c-'A') + 10
			rem = (rem*100 + v) % 97
		default:
			return false
		}
	}
	return rem == 1
}

// validCard strips separators and requires 13–19 digits passing Luhn.
func validCard(candidate string) bool {
	digits := onlyDigits(candidate)
	if len(digits) < 13 || len(digits) > 19 {
		return false
	}
	return luhnValid(digits)
}

// luhnValid computes the Luhn checksum over a digit string (rightmost digit is
// the check digit).
func luhnValid(digits string) bool {
	sum := 0
	double := false
	for i := len(digits) - 1; i >= 0; i-- {
		d := int(digits[i] - '0')
		if double {
			d *= 2
			if d > 9 {
				d -= 9
			}
		}
		sum += d
		double = !double
	}
	return sum%10 == 0
}

func onlyDigits(s string) string {
	var b strings.Builder
	b.Grow(len(s))
	for i := 0; i < len(s); i++ {
		if isDigit(s[i]) {
			b.WriteByte(s[i])
		}
	}
	return b.String()
}
