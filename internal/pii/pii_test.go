package pii

import (
	"strings"
	"testing"
)

// count returns the count for a type in a finding set (0 if absent).
func count(findings []Finding, t Type) int {
	for _, f := range findings {
		if f.Type == t {
			return f.Count
		}
	}
	return 0
}

func TestPersonnummer(t *testing.T) {
	cases := []struct {
		name string
		text string
		want int
	}{
		{"valid short with dash", "kund 811218-9876 ringde", 1},
		{"valid short no dash", "pnr 8112189876 finns", 1},
		{"valid 12-digit", "personnummer 198112189876 registrerat", 1},
		{"valid 12-digit with dash", "19811218-9876", 1},
		{"valid samordningsnummer (+60 day)", "sam 811278-9873 ok", 1},
		{"invalid luhn", "811218-9877", 0},
		{"invalid month passes luhn", "811318-9875", 0},
		{"invalid century", "218112189876", 0},
		{"embedded in longer number", "9999811218-98761111", 0},
		{"two valid", "811218-9876 och 19811218-9876", 2},
		{"plain year no pnr", "året 1985 hände det", 0},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := count(Detect(tc.text), TypePersonnummer); got != tc.want {
				t.Fatalf("personnummer count = %d, want %d (text %q)", got, tc.want, tc.text)
			}
		})
	}
}

func TestEmail(t *testing.T) {
	if got := count(Detect("kontakta anna.svensson@example.se idag"), TypeEmail); got != 1 {
		t.Fatalf("email count = %d, want 1", got)
	}
	if got := count(Detect("ingen adress här, bara @handle och punkt."), TypeEmail); got != 0 {
		t.Fatalf("email count = %d, want 0", got)
	}
}

func TestPhone(t *testing.T) {
	cases := []struct {
		name string
		text string
		want int
	}{
		{"e164", "ring +46701234567 nu", 1},
		{"swedish mobile with dash", "mobil 070-1234567", 1},
		{"swedish landline", "vx 08-52012345", 1},
		{"too short", "kod 0123", 0},
		{"bare digits no prefix", "id 12345678", 0},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := count(Detect(tc.text), TypePhone); got != tc.want {
				t.Fatalf("phone count = %d, want %d (text %q)", got, tc.want, tc.text)
			}
		})
	}
}

func TestIBAN(t *testing.T) {
	cases := []struct {
		name string
		text string
		want int
	}{
		{"valid gb", "betala till GB82WEST12345698765432 tack", 1},
		{"valid se", "konto SE4550000000058398257466", 1},
		{"bad checksum", "GB82WEST12345698765431", 0},
		{"embedded", "XGB82WEST12345698765432", 0},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := count(Detect(tc.text), TypeIBAN); got != tc.want {
				t.Fatalf("iban count = %d, want %d (text %q)", got, tc.want, tc.text)
			}
		})
	}
}

func TestCard(t *testing.T) {
	cases := []struct {
		name string
		text string
		want int
	}{
		{"visa test number", "kort 4111 1111 1111 1111 gick igenom", 1},
		{"visa with dashes", "4111-1111-1111-1111", 1},
		{"bad luhn", "4111 1111 1111 1112", 0},
		{"too few digits", "1234 5678 9012", 0},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := count(Detect(tc.text), TypeCard); got != tc.want {
				t.Fatalf("card count = %d, want %d (text %q)", got, tc.want, tc.text)
			}
		})
	}
}

// A personnummer must not double-count as phone or card.
func TestNoCrossCategoryDoubleCount(t *testing.T) {
	findings := Detect("kund 811218-9876")
	if got := count(findings, TypePersonnummer); got != 1 {
		t.Fatalf("personnummer = %d, want 1", got)
	}
	if got := count(findings, TypeCard); got != 0 {
		t.Fatalf("card = %d, want 0 (10 digits is below card range)", got)
	}
}

func TestFindingsNeverContainValues(t *testing.T) {
	findings := Detect("811218-9876 och anna@example.se")
	for _, f := range findings {
		if strings.Contains(string(f.Type), "9876") || strings.Contains(string(f.Type), "anna") {
			t.Fatalf("finding leaked matched value: %+v", f)
		}
	}
	types := Types(findings)
	if len(types) != 2 || types[0] != "email" || types[1] != "personnummer" {
		t.Fatalf("types = %v, want [email personnummer] (sorted)", types)
	}
}

func TestEmptyAndCleanText(t *testing.T) {
	if Detect("") != nil {
		t.Fatal("empty text should yield nil")
	}
	if got := Detect("helt vanlig text utan känsliga uppgifter, version 2.1.3"); got != nil {
		t.Fatalf("clean text should yield nil, got %v", got)
	}
}

func TestScanCapDoesNotPanic(t *testing.T) {
	big := strings.Repeat("a", maxScanBytes) + " 811218-9876"
	// PII beyond the cap is not seen — the cap bounds cost, and that trade-off
	// is intentional.
	if got := count(Detect(big), TypePersonnummer); got != 0 {
		t.Fatalf("pii beyond scan cap should not be counted, got %d", got)
	}
}

func BenchmarkDetect10KB(b *testing.B) {
	text := strings.Repeat("vanlig prosa med siffror 12345 och ord ", 250) // ~10KB
	text += " 811218-9876 anna@example.se +46701234567"
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		Detect(text)
	}
}
