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
	"unicode"
	"unicode/utf8"
)

// Type identifies a category of detected PII.
type Type string

const (
	TypePersonnummer Type = "personnummer" // Swedish personal identity number
	TypeEmail        Type = "email"
	TypePhone        Type = "phone"
	TypeIBAN         Type = "iban"
	TypeCard         Type = "card" // payment card number (Luhn-valid)
	// TypeAddress is a Swedish postal address: a street with a house number
	// and a five-digit postcode in the same text. Street names are a closed
	// set of Swedish street words, so an invoice total next to a year does
	// not count.
	TypeAddress Type = "address"
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
	// Phone numbers as people write them: +46 70 123 45 67, 070-123 45 67,
	// 08-123 456 78, 0701234567. Groups of digits separated by one space or
	// hyphen; the digit count is checked afterwards.
	// Alternatives, tried in order: international with a +, optionally with
	// the area code in parentheses (+1 (555) 123-4567, +46 70 123 45 67);
	// international with 00 (0046 70 123 45 67); an area code in parentheses
	// ((555) 123-4567, (08) 123 456); North-American 555-123-4567 or
	// 555.123.4567; national with a leading 0 (070-123 45 67, 020 7946 0958).
	// Every match is validated afterwards (digit count, not a date, not an
	// order or invoice number).
	rePhone = regexp.MustCompile(`\+\d{1,3}[-\s]?\(\d{1,4}\)(?:[-\s.]?\d{2,4}){1,3}` +
		`|\+\d{1,3}(?:[-\s]?\d{1,4}){2,5}` +
		`|00\d{1,3}(?:[-\s]?\d{1,4}){2,5}` +
		`|\(\d{2,4}\)\s?\d{2,4}(?:[-\s.]?\d{2,5}){1,2}` +
		`|\b\d{3}[-.]\d{3}[-.]\d{4}\b` +
		`|0\d{1,3}(?:[-\s]?\d{2,4}){1,4}`)
	// reDateLike is a date written with separators (08-11-2024, 2024-11-08):
	// the national pattern above would otherwise read it as a number.
	reDateLike = regexp.MustCompile(`^(?:\d{1,2}[-./]\d{1,2}[-./]\d{2,4}|\d{4}[-./]\d{1,2}[-./]\d{1,2})$`)
	// A Swedish street with a house number: Storgatan 12, Industrivägen 4B,
	// Kungsgränd 3, and the two-word form Lilla torget 47, Stora gatan 2.
	// The street word is what makes it an address.
	reStreet = regexp.MustCompile(`(?i)\b(?:\p{L}+(?:gatan|gata|vägen|väg|gränd|gränden|torget|torg|allén|allé|stigen|plan|backen|leden|promenaden|kajen|esplanaden)|\p{L}+\s(?:gatan|vägen|gränden|torget|allén|stigen|backen|kajen))\s+\d{1,4}\s?[A-Za-z]?\b`)
	// A Swedish postcode followed by a town: 111 22 Stockholm.
	rePostcode = regexp.MustCompile(`\b\d{3}\s?\d{2}\s+\p{Lu}\p{L}+`)
	// An IBAN written as one run or in groups of four, as it is printed on
	// invoices: SE45 5000 0000 0583 9825 7466. Validated with mod 97.
	reIBAN = regexp.MustCompile(`[A-Z]{2}\d{2}(?:[A-Za-z0-9]{10,30}|(?: [A-Z0-9]{4}){2,7}(?: [A-Z0-9]{1,3})?)`)
	reCard = regexp.MustCompile(`\d(?:[\d -]{11,24})\d`)
)

// Span is one detected personal identifier: its type and byte offsets in the
// scanned text. Used to mask values before a question is sent (ISSUE-151);
// Detect uses the same scan and keeps only the counts.
type Span struct {
	Type       Type
	Start, End int
}

// Find returns every identifier in text, sorted by position, with overlaps
// removed (the earlier, then the longer, match wins). Unlike Detect it scans
// the whole text: masking must not stop at a size limit.
func Find(text string) []Span {
	var spans []Span
	scan(text, func(t Type, start, end int) { spans = append(spans, Span{t, start, end}) })
	sort.Slice(spans, func(i, j int) bool {
		if spans[i].Start != spans[j].Start {
			return spans[i].Start < spans[j].Start
		}
		return spans[i].End > spans[j].End
	})
	out := spans[:0]
	last := -1
	for _, sp := range spans {
		if sp.Start < last {
			continue
		}
		out = append(out, sp)
		last = sp.End
	}
	return out
}

