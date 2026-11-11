package pii

import (
	"regexp"
	"strings"
	"sync/atomic"
	"unicode"
	"unicode/utf8"
)

// Names in free text (ISSUE-168).
//
// A name is a given name from a fixed list followed by a capitalised word
// that reads as a family name: "Karl Öberg", "Fatima Al-Sayed", "Anna Maria
// Lind". It is a rule, not a model: the list is in this file, and the same
// text gives the same answer every time.
//
// What it does not do, by design:
//   - A given name alone ("ask Karl") is not a name. Too many are also words.
//   - It cannot tell a customer from a living public figure: a minister or
//     an athlete is found too. That is an over-flag in the safe direction.
//     People from history are left alone (historical.go).
//   - Given names that are also everyday words (Will, May, Mark, Love, Björn
//     as "björn") are left out of the list.
//
// Once a full name is found, later mentions of its given or family name on
// their own, in the same text, count too, so that mask and send hides them.

// namesEnabled is the switch behind ROUTER_NAME_DETECTION=off.
var namesEnabled atomic.Bool

func init() { namesEnabled.Store(true) }

// SetNameDetection turns name detection on or off for the process.
func SetNameDetection(on bool) { namesEnabled.Store(on) }

// NameDetection reports whether names in free text are detected.
func NameDetection() bool { return namesEnabled.Load() }

// reNameWord is a capitalised word, optionally hyphenated or with an
// apostrophe: Karl, Öberg, Al-Sayed, O'Brien, Ann-Sofie.
var reNameWord = regexp.MustCompile(`\p{Lu}[\p{Ll}]+(?:[-'’]\p{Lu}?[\p{Ll}]+)*`)

// nameParticles may stand between a given and a family name.
var nameParticles = map[string]bool{"von": true, "van": true, "de": true, "af": true, "der": true, "den": true, "del": true, "la": true, "bin": true, "al": true, "el": true}

// notFamilyNames are capitalised words that follow a given name without being
// a family name: places and institutions ("Victoria Station"), calendar
// words, and words capitalised in a heading ("Meeting With Anna About …").
var notFamilyNames = wordSet(`
station street road avenue park square springs city lake island county bay beach valley hill hills
university school college hospital church cathedral airport bridge tower hall house hotel museum
inc ltd llc gmbh group holding holdings bank foundation institute
monday tuesday wednesday thursday friday saturday sunday
january february march april may june july august september october november december
about and are because but for from has have how into not our she the then they this that was were what when
where who why will with you your can could should would shall must also please thanks thank regards
och men att för från med har hade ska kan inte vår våra det den detta dessa som när var vad hur tack hej
gatan vägen torget
`)

// givenNames is the list. Sweden's most common given names, the most common
// names in the languages spoken here, and common English ones. Lower case.
var givenNames = wordSet(`
anna eva maria karin sara kristina lena emma kerstin marie ingrid birgitta malin jenny hanna
linda annika susanne elin monica inger johanna sofia ulla elisabeth katarina helena camilla
åsa sandra ida amanda julia anette ann emelie gunilla siv anita margareta marianne britt
agneta irene barbro yvonne louise cecilia therese frida matilda elsa maja alice ella wilma
ebba olivia astrid saga freja alma agnes klara nora linnea moa tilde lova ellen stina tove
tuva nellie selma signe vera ines leah lilly molly isabelle isabella felicia josefin
josefine rebecka caroline charlotte madeleine pernilla ulrika marika mikaela nathalie petra
jessica victoria viktoria veronica angelica annelie carina catarina christina ellinor erika
gun gunnel maj majken märta rut solveig sonja viola vivianne ylva lisa lotta nina tina sofie
hedvig greta edith tyra thea lovisa alva nova stella juni svea ester elvira
erik lars karl carl gustaf olov eric anders johan per nils lennart mikael jan hans olof peter gunnar sven fredrik
bengt bo daniel gustav åke göran alexander magnus martin andreas stefan leif thomas tomas
mats ulf henrik kjell jonas christer arne rolf bertil ingemar roger marcus mattias håkan
kent niklas patrik christian robert oskar oscar viktor victor emil simon jakob jacob
david filip philip axel anton isak isac albin elias hugo lucas lukas liam noah oliver william
theo leo vincent adam ludvig melvin nils olle sixten harry frank ville elliot alfred arvid
edvin ebbe gabriel samuel josef joel jonathan johannes joakim kristian kristoffer
christoffer linus rasmus sebastian tobias tim tommy tony torbjörn ulrik urban valter wilhelm vilhelm
yngve örjan östen pontus pär ola ove ragnar reine rickard richard rikard rune sigvard
staffan stellan sune sören tage ture uno claes klas conny dennis dick donald evert folke
gert gösta hampus henry holger ingvar jesper jimmy jörgen kalle kenneth kurt lasse
mohammed mohammad mohamed muhammad ahmed ahmad ali hassan hussein omar ibrahim mustafa
mehmet abdullah khalid yusuf youssef hamza amir karim samir tariq bilal reza mehdi
fatima aisha amina maryam mariam zahra layla leila yasmin nour noor huda rania samira zeynep
elif ayse fatma hatice emine
mikko juha matti jukka antti timo kari pekka jari heikki ville tuula ritva pirjo päivi anne
liisa marja sirpa
jan piotr krzysztof andrzej tomasz pawel marcin katarzyna agnieszka malgorzata magdalena
ewa joanna
milan goran zoran dragan marko nikola ivan luka ana ivana jelena marija mirjana
james john michael george paul kevin brian steven edward ronald anthony jason matthew gary
timothy larry jeffrey scott eric stephen andrew raymond gregory joshua jerry
walter patrick douglas arthur ryan joe jack albert justin terry gerald keith roger
mary patricia jennifer elizabeth barbara susan margaret dorothy nancy karen betty helen
donna carol ruth sharon michelle laura sarah kimberly deborah stephanie
rebecca kathleen amy shirley angela brenda pamela emily nicole katherine samantha christine
rachel carolyn janet catherine heather diane julie joyce evelyn
carlos jose juan luis miguel antonio francisco pedro manuel jorge diego pablo sofia lucia
carmen isabel elena pilar teresa
wei ming li hui jun yuki hiroshi kenji akira ravi raj arjun priya anjali sunita deepak sanjay
`)

