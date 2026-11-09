#!/usr/bin/env python3
"""Generates benchmark/corpus.jsonl: a labelled set of synthetic questions for
measuring what an AI data gate catches and what it wrongly flags.

Everything is synthetic. Personal identity numbers, IBANs and card numbers are
made up but carry valid check digits, so a checksum-validating detector sees
them as real; no real person, account or key is in here. Keys are written in
formats public documentation uses as examples, or as obviously made-up values.

Each line: {"id", "text", "labels": [classes], "kind", "lang"}
  labels — the data classes a careful reader would give it; [] means harmless
  kind   — identifier | term | meaning | lookalike | plain
           identifier: a value with a format (number, address, key)
           term:       a word or marking that names sensitive data
           meaning:    sensitive only because of what it says
           lookalike:  harmless text that resembles sensitive data
           plain:      ordinary harmless questions

The generator is deterministic (fixed seed), so the corpus only changes when
this file does. Run:  python3 benchmark/generate.py > benchmark/corpus.jsonl
"""
import json
import random
import sys

rng = random.Random(20261110)

FIRST = ["Anna", "Erik", "Maria", "Johan", "Sara", "Lars", "Emma", "Karl", "Elin", "Oskar",
         "Fatima", "Ahmed", "Nora", "Viktor", "Ida", "Mehmet", "Lena", "Peter", "Hanna", "Ali"]
LAST = ["Svensson", "Larsson", "Lindqvist", "Berg", "Nilsson", "Karlsson", "Holm", "Ek",
        "Andersson", "Johansson", "Öberg", "Sjöberg", "Hassan", "Nyström", "Björk"]
STREETS = ["Storgatan", "Kungsgatan", "Industrivägen", "Skolgatan", "Ringvägen",
           "Drottninggatan", "Sjövägen", "Kyrkogatan", "Parkvägen", "Lilla torget"]
TOWNS = [("111 22", "Stockholm"), ("411 03", "Göteborg"), ("211 19", "Malmö"),
         ("753 20", "Uppsala"), ("582 22", "Linköping"), ("903 26", "Umeå")]

COMPANIES = ["Nordkraft AB", "Bryggan Logistik", "Fjällsoft", "Västra Bygg", "Ekholm & Co", "Solvik Energi", "Granit Data", "Havsbris Foods"]
QUARTERS = ["Q1", "Q2", "Q3", "Q4"]
MONTHS = ["January", "March", "May", "September", "November", "februari", "april", "juni", "oktober", "december"]
DRUGS = ["sertraline 50 mg", "metformin 500 mg", "warfarin", "insulin", "ibuprofen 400 mg", "levothyroxine"]
TOPICS = ["electric buses", "remote work", "the new canteen", "our recycling scheme", "the summer party", "cyber hygiene",
          "the spring sales kick-off", "onboarding new staff", "the office move", "customer satisfaction", "energy prices",
          "the coffee machine", "bike-to-work week", "our volunteering day", "the intranet redesign", "flexible hours"]
FORMS = ["a short LinkedIn post", "a newsletter paragraph", "three headline ideas", "a two-line summary", "a polite email",
         "a FAQ entry", "a short speech", "a slide title and three bullets", "a tweet-length update", "an agenda"]

out = []
seen = set()


def add(text, labels, kind, lang):
    """Adds a case unless the same text is already in the corpus: duplicates
    would count the same question twice."""
    if text in seen:
        return False
    seen.add(text)
    out.append({"id": f"{len(out)+1:03d}", "text": text, "labels": sorted(set(labels)),
                "kind": kind, "lang": lang})
    return True


def fill(n, make, labels, kind):
    """Collects n distinct cases from make(), which returns a text."""
    got, tries = 0, 0
    while got < n and tries < n * 200:
        tries += 1
        if add(make(), labels, kind, "mixed"):
            got += 1
    if got < n:
        raise SystemExit(f"only {got} of {n} distinct cases for {labels} {kind}: add templates")


def luhn_digit(digits):
    total = 0
    for i, d in enumerate(reversed(digits)):
        n = int(d)
        if i % 2 == 0:
            n *= 2
            if n > 9:
                n -= 9
        total += n
    return str((10 - total % 10) % 10)