// scan runs every detector over text and reports each validated match.
func scan(text string, emit func(t Type, start, end int)) {
	for _, loc := range rePersonnummer.FindAllStringIndex(text, -1) {
		if digitBounded(text, loc[0], loc[1]) && validPersonnummer(text[loc[0]:loc[1]]) {
			emit(TypePersonnummer, loc[0], loc[1])
		}
	}
	for _, loc := range reEmail.FindAllStringIndex(text, -1) {
		emit(TypeEmail, loc[0], loc[1])
	}
	for _, loc := range rePhone.FindAllStringIndex(text, -1) {
		if digitBounded(text, loc[0], loc[1]) && validPhone(text[loc[0]:loc[1]]) && !numberContext(text, loc[0]) {
			emit(TypePhone, loc[0], loc[1])
		}
	}
	for _, loc := range reIBAN.FindAllStringIndex(text, -1) {
		// A grouped IBAN may have swallowed the next word ("… 7466 IDAG"):
		// drop trailing groups until the checksum holds or it is too short.
		start, end := loc[0], loc[1]
		for end-start >= 15 {
			if alnumBounded(text, start, end) && validIBAN(text[start:end]) {
				emit(TypeIBAN, start, end)
				break
			}
			sp := strings.LastIndex(text[start:end], " ")
			if sp < 0 {
				break
			}
			end = start + sp
		}
	}
	// An address is a street with a house number AND a postcode in the same
	// text; both parts are reported, so masking hides both.
	if streets := reStreet.FindAllStringIndex(text, -1); len(streets) > 0 {
		if posts := rePostcode.FindAllStringIndex(text, -1); len(posts) > 0 {
			for _, loc := range streets {
				emit(TypeAddress, loc[0], loc[1])
			}
			for _, loc := range posts {
				emit(TypeAddress, loc[0], loc[1])
			}
		}
	}
	for _, loc := range reCard.FindAllStringIndex(text, -1) {
		if digitBounded(text, loc[0], loc[1]) && validCard(text[loc[0]:loc[1]]) {
			emit(TypeCard, loc[0], loc[1])
		}
	}
}

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
	scan(text, func(t Type, _, _ int) { counts[t]++ })
	// An address counts once, however many street and postcode parts it has.
	if counts[TypeAddress] > 0 {
		counts[TypeAddress] = 1
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
	if reDateLike.MatchString(strings.TrimSpace(candidate)) {
		return false
	}
	switch {
	case strings.HasPrefix(candidate, "+"):
		return len(digits) >= 8 && len(digits) <= 15
	case strings.HasPrefix(candidate, "00"):
		// 00 + country code + number: E.164 written without the plus.
		return len(digits) >= 10 && len(digits) <= 17
	case strings.HasPrefix(candidate, "("):
		// (555) 123-4567, (08) 123 456 78, (020) 7946 0958.
		return len(digits) >= 8 && len(digits) <= 11
	case !strings.HasPrefix(candidate, "0"):
		// North-American 555-123-4567: exactly ten digits.
		return len(digits) == 10
	}
	// A Swedish number is 9 or 10 digits with the leading 0; 8 allows short
	// city numbers written without the area code's full length.
	return len(digits) >= 8 && len(digits) <= 11
}

// numberWords are labels that make a run of digits an order, invoice, case or
// account number rather than a phone number. Account and bank numbers are
// caught by the financial rules on their own.
var numberWords = []string{
	"order", "ordernr", "ordernummer", "beställning", "invoice", "faktura", "fakturanr", "fakturanummer",
	"ref", "referens", "reference", "ärende", "ärendenr", "case", "ticket", "kundnr", "kundnummer",
	"customer no", "org.nr", "orgnr", "organisationsnummer", "art.nr", "artikelnr", "artikel",
	"konto", "kontonr", "account", "bankgiro", "plusgiro", "serial", "serienummer", "id",
}

// numberContext reports whether the words just before start label the digits
// as some other kind of number.
func numberContext(text string, start int) bool {
	from := start - 28
	if from < 0 {
		from = 0
	}
	before := strings.ToLower(text[from:start])
	before = strings.TrimRight(before, " :#.-\t")
	for _, w := range numberWords {
		if !strings.HasSuffix(before, w) {
			continue
		}
		// A whole word only: "id" must not match the end of "David".
		i := len(before) - len(w)
		if i == 0 {
			return true
		}
		r, _ := utf8.DecodeLastRuneInString(before[:i])
		if !unicode.IsLetter(r) && !unicode.IsDigit(r) {
			return true
		}
	}
	return false
}

// validIBAN checks length and the ISO 13616 mod-97 checksum.
func validIBAN(candidate string) bool {
	candidate = strings.ReplaceAll(candidate, " ", "")
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
