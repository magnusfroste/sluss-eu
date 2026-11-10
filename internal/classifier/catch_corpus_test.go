package classifier_test

// The "what Sluss catches" corpus (ISSUE-162). The manual page
// manual/what-sluss-catches.md quotes the numbers this test produces, so the
// page can only say what the classifier actually does. Every case states
// whether the deterministic rules catch it ("caught") or, by design, do not
// ("miss": meaning without an identifier, marker or term). When a miss gets
// caught, or a catch gets lost, this test fails and the page is updated with
// the new numbers: honesty is enforced, not promised.
//
// Run with -v to print the table the manual page carries:
//
//	go test ./internal/classifier -run TestCatchCorpus -v

import (
	"fmt"
	"os"
	"sort"
	"strings"
	"testing"

	"github.com/magnusfroste/sluss/internal/classifier"
	"github.com/magnusfroste/sluss/internal/openai"
	"github.com/magnusfroste/sluss/internal/router"
)

type catchCase struct {
	class  router.Sensitivity // the class a careful reader would give it
	prompt string
	caught bool // what the rules do today: true = classified as class, false = a documented miss
	why    string
}

// A Slack-shaped token assembled at run time, so no scanner mistakes the test
// file for a leak.
var fakeSlackToken = "xoxb-" + strings.Repeat("1", 12) + "-" + strings.Repeat("2", 12) + "-" + "abcdefghijklmnopqrstuvwx"