def personnummer(full=False, plus=False):
    y = rng.randint(40, 99)
    m = rng.randint(1, 12)
    d = rng.randint(1, 28)
    n = rng.randint(200, 999)
    base = f"{y:02d}{m:02d}{d:02d}{n:03d}"
    c = luhn_digit(base)
    sep = "+" if plus else "-"
    s = f"{base[:6]}{sep}{base[6:]}{c}"
    return ("19" + s) if full else s


def iban(grouped=True):
    bank = "5000"
    acct = "".join(str(rng.randint(0, 9)) for _ in range(16))
    bban = bank + acct
    # check digits: move "SE00" to the end, letters to numbers, 98 - mod 97
    num = bban + "2814" + "00"   # S=28, E=14
    check = 98 - int(num) % 97
    s = f"SE{check:02d}{bban}"
    if grouped:
        return " ".join(s[i:i+4] for i in range(0, len(s), 4))
    return s


def card():
    body = "4" + "".join(str(rng.randint(0, 9)) for _ in range(14))
    c = body + luhn_digit(body)
    return " ".join(c[i:i+4] for i in range(0, 16, 4))


def phone():
    style = rng.choice(["se-mobile", "se-mobile-flat", "intl", "zero-intl", "paren", "nanp"])
    if style == "se-mobile":
        return f"07{rng.randint(0,9)}-{rng.randint(100,999)} {rng.randint(10,99)} {rng.randint(10,99)}"
    if style == "se-mobile-flat":
        return f"07{rng.randint(0,9)}{rng.randint(1000000,9999999)}"
    if style == "intl":
        return f"+46 7{rng.randint(0,9)} {rng.randint(100,999)} {rng.randint(10,99)} {rng.randint(10,99)}"
    if style == "zero-intl":
        return f"0046 7{rng.randint(0,9)} {rng.randint(100,999)} {rng.randint(10,99)} {rng.randint(10,99)}"
    if style == "paren":
        return f"(555) {rng.randint(100,999)}-{rng.randint(1000,9999)}"
    return f"555-{rng.randint(100,999)}-{rng.randint(1000,9999)}"


def name():
    return f"{rng.choice(FIRST)} {rng.choice(LAST)}"


def email(n):
    first, last = n.lower().split()
    tr = str.maketrans("åäö", "aao")
    return f"{first.translate(tr)}.{last.translate(tr)}@example.com"


def address():
    pc, town = rng.choice(TOWNS)
    return f"{rng.choice(STREETS)} {rng.randint(1, 80)}, {pc} {town}"


# ---------------------------------------------------------------- personal data: identifiers
def C(): return rng.choice(COMPANIES)
def N(): return name()
def F(): return rng.choice(FIRST)

fill(16, lambda: rng.choice([
    f"Summarise the case for customer {personnummer()} at {C()}.",
    f"Write a short reply to {N()} ({rng.choice([personnummer(), personnummer(full=True)])}) about the late delivery.",
    f"Kunden med personnummer {personnummer(full=True)} vill avsluta sitt konto; skriv ett svar.",
    f"Sammanfatta ärendet för {personnummer(plus=rng.random() < 0.2)} och föreslå nästa steg."]), ["pii"], "identifier")
fill(12, lambda: (lambda n: rng.choice([
    f"Draft an email to {email(n)} confirming the meeting on {rng.choice(MONTHS)} {rng.randint(1,28)}.",
    f"Mejla {email(n)} och påminn om fakturan från {C()}.",
    f"Forward the notes from {C()} to {email(n)} with a short summary."]))(N()), ["pii"], "identifier")
fill(14, lambda: rng.choice([
    f"Call {F()} back on {phone()} about the order.",
    f"Ring {F()} på {phone()} och boka ett möte.",
    f"Confirm the callback number {phone()} in the summary for {C()}."]), ["pii"], "identifier")
fill(8, lambda: rng.choice([
    f"{N()}, {address()}, has complained about the delivery. Draft a reply.",
    f"Skicka avtalet till {N()}, {address()}."]), ["pii"], "identifier")
fill(8, lambda: rng.choice([
    f"Write a payment reminder for {C()}; the money goes to {iban()}.",
    f"Betalningen ska till {iban(grouped=rng.random() < 0.5)}; skriv en påminnelse."]), ["pii"], "identifier")
fill(6, lambda: rng.choice([
    f"The customer paid with card {card()}; summarise the dispute.",
    f"Kortnummer {card()}, utgång {rng.randint(1,12):02d}/{rng.randint(26,30)}. Varför nekades köpet?"]), ["pii"], "identifier")

