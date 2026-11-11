package classifier_test

import (
	"strings"
	"testing"

	"github.com/magnusfroste/sluss/internal/openai"
	"github.com/magnusfroste/sluss/internal/pii"
	"github.com/magnusfroste/sluss/internal/router"
)

func classesOf(text string) map[string]bool {
	job := router.NewJobDescriptor(router.JobDescriptorInput{Request: &openai.ChatRequest{
		Model:    "auto",
		Messages: []openai.Message{{Role: "user", Content: text}},
	}})
	out := map[string]bool{}
	for _, s := range job.AllSensitivities() {
		out[s] = true
	}
	return out
}

// ISSUE-168: a broader classifier. The examples are not in the benchmark set.
func TestBroaderClassifier(t *testing.T) {
	// Built in pieces so the repository's secret scan does not see a key.
	stripe := "sk_" + "live_" + strings.Repeat("a1B2c3D4", 3)
	gitlab := "glpat-" + strings.Repeat("x9Y8z7W6", 3)
	cases := []struct {
		name, text string
		want       string // "" means no class at all
	}{
		// Names.
		{"customer name", "Skriv ett svar till vår kund Birgitta Åkesson om reklamationen.", "pii"},
		{"name with particle", "Book a call with Carl von Essen about the renewal.", "pii"},
		{"double given name", "Anna Maria Lindholm has asked for her data; draft the reply.", "pii"},
		{"hyphenated family name", "Remind Yusuf Al-Masri about the unpaid invoice.", "pii"},
		{"given name alone", "Ask Henrik to book the room for Thursday.", ""},
		{"a station is not a person", "Meet at Victoria Station at nine and walk to the office.", ""},
		{"heading in title case", "Agenda: Meeting With Sara About Budget And Planning", ""},
		{"company, not a person", "Draft a thank-you note to Nordisk Frakt AB for the fast delivery.", ""},

		// Other identity numbers. The Finnish number uses an individual
		// number reserved for tests; the Norwegian one is synthetic (month
		// plus 80); the US one was retired after appearing on sample cards.
		{"finnish id", "Asiakas 010190-900P haluaa perua tilauksen; write the reply in English.", "pii"},
		{"norwegian id", "Kunden med fødselsnummer 01815010260 vill byta adress.", "pii"},
		{"us ssn", "Her SSN is 078-05-1120; complete the onboarding form.", "pii"},
		{"uk nino", "National insurance number AB 12 34 56 C is missing from the payslip; write to payroll.", "pii"},
		{"danish cpr", "Kundens CPR-nummer 010190-1234 skal med i svaret.", "pii"},
		{"date of birth", "Den sökande, född 1987-04-12, vill ha besked före fredag.", "pii"},
		{"a date that is not a birth", "The office opened 1987-04-12; write an anniversary post.", ""},
		{"passport", "Passnummer AA1234567 behövs för visumansökan; skriv följebrevet.", "pii"},
		{"vehicle", "Tjänstebilen med regnr KLM 47B ska säljas; skriv annonsen.", "pii"},

		// Keys.
		{"stripe key", "Why does the checkout fail with " + stripe + " in the header?", "secrets_possible"},
		{"gitlab token", "CI says unauthorized; I use " + gitlab + " as the token.", "secrets_possible"},
		{"swedish word for password", "Lösenordet till lagerdatorn står på lappen under tangentbordet, är det okej?", "secrets_possible"},

		// Terms the people who handle the data use.
		{"health: condition", "Our colleague was diagnosed with epilepsy; how should the night shift be planned?", "health"},
		{"health: papers", "Sammanfatta läkarintyget och arbetsförmågebedömningen inför mötet med Försäkringskassan.", "health"},
		{"financial: deal", "Draft the board memo on the term sheet and the share purchase agreement.", "financial"},
		{"financial: credit", "Kunden har en betalningsanmärkning och ett ärende hos Kronofogden; hur svarar vi på låneansökan?", "financial"},
		{"legal: court", "Tingsrätten har skickat en stämning där vi är svarande; sammanfatta vad som krävs av oss.", "legal"},
		{"legal: documents", "Review the shareholder agreement and the power of attorney before signing.", "legal"},
		{"special category", "Listan över fackmedlemmar på lagret ska skickas till lönekontoret; skriv följebrevet.", "pii"},
		{"hr record", "The personnel file shows a written warning last year; draft the follow-up meeting notes.", "pii"},
		{"critical infrastructure", "Uppdatera planen för skyddsobjektet inför totalförsvarsövningen.", "security_classified"},

		// A general question uses the words without holding the data.
		{"general: finance", "What is a term sheet?", ""},
		{"general: health", "Vad är epilepsi?", ""},
		{"general: legal", "Hur fungerar en skiljedom?", ""},
		{"general: secrets", "How should passwords be stored in a web application?", ""},
		{"general, but own material", "What does this term sheet mean for us?\nValuation 40 MSEK, 20 % option pool.", "financial"},
		{"general, but about a person", "What is the diagnosis for Henrik?", "health"},
		{"a marking is never general", "What does NATO RESTRICTED mean on the cover page?", "security_classified"},
		{"history is not health", "Write three lines on how the Great Depression began.", ""},
	}
	for _, c := range cases {
		got := classesOf(c.text)
		if c.want == "" && len(got) > 0 {
			t.Errorf("%s: flagged %v, want nothing", c.name, got)
		}
		if c.want != "" && !got[c.want] {
			t.Errorf("%s: got %v, want %s", c.name, got, c.want)
		}
	}
}