var catchCorpus = []catchCase{
	// ---- Personal data (pii) -------------------------------------------------
	{router.SensitivityPII, "Summarise the case for customer 811218-2392.", true, "Swedish personal identity number, checksum-validated"},
	{router.SensitivityPII, "Kunden med personnummer 19811218-2392 vill avsluta sitt konto.", true, "12-digit personnummer"},
	{router.SensitivityPII, "Sammanfatta ärendet för 811218+2392 (pensionär).", true, "the + form for people over 100"},
	{router.SensitivityPII, "Mail anna.svensson@example.com and ask about the invoice.", true, "an e-mail address"},
	{router.SensitivityPII, "Ring Anna på 070-123 45 67 och boka ett möte.", true, "a Swedish mobile number"},
	{router.SensitivityPII, "Call our customer Jane on (555) 123-4567 about the refund.", true, "a phone number in parentheses (North-American)"},
	{router.SensitivityPII, "Kunden nås på +44 20 7946 0958 eller 0046 70 123 45 67.", true, "international numbers with + and with 00"},
	{router.SensitivityPII, "Anna Svensson, Storgatan 12, 111 22 Stockholm, har klagat på leveransen.", true, "a postal address with a postcode"},
	{router.SensitivityPII, "Skriv ett brev till vår kund Anna Svensson om den försenade leveransen.", true, "a full name (ISSUE-168)"},
	{router.SensitivityPII, "Here is the employee list: Anna (sales, 34), Erik (finance, 51), Maria (HR, 29). Who should get the award?", false, "names and ages, no identifier"},
	{router.SensitivityPII, "Rewrite this performance review: he is often late and argues with the team lead.", true, "a performance review is a record about a person (ISSUE-168)"},

	// ---- Keys and secrets (secrets_possible) ----------------------------------
	{router.SensitivitySecretsPossible, "Why does this fail? OPENAI_API_KEY=sk-proj-abcdefghijklmnopqrstuvwxyz0123456789ABCDEFGHIJ", true, "an OpenAI-style key"},
	{router.SensitivitySecretsPossible, "aws_secret_access_key = wJalrXUtnFEMI/K7MDENG/bPxRfiCYEXAMPLEKEY", true, "an AWS secret"},
	{router.SensitivitySecretsPossible, "token: ghp_ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghij12", true, "a GitHub token"},
	{router.SensitivitySecretsPossible, "-----BEGIN RSA PRIVATE KEY-----\nMIIEpAIBAAKCAQEA...\n-----END RSA PRIVATE KEY-----", true, "a private key"},
	{router.SensitivitySecretsPossible, "slack bot " + fakeSlackToken, true, "a Slack token"},
	{router.SensitivitySecretsPossible, "The deploy script has the api key hardcoded — rotate the secret.", true, "the words api key and secret"},
	{router.SensitivitySecretsPossible, "Our database password is hunter2, please update the config.", true, "the word password"},
	{router.SensitivitySecretsPossible, "Use this to log in: Tr0ub4dor&3 on the admin account.", false, "a password with no label around it"},

	// ---- Security-classified material ---------------------------------------
	{router.SensitivitySecurityClassified, "HEMLIG enligt säkerhetsskyddslagen. Sammanfatta lägesbilden om skyddsobjektet.", true, "a Swedish stamp and the law"},
	{router.SensitivitySecurityClassified, "Begränsat hemlig — skriv en sammanfattning av rapporten.", true, "a protection level"},
	{router.SensitivitySecurityClassified, "Translate this classified document, marked NATO restricted.", true, "a NATO marking"},
	{router.SensitivitySecurityClassified, "Summarise the attached: RESTREINT UE/EU RESTRICTED.", true, "an EU marking"},
	{router.SensitivitySecurityClassified, "Här är planen för hur vi skyddar kraftnätets styrsystem mot sabotage; föreslå förbättringar.", true, "how a critical facility is protected, no marking (ISSUE-169)"},

	// ---- Health ---------------------------------------------------------------
	{router.SensitivityHealth, "Sammanfatta patientjournalen: diagnos F32.1, sjukskriven 50 %.", true, "patient record, diagnosis"},
	{router.SensitivityHealth, "The patient was prescribed sertraline 50 mg for depression.", true, "patient, prescribed, a diagnosis"},
	{router.SensitivityHealth, "Min kollega Anna är sjukskriven i tre månader; hur fördelar vi hennes uppgifter?", true, "sjukskriven"},
	{router.SensitivityHealth, "Erik has been off for two months with burnout. Draft a kind message to the team.", true, "burnout"},
	{router.SensitivityHealth, "Write a note that Maria will be away for chemo on Tuesdays.", true, "chemo"},
	{router.SensitivityHealth, "Erik is on leave until March because of what happened in the spring; who covers his accounts?", false, "an absence with no health word"},

	// ---- Financial ------------------------------------------------------------
	{router.SensitivityFinancial, "Here is the balance sheet and P&L for Q3 before the earnings release; summarise.", true, "balance sheet, P&L, earnings"},
	{router.SensitivityFinancial, "Betala fakturan till IBAN SE45 5000 0000 0583 9825 7466.", false, "an IBAN is flagged as personal data (pii), like a card number: it still stays local"},
	{router.SensitivityFinancial, "Kortnummer 4111 1111 1111 1111, utgång 12/27.", false, "a card number is flagged as personal data (pii), not financial: it still stays local"},
	{router.SensitivityFinancial, "We will buy the company for about 40 million next month; keep it quiet. Draft the board memo.", false, "an acquisition in plain words"},
	{router.SensitivityFinancial, "Revenue was 12.4 MSEK, up 8 %. Write the press release.", false, "numbers that are about to be public, no financial term the rules know"},

	// ---- Legal ----------------------------------------------------------------
	{router.SensitivityLegal, "Review this NDA and the indemnification clause before we sign.", true, "NDA, clause"},
	{router.SensitivityLegal, "Sammanfatta stämningsansökan och vårt svaromål i tvisten med leverantören.", true, "stämningsansökan, svaromål"},
	{router.SensitivityLegal, "They say we broke the deal and want money; what do we answer?", false, "a dispute in everyday words"},

	// ---- Source code ----------------------------------------------------------
	{router.SensitivitySourceCode, "func main() {\n\tdb := sql.Open(\"postgres\", dsn)\n\trows, _ := db.Query(\"select * from users\")\n}", true, "code"},
	{router.SensitivitySourceCode, "Traceback (most recent call last):\n  File \"app.py\", line 12, in <module>\n    main()\nKeyError: 'user'", true, "a stack trace"},

	// ---- Harmless (none): must stay none ------------------------------------
	{router.SensitivityNone, "Write a git commit message for this diff: renamed a variable.", true, "harmless"},
	{router.SensitivityNone, "Our checkout service calls the pricing service twice per cart; is that a design smell?", true, "architecture in prose is not code (ISSUE-167; it was over-flagged before)"},
	{router.SensitivityNone, "Write a haiku about routers.", true, "harmless"},
	{router.SensitivityNone, "Mötet flyttas till 08-11-2024; ordernummer 0812-345 678 är levererat.", true, "a date and an order number are not phone numbers"},
	{router.SensitivityNone, "Sammanfatta den här artikeln om Zscaler och Netskope.", true, "harmless, mentions security vendors"},
	{router.SensitivityNone, "Ge mig konfidentiell feedback på mitt CV.", true, "lower-case konfidentiell is prose"},
	{router.SensitivityNone, "Det är ingen hemlighet att vi vill växa; skriv en hemlig önskelista till julfesten.", true, "lower-case hemlig is prose, not a stamp"},
	{router.SensitivityNone, "What does NIS2 require of a mid-sized manufacturer?", true, "a question about a law is not legal material"},
	{router.SensitivityNone, "Explain how a hash chain makes an audit log tamper-evident.", true, "harmless"},
}