# ---------------------------------------------------------------- personal data: meaning
fill(14, lambda: rng.choice([
    f"Write a letter to our customer {N()} about the delayed refund.",
    f"Rewrite this performance review of {N()}: often late, argues with the team lead.",
    f"{N()} is {rng.randint(24,61)}, works in sales in Göteborg and is going through a divorce. How do we support them?",
    f"Here is the staff list: {N()} (sales, {rng.randint(22,64)}), {N()} (finance, {rng.randint(22,64)}). Who should lead the project?",
    f"Skriv ett brev till vår kund {N()} om den försenade leveransen."]), ["pii"], "meaning")

# ---------------------------------------------------------------- secrets: identifiers
SECRET_VALUES = [
    lambda: "aws_secret_access_key = wJalrXUtnFEMI/K7MDENG/bPxRfiCYEXAMPLEKEY",
    lambda: "AWS_ACCESS_KEY_ID=AKIAIOSFODNN7EXAMPLE",
    lambda: f"password: {rng.choice(['Tr0ub4dor', 'hunter2', 'Sommar2024!', 'correct-horse', 'Vinter#88'])}",
    lambda: f"db_password={rng.choice(['s3cr3t!', 'Passw0rd99', 'letmein42', 'Hav$bris1'])}",
    lambda: f"postgres://app:{rng.choice(['s3cr3tpw', 'hunter22', 'Pa55w0rd', 'Fj4ll!x'])}@db.internal:5432/{rng.choice(['sales','hr','crm'])}",
    lambda: f"Authorization: Bearer {''.join(rng.choice('abcdefghijklmnopqrstuvwxyz0123456789') for _ in range(32))}",
    lambda: f"api_key = \"{''.join(rng.choice('ABCDEFGHJKLMNPQRSTUVWXYZ23456789') for _ in range(28))}\"",
]
fill(24, lambda: rng.choice([
    "Why does this fail?\n{v}", "Varför fungerar inte anslutningen? Konfigen:\n{v}",
    "Clean up this config before I share it with {c}:\n{v}"]).format(v=rng.choice(SECRET_VALUES)(), c=C()),
    ["secrets_possible"], "identifier")
fill(8, lambda: rng.choice([
    f"Use this to log in: {rng.choice(['Tr0ub4dor&3', 'Blue-Kettle-77', 'Sommar2024!', 'Gran!t-42'])} on the {rng.choice(['admin', 'billing', 'backup'])} account.",
    f"Inloggningen till {rng.choice(['servern', 'routern', 'kassasystemet'])} är admin och {rng.choice(['Vinter2023', 'kaffe#99', 'Solvik77'])}, kan du sätta upp ett skript?"]),
    ["secrets_possible"], "meaning")

# ---------------------------------------------------------------- security-classified
fill(16, lambda: rng.choice([
    "HEMLIG enligt säkerhetsskyddslagen. Sammanfatta lägesbilden för {p}.",
    "Begränsat hemlig — skriv en sammanfattning av rapporten om {p}.",
    "KONFIDENTIELL\nAnalysera beredskapsplanen för {p}.",
    "Translate this classified document about {p}, marked NATO RESTRICTED.",
    "Summarise the attached on {p}: RESTREINT UE/EU RESTRICTED.",
    "Detta är säkerhetsskyddsklassificerade uppgifter om {p}; gör en sammanfattning.",
    "KVALIFICERAT HEMLIG. Granska planeringsunderlaget för {p}."]).format(
        p=rng.choice(["vattenverket", "kraftnätet", "ledningscentralen", "the port", "the radar site", "hamnen", "the data centre"])),
    ["security_classified"], "term")
fill(8, lambda: rng.choice([
    "Here is how we protect {p} against sabotage; suggest improvements.",
    "Lista svagheterna i vårt skydd av {p} och föreslå åtgärder.",
    "Our backup site for {p} is in the old bank vault on Ringvägen; draft the access plan."]).format(
        p=rng.choice(["the grid's control systems", "ledningscentralen", "the emergency service", "vattenverket", "the radar site"])),
    ["security_classified"], "meaning")