func TestNameDetectionCanBeTurnedOff(t *testing.T) {
	const q = "Skriv ett svar till vår kund Birgitta Åkesson om reklamationen."
	pii.SetNameDetection(false)
	off := classesOf(q)
	pii.SetNameDetection(true)
	if on := classesOf(q); !on["pii"] || off["pii"] {
		t.Fatalf("on=%v off=%v", on, off)
	}
}

// ISSUE-169: how a critical facility is protected, with no marking and none
// of the field's formal words. Examples are not in the benchmark set.
func TestCriticalFacilityProtection(t *testing.T) {
	cases := []struct {
		name, text string
		want       bool
	}{
		{"substation perimeter", "Granska skalskyddet runt ställverket i Norrby och föreslå var kamerorna ska sitta.", true},
		{"pumping station guard rota", "Here is the guard rota for the pumping station; find the hours with nobody on site.", true},
		{"control room redundancy", "Kontrollrummet saknar reservkraft efter klockan 22; skriv en åtgärdslista.", true},
		{"dispatch centre weak points", "List the weak points of the dispatch centre's access control before the audit.", true},
		{"fuel depot threat", "Sammanfatta hotbilden mot bränsledepån inför ledningsgruppens möte.", true},
		{"a visit is not protection", "Boka ett studiebesök på vattenverket för klass 5B i maj.", false},
		{"a facility in the news", "Write a short news item: the new power plant opens in June.", false},
		{"protection without a facility", "List the weaknesses in our onboarding process for new hires.", false},
		{"an office door", "Vi behöver nya passerkort till kontoret; skriv en beställning.", false},
	}
	for _, c := range cases {
		if got := classesOf(c.text)["security_classified"]; got != c.want {
			t.Errorf("%s: security_classified=%v, want %v", c.name, got, c.want)
		}
	}
	// The Finnish number's digits are not also a phone number.
	job := router.NewJobDescriptor(router.JobDescriptorInput{Request: &openai.ChatRequest{Model: "auto",
		Messages: []openai.Message{{Role: "user", Content: "Asiakas 010190-900P haluaa perua tilauksen."}}}})
	if strings.Join(job.PIITypes, ",") != "national_id" {
		t.Errorf("pii types %v, want national_id only", job.PIITypes)
	}
}

// ISSUE-173: a person from history is not personal data; a living public
// figure and a private person still are.
func TestHistoricalFigures(t *testing.T) {
	cases := []struct {
		name, text string
		pii        bool
	}{
		{"an author", "Sammanfatta Vilhelm Mobergs utvandrarserie för bokcirkeln.", false},
		{"a scientist", "Explain what Charles Darwin meant by natural selection.", false},
		{"possessive", "What was Florence Nightingale's contribution to statistics?", false},
		{"with a particle", "Skriv tre meningar om Carl von Linné till skolans växtvandring.", false},
		{"a private person", "Sammanfatta ärendet för Vilhelm Moström som väntar på återbetalning.", true},
		{"a living public figure", "Skriv ett brev till Carl Bildt och bjud in honom som talare.", true},
		{"history beside a private person", "Compare Charles Darwin's handwriting with our customer Henrik Wallin's signature.", true},
		{"history beside an identifier", "Charles Darwin is the password hint; the account belongs to anna.ek@example.org.", true},
	}
	for _, c := range cases {
		if got := classesOf(c.text)["pii"]; got != c.pii {
			t.Errorf("%s: pii=%v, want %v", c.name, got, c.pii)
		}
	}
}

// A street, a square or a school named after a person is a place, not a
// person.
func TestPlacesNamedAfterPeople(t *testing.T) {
	for _, c := range []struct {
		text string
		pii  bool
	}{
		{"Give directions from Karl Johans gate to the office.", false},
		{"Mötet hålls på Anna Whitlocks gymnasium klockan nio.", false},
		{"Hur tar jag mig från Gustav Adolfs torg till centralen?", false},
		{"Park at Martin Olsson Road and walk the last bit.", false},
		{"Skicka kallelsen till Karl Johansson om mötet på torget.", true},
		{"Ring Anna Whitlock om gymnasiets schema.", true},
	} {
		if got := classesOf(c.text)["pii"]; got != c.pii {
			t.Errorf("%q: pii=%v, want %v", c.text, got, c.pii)
		}
	}
}