func classify(prompt string) string {
	features := classifier.ExtractFromMessages([]openai.Message{{Role: "user", Content: prompt}}, classifier.RequestHints{})
	return classifier.ClassifyRisk(features, "summarization", "").Sensitivity
}

func TestCatchCorpus(t *testing.T) {
	report := os.Getenv("CATCH_REPORT") != ""
	type tally struct{ cases, caught, flagged int }
	byClass := map[router.Sensitivity]*tally{}
	var order []router.Sensitivity
	var misses []string
	for _, c := range catchCorpus {
		got := classify(c.prompt)
		tl := byClass[c.class]
		if tl == nil {
			tl = &tally{}
			byClass[c.class] = tl
			order = append(order, c.class)
		}
		tl.cases++
		exact := got == string(c.class)
		if c.class == router.SensitivityNone {
			// For harmless prompts "caught" means left alone.
			exact = got == "none"
		}
		if exact {
			tl.caught++
		}
		if got != "none" && c.class != router.SensitivityNone {
			tl.flagged++
		}
		if report {
			t.Logf("%-20s got=%-20s caught=%-5v %s", c.class, got, exact, c.prompt)
		}
		if exact != c.caught {
			t.Errorf("%q: classified as %q; the corpus says caught=%v (%s). Update the case and manual/what-sluss-catches.md", c.prompt, got, c.caught, c.why)
		}
		if !c.caught {
			misses = append(misses, fmt.Sprintf("%s: %s", c.class, c.why))
		}
	}
	sort.SliceStable(order, func(i, j int) bool { return order[i] < order[j] })
	var b strings.Builder
	b.WriteString("\n| Kind of data | Cases | Classified as that kind | Flagged as sensitive at all |\n|---|---|---|---|\n")
	total, totalCaught := 0, 0
	for _, cl := range order {
		tl := byClass[cl]
		flagged := fmt.Sprintf("%d of %d", tl.flagged, tl.cases)
		if cl == router.SensitivityNone {
			flagged = "—"
		}
		fmt.Fprintf(&b, "| %s | %d | %d of %d | %s |\n", cl, tl.cases, tl.caught, tl.cases, flagged)
		total += tl.cases
		totalCaught += tl.caught
	}
	fmt.Fprintf(&b, "\nTotal: %d cases, %d classified as the careful reader would (%d%%).\n", total, totalCaught, totalCaught*100/total)
	b.WriteString("\nDocumented misses:\n")
	for _, m := range misses {
		b.WriteString("- " + m + "\n")
	}
	t.Log(b.String())
}