# ---------------------------------------------------------------- health
fill(16, lambda: rng.choice([
    f"Sammanfatta patientjournalen: diagnos F3{rng.randint(1,4)}.{rng.randint(0,9)}, sjukskriven {rng.choice([25,50,75,100])} %.",
    f"The patient was prescribed {rng.choice(DRUGS)}; write the referral.",
    f"{F()} är sjukskriven i {rng.choice(['två','tre','fyra'])} månader; hur fördelar vi uppgifterna?",
    f"{F()} has been off for {rng.choice(['two','three','six'])} weeks with burnout. Draft a kind message to the team.",
    f"Write a note that {F()} will be away for chemo on {rng.choice(['Mondays','Tuesdays','Fridays'])}.",
    f"Läkemedelslista och behandlingsplan för patienten {F()} ska sammanfattas."]), ["health"], "term")
fill(10, lambda: rng.choice([
    f"{F()} is on leave until {rng.choice(MONTHS)} because of what happened in the spring; who covers the accounts?",
    f"{F()} har haft det tufft sedan olyckan och orkar inte jobba heltid. Skriv ett förslag på anpassning.",
    f"{F()} needs to leave early every {rng.choice(['Monday','Thursday','Friday'])} for treatment. Rework the rota."]), ["health"], "meaning")

# ---------------------------------------------------------------- financial
fill(16, lambda: rng.choice([
    f"Here is the balance sheet and P&L for {rng.choice(QUARTERS)} before the earnings release; summarise for {C()}.",
    f"Sammanfatta bokslutet och resultaträkningen för {C()} innan kvartalsrapporten publiceras.",
    f"The payroll file for {rng.choice(MONTHS)} is attached; check the totals.",
    f"Lönelistan för {rng.choice(MONTHS)} ligger i bilagan, stäm av summorna.",
    f"We have insider information on the merger with {C()}; draft the board memo.",
    f"Gör en due diligence-sammanfattning av årsredovisningen för {C()}."]), ["financial"], "term")
fill(10, lambda: rng.choice([
    f"We will buy {C()} for about {rng.randint(20,90)} million next month; keep it quiet. Draft the memo.",
    f"Revenue was {rng.randint(5,40)}.{rng.randint(0,9)} MSEK, up {rng.randint(2,15)} % — before we announce it Friday, write the press release.",
    f"Vi tänker lägga ett bud på {C()} nästa vecka, skriv ett utkast till styrelsen.",
    f"Our margin collapsed to {rng.randint(1,4)} % this quarter and the bank does not know yet. How do we tell them?"]), ["financial"], "meaning")

# ---------------------------------------------------------------- legal
fill(14, lambda: rng.choice([
    f"Review this NDA with {C()} and the indemnification clause before we sign.",
    f"Sammanfatta stämningsansökan och vårt svaromål i tvisten med {C()}.",
    f"Draft a settlement agreement for the breach of contract claim from {C()}.",
    f"Granska anställningsavtalet och sekretessavtalet för {F()} innan vi skriver under."]), ["legal"], "term")
fill(8, lambda: rng.choice([
    f"{C()} says we broke the deal and wants money; what do we answer?",
    f"Leverantören {C()} hotar med att gå vidare om vi inte betalar, hur svarar vi?",
    f"An ex-employee, {F()}, says they were let go unfairly and wants compensation. Write our reply."]), ["legal"], "meaning")

# ---------------------------------------------------------------- source code
CODE = [
    "```go\nfunc main() {{\n\tdb, err := sql.Open(\"postgres\", dsn)\n\tif err != nil {{ panic(err) }}\n\t_ = db.Ping()\n}}\n```\nWhy does this panic in {e}?",
    "def load_{n}(path):\n    with open(path) as f:\n        return json.load(f)\n\nWhy is this slow on large files in {e}?",
    "Traceback (most recent call last):\n  File \"{n}.py\", line 12, in <module>\n    main()\nKeyError: 'user'",
    "Refactor this:\nfunction total_{n}(items) {{\n  let s = 0;\n  for (const i of items) {{ s += i.price; }}\n  return s;\n}}",
    "SELECT u.id, o.total FROM users u JOIN orders o ON o.user_id = u.id WHERE o.total > {k};\nAdd an index for this query in {e}.",
]
fill(16, lambda: rng.choice(CODE).format(n=rng.choice(["orders","users","invoices","stock","events"]), e=rng.choice(["staging","production","CI"]), k=rng.choice([50,100,500,1000])),
     ["source_code"], "identifier")

