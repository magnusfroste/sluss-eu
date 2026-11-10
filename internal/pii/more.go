package pii

import (
	"regexp"
	"strings"
)

// More identifiers (ISSUE-168): other countries' identity numbers, and values
// that are personal only next to the word that says what they are.

var (
	// Finnish henkilötunnus: DDMMYY, a century sign, three digits, a check
	// character. 131052-308T.
	reHETU = regexp.MustCompile(`\b\d{6}[-+ABCDEFYXWVU]\d{3}[0-9A-FHJ-NPR-Y]\b`)
	// Norwegian fødselsnummer: eleven digits, optionally DDMMYY NNNNN.
	reFNR = regexp.MustCompile(`\b\d{6}[ ]?\d{5}\b`)
	// US social security number, written with hyphens.
	reSSN = regexp.MustCompile(`\b\d{3}-\d{2}-\d{4}\b`)
	// UK national insurance number: AB 12 34 56 C.
	reNINO = regexp.MustCompile(`\b[A-CEGHJ-PR-TW-Z][A-CEGHJ-NPR-TW-Z] ?\d{2} ?\d{2} ?\d{2} ?[A-D]\b`)
	// Danish CPR: DDMMYY-NNNN. It has had no check digit since 2007, so it
	// only counts next to the word.
	reCPR = regexp.MustCompile(`(?i)\bcpr(?:-?(?:nummer|nr|number))?\.?:?\s*(\d{6}-?\d{4})\b`)

	// A date next to words that make it a date of birth.
	reBirthdate = regexp.MustCompile(`(?i)(?:\bf[öo]dd(?: den)?|\bf[öo]delsedatum|\bdate of birth|\bdob|\bborn(?: on)?|\bbirth ?date|\bbirthday)\b[\s:]*((?:19|20)\d{2}[-/.]\d{1,2}[-/.]\d{1,2}|\d{1,2}[-/.]\d{1,2}[-/.](?:19|20)\d{2}|\d{1,2} [\p{L}]{3,9}\.? (?:19|20)\d{2})`)
	// A passport number next to the word.
	rePassport = regexp.MustCompile(`(?i)\b(?:pass(?:nummer|nr)|passport(?: (?:no|number|nr))?)\b\.?[\s:#]*([A-Z0-9]{2}[A-Z0-9 ]{4,9}\d)\b`)
	// A vehicle registration next to the word: ABC 123, ABC12D.
	reVehicle = regexp.MustCompile(`(?i)\b(?:reg(?:istrerings)?\.? ?(?:nummer|nr)|regnr|registration (?:no|number|plate)|licen[cs]e plate|number plate|registreringsskylt)\b\.?[\s:]*([A-Z]{3} ?\d{2}[0-9A-Z])\b`)
)

func scanMore(text string, emit func(t Type, start, end int)) {
	for _, loc := range reHETU.FindAllStringIndex(text, -1) {
		if validHETU(text[loc[0]:loc[1]]) {
			emit(TypeNationalID, loc[0], loc[1])
		}
	}
	for _, loc := range reFNR.FindAllStringIndex(text, -1) {
		if digitBounded(text, loc[0], loc[1]) && validFNR(text[loc[0]:loc[1]]) {
			emit(TypeNationalID, loc[0], loc[1])
		}
	}
	for _, loc := range reSSN.FindAllStringIndex(text, -1) {
		if digitBounded(text, loc[0], loc[1]) && validSSN(text[loc[0]:loc[1]]) {
			emit(TypeNationalID, loc[0], loc[1])
		}
	}
	for _, loc := range reNINO.FindAllStringIndex(text, -1) {
		// Capital letters only: "be 12 34 56 a" in prose is not one.
		emit(TypeNationalID, loc[0], loc[1])
	}
	for _, m := range reCPR.FindAllStringSubmatchIndex(text, -1) {
		if validDDMMYY(text[m[2] : m[2]+6]) {
			emit(TypeNationalID, m[2], m[3])
		}
	}
	for _, m := range reBirthdate.FindAllStringSubmatchIndex(text, -1) {
		emit(TypeBirthdate, m[2], m[3])
	}
	for _, m := range rePassport.FindAllStringSubmatchIndex(text, -1) {
		v := text[m[2]:m[3]]
		// At least four digits: "passport renewal" has none.
		if countDigits(v) >= 4 {
			emit(TypePassport, m[2], m[3])
		}
	}
	for _, m := range reVehicle.FindAllStringSubmatchIndex(text, -1) {
		emit(TypeVehicle, m[2], m[3])
	}
}

func countDigits(s string) int {
	n := 0
	for i := 0; i < len(s); i++ {
		if isDigit(s[i]) {
			n++
		}
	}
	return n
}

func validDDMMYY(s string) bool {
	if len(s) < 6 {
		return false
	}
	d := int(s[0]-'0')*10 + int(s[1]-'0')
	m := int(s[2]-'0')*10 + int(s[3]-'0')
	return d >= 1 && d <= 31 && m >= 1 && m <= 12
}

// validHETU checks the date and the check character: the nine digits as a
// number, modulo 31, indexes the table.
func validHETU(s string) bool {
	if len(s) != 11 || !validDDMMYY(s) {
		return false
	}
	const table = "0123456789ABCDEFHJKLMNPRSTUVWXY"
	n := 0
	for _, c := range s[:6] + s[7:10] {
		if c < '0' || c > '9' {
			return false
		}
		n = n*10 + int(c-'0')
	}
	return table[n%31] == s[10]
}

// validFNR checks a Norwegian fødselsnummer: a date (D-numbers add 40 to the
// day, H-numbers 40 and synthetic numbers 80 to the month) and two modulo-11
// check digits.
func validFNR(s string) bool {
	s = strings.ReplaceAll(s, " ", "")
	if len(s) != 11 {
		return false
	}
	var d [11]int
	for i := 0; i < 11; i++ {
		d[i] = int(s[i] - '0')
	}
	day, month := d[0]*10+d[1], d[2]*10+d[3]
	if day > 40 {
		day -= 40
	}
	// Synthetic test numbers (Skatteetaten's Tenor) add 80 to the month; a
	// test number is flagged like a real one, as with Swedish test numbers.
	if month > 80 {
		month -= 80
	} else if month > 40 {
		month -= 40
	}
	if day < 1 || day > 31 || month < 1 || month > 12 {
		return false
	}
	w1 := [9]int{3, 7, 6, 1, 8, 9, 4, 5, 2}
	w2 := [10]int{5, 4, 3, 2, 7, 6, 5, 4, 3, 2}
	sum := 0
	for i, w := range w1 {
		sum += w * d[i]
	}
	k1 := (11 - sum%11) % 11
	if k1 == 10 || k1 != d[9] {
		return false
	}
	sum = 0
	for i, w := range w2 {
		sum += w * d[i]
	}
	k2 := (11 - sum%11) % 11
	return k2 != 10 && k2 == d[10]
}

// validSSN applies the US rules: no area 000, 666 or 9xx, no group 00, no
// serial 0000.
func validSSN(s string) bool {
	area, group, serial := s[:3], s[4:6], s[7:]
	return area != "000" && area != "666" && area[0] != '9' && group != "00" && serial != "0000"
}