func wordSet(s string) map[string]bool {
	m := map[string]bool{}
	for _, w := range strings.Fields(s) {
		m[w] = true
	}
	return m
}

// findNames reports every name in text as [start,end) byte offsets.
func findNames(text string) [][2]int {
	if !namesEnabled.Load() {
		return nil
	}
	words := reNameWord.FindAllStringIndex(text, -1)
	if len(words) == 0 {
		return nil
	}
	var out [][2]int
	parts := map[string]bool{} // given and family names of the names found
	used := make([]bool, len(words))
	for i := 0; i < len(words); i++ {
		w := words[i]
		if !letterBounded(text, w[0], w[1]) || !givenNames[strings.ToLower(text[w[0]:w[1]])] {
			continue
		}
		// The family name: the next capitalised word after one space, or
		// after a particle ("Anna von Essen").
		j, end := i+1, -1
		if j < len(words) && familyFollows(text, w[1], words[j]) {
			end = words[j][1]
			// A second given name is followed by the family name:
			// "Anna Maria Lind".
			if givenNames[strings.ToLower(text[words[j][0]:words[j][1]])] && j+1 < len(words) && familyFollows(text, words[j][1], words[j+1]) {
				j++
				end = words[j][1]
			}
		}
		if end < 0 {
			continue
		}
		// A street, a square or a school named after someone is a place:
		// "Karl Johans gate", "Olof Palmes plats", "Anna Whitlocks gymnasium".
		if placeFollows(text, end) {
			for k := i; k <= j; k++ {
				used[k] = true
			}
			i = j
			continue
		}
		// A person from history is not a person whose data needs keeping
		// in the house (historical.go).
		if isHistoricalFigure(text[w[0]:end]) {
			for k := i; k <= j; k++ {
				used[k] = true
			}
			i = j
			continue
		}
		out = append(out, [2]int{w[0], end})
		for k := i; k <= j; k++ {
			used[k] = true
			parts[text[words[k][0]:words[k][1]]] = true
		}
		i = j
	}
	if len(out) == 0 {
		return nil
	}
	// Later mentions of a part on its own: "Karl Öberg … tell Karl that".
	for i, w := range words {
		if used[i] || !letterBounded(text, w[0], w[1]) {
			continue
		}
		word := text[w[0]:w[1]]
		if parts[word] || parts[strings.TrimSuffix(word, "s")] && strings.HasSuffix(word, "s") {
			out = append(out, [2]int{w[0], w[1]})
		}
	}
	return out
}

// familyFollows reports whether next is a family name directly after the word
// ending at prevEnd: one space between them, or a space, a particle and a
// space; and next is not a word that only looks like one.
func familyFollows(text string, prevEnd int, next []int) bool {
	gap := text[prevEnd:next[0]]
	if gap != " " {
		p := strings.TrimSpace(gap)
		if !nameParticles[p] || gap != " "+p+" " {
			return false
		}
	}
	if !letterBounded(text, next[0], next[1]) {
		return false
	}
	word := text[next[0]:next[1]]
	if utf8.RuneCountInString(word) < 2 {
		return false
	}
	return !notFamilyNames[strings.ToLower(word)]
}

// letterBounded reports whether [start,end) is a whole word: no letter or
// digit directly before or after it. Go's \b is ASCII-only, so Åsa needs this.
func letterBounded(s string, start, end int) bool {
	if start > 0 {
		r, _ := utf8.DecodeLastRuneInString(s[:start])
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			return false
		}
	}
	if end < len(s) {
		r, _ := utf8.DecodeRuneInString(s[end:])
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			return false
		}
	}
	return true
}

// HasGivenName reports whether text holds a capitalised given name from the
// list, with or without a family name: "Erik", "Fatima Öberg".
func HasGivenName(text string) bool {
	if !namesEnabled.Load() {
		return false
	}
	for _, w := range reNameWord.FindAllStringIndex(text, -1) {
		if letterBounded(text, w[0], w[1]) && givenNames[strings.ToLower(text[w[0]:w[1]])] {
			return true
		}
	}
	return false
}

// placeWords follow a name that has become the name of a place.
var placeWords = wordSet(`
gate gata gatan gade väg vägen vei vej torg torget plass plats platsen plads allé alle esplanad park parken bro bron
skola skolan gymnasium gymnasiet sjukhus sjukhuset stadion arena kyrka kyrkan museum museet
street road avenue boulevard square bridge airport school hospital stadium institute university
strasse straße platz
`)

// placeFollows reports whether the word after the name ending at end is one
// that makes it a place name.
func placeFollows(text string, end int) bool {
	rest := text[end:]
	if !strings.HasPrefix(rest, " ") {
		return false
	}
	rest = rest[1:]
	n := 0
	for n < len(rest) {
		r, size := utf8.DecodeRuneInString(rest[n:])
		if !unicode.IsLetter(r) {
			break
		}
		n += size
	}
	return n > 0 && placeWords[strings.ToLower(rest[:n])]
}