# ---------------------------------------------------------------- harmless lookalikes
LOOKALIKES = [
    lambda: f"Mötet flyttas till {rng.randint(1,28):02d}-{rng.randint(1,12):02d}-2025; ordernummer {rng.randint(1000,9999)}-{rng.randint(100,999)} {rng.randint(100,999)} är levererat. Skriv ett kort mejl.",
    lambda: f"Invoice #{rng.randint(1000,9999)}-{rng.randint(100,999)} for {rng.randint(1,9)} {rng.randint(100,999)} kr is attached; draft a polite reminder.",
    lambda: f"Release {rng.randint(1,4)}.{rng.randint(0,20)}.{rng.randint(0,9)} is out; write the changelog intro.",
    lambda: f"Version {rng.randint(1,4)}.{rng.randint(0,20)}.{rng.randint(0,9)}, build {rng.randint(100,999)}.{rng.randint(100,999)}.{rng.randint(10,99)}, fixes the login page; write release notes.",
    lambda: f"Pris: {rng.randint(1,9)} {rng.randint(100,999)} kr inkl. moms, artikelnummer {rng.randint(1000,9999)}-{rng.randint(100,999)} {rng.randint(100,999)}. Skriv en produkttext.",
    lambda: f"Kvartal {rng.randint(1,4)} startar den 1 {rng.choice(['januari','april','juli','oktober'])}; skriv en påminnelse till alla chefer.",
    lambda: f"Our office phone hours are {rng.randint(7,9)}–{rng.randint(15,17)}; write a sign for the door at {C()}.",
    lambda: f"Det är ingen hemlighet att {C()} vill växa; skriv en hemlig önskelista till julfesten.",
    lambda: f"Ge mig konfidentiell feedback på mitt CV för en roll på {C()}.",
    lambda: f"What makes a good password policy for a {rng.choice([20,50,200,1000])}-person company?",
    lambda: f"Explain how API keys should be rotated at {C()}, in general terms.",
    lambda: f"Please be patient with the new {rng.choice(['booking system','intranet','phone system','expense tool'])}; write a short notice to staff.",
    lambda: f"Write a short history of the Swedish personal identity number system for {rng.choice(['a school','a newsletter','new staff'])}.",
    lambda: f"How does a hash chain make an audit log tamper-evident? Explain it for {rng.choice(['a CFO','a new developer','an auditor'])}.",
    lambda: f"Summarise the main points of {rng.choice(['NIS2','DORA','the GDPR','the AI Act'])} for a {rng.choice(['mid-sized manufacturer','municipality','bank','retailer'])}.",
    lambda: f"Explain what an IBAN is to {rng.choice(['a new hire','a customer','a student'])}, without using a real one.",
    lambda: f"Translate 'the meeting is classified as optional' into {rng.choice(['Swedish','German','Finnish'])}.",
    lambda: f"Write a fun quiz about famous court cases in {rng.choice(['history','film','literature'])}.",
    lambda: f"Draft an internal guide to expense reports for {C()}.",
    lambda: f"Write a reminder that the patient-parking spots at {rng.choice(['the clinic','reception'])} are for visitors only.",
]
fill(110, lambda: rng.choice(LOOKALIKES)(), [], "lookalike")

# ---------------------------------------------------------------- harmless plain
fill(110, lambda: rng.choice([
    lambda: f"Write {rng.choice(FORMS)} about {rng.choice(TOPICS)}.",
    lambda: f"Skriv {rng.choice(['ett kort inlägg','tre rubriker','en sammanfattning','ett mejl'])} om {rng.choice(['den nya kantinen','sommarfesten','kontorsflytten','cykelveckan','kaffeautomaten','intranätet'])}.",
    lambda: f"Explain {rng.choice(['TCP vs UDP','DNS','how a VPN works','what a CDN does','what Kubernetes is'])} to {rng.choice(['a beginner','a manager','a ten-year-old'])}.",
    lambda: f"Plan a team lunch for {rng.randint(5,30)} people with {rng.randint(1,5)} vegetarians.",
    lambda: f"Give me {rng.randint(3,10)} interview questions for a {rng.choice(['junior developer','project manager','designer','sales lead'])}.",
    lambda: f"Write a git commit message for a diff that renames {rng.choice(['getUser','loadOrders','calcTotal','sendMail'])} to {rng.choice(['fetchUser','readOrders','sumTotal','dispatchMail'])}.",
])(), [], "plain")

for row in out:
    sys.stdout.write(json.dumps(row, ensure_ascii=False) + "\n")
