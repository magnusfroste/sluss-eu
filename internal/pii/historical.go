package pii

import "strings"

// Historical figures (ISSUE-173).
//
// The name rule cannot tell a customer from an author, so a school essay
// about Astrid Lindgren was treated as personal data. Data protection law
// protects living people; the people below died long ago or are otherwise
// part of history, and a question about them is a question about history.
//
// The list is deliberately only people who are no longer alive. A living
// public figure (a politician in office, an athlete, an activist) is still a
// living person and is still found: that is an over-flag in the safe
// direction, and the list will not grow to cover them.
//
// A name on the list is skipped only as a name. A personal identity number,
// an address or a health term in the same text still counts.
var historicalFigures = nameSet(`
Astrid Lindgren; Selma Lagerlöf; August Strindberg; Alfred Nobel; Olof Palme; Dag Hammarskjöld; Anders Celsius;
Carl von Linné; Carl Linnaeus; Ingrid Bergman; Ingmar Bergman; Greta Garbo; Raoul Wallenberg; Tage Erlander;
Per Albin Hansson; Hjalmar Branting; Gustav Vasa; Gustav II Adolf; Karl XII; Evert Taube; Carl Michael Bellman;
Vilhelm Moberg; Tove Jansson; Karin Boye; Edith Södergran; Gustaf Fröding; Verner von Heidenstam; Hjalmar Söderberg;
Pär Lagerkvist; Harry Martinson; Tomas Tranströmer; Stieg Larsson; Henning Mankell; Elsa Beskow; Carl Larsson;
Anders Zorn; Hilma af Klint; Jenny Lind; Birgit Nilsson; Jussi Björling; Povel Ramel; Cornelis Vreeswijk;
Emanuel Swedenborg; Jöns Jacob Berzelius; Carl Wilhelm Scheele; Svante Arrhenius; Anders Jonas Ångström;
John Ericsson; Gustaf de Laval; Lars Magnus Ericsson; Ingvar Kamprad; Folke Bernadotte; Ellen Key; Fredrika Bremer;
Alva Myrdal; Gunnar Myrdal; Elsa Brändström; Kata Dalström; Axel Oxenstierna; Engelbrekt Engelbrektsson;
Henrik Ibsen; Edvard Grieg; Edvard Munch; Roald Amundsen; Fridtjof Nansen; Knut Hamsun; Thor Heyerdahl;
Hans Christian Andersen; Søren Kierkegaard; Niels Bohr; Karen Blixen; Tycho Brahe; Jean Sibelius; Elias Lönnrot;
Albert Einstein; Marie Curie; Isaac Newton; Charles Darwin; Galileo Galilei; Johannes Kepler; Nikola Tesla;
Thomas Edison; Alexander Graham Bell; Louis Pasteur; Gregor Mendel; Michael Faraday; James Clerk Maxwell;
Max Planck; Werner Heisenberg; Erwin Schrödinger; Enrico Fermi; Richard Feynman; Stephen Hawking; Alan Turing;
Ada Lovelace; Grace Hopper; John von Neumann; Claude Shannon; Rosalind Franklin; Alexander Fleming; Florence Nightingale;
Sigmund Freud; Carl Jung; Ivan Pavlov; Leonardo da Vinci; Michelangelo Buonarroti; Vincent van Gogh; Pablo Picasso;
Claude Monet; Rembrandt van Rijn; Frida Kahlo; Salvador Dalí; Johann Sebastian Bach; Wolfgang Amadeus Mozart;
Ludwig van Beethoven; Franz Schubert; Frédéric Chopin; Giuseppe Verdi; Richard Wagner; Pyotr Tchaikovsky;
William Shakespeare; Charles Dickens; Jane Austen; Mark Twain; Leo Tolstoy; Fyodor Dostoevsky; Victor Hugo;
Johann Wolfgang von Goethe; Friedrich Schiller; Franz Kafka; James Joyce; Virginia Woolf; George Orwell;
Ernest Hemingway; Agatha Christie; Oscar Wilde; Emily Dickinson; Edgar Allan Poe; Jules Verne; Miguel de Cervantes;
Dante Alighieri; Immanuel Kant; Friedrich Nietzsche; Karl Marx; Adam Smith; John Maynard Keynes; John Locke;
Jean-Jacques Rousseau; René Descartes; Baruch Spinoza; Thomas Hobbes; David Hume; Hannah Arendt; Simone de Beauvoir;
Abraham Lincoln; George Washington; Thomas Jefferson; Benjamin Franklin; Franklin Roosevelt; Theodore Roosevelt;
John Kennedy; Martin Luther King; Rosa Parks; Nelson Mandela; Mahatma Gandhi; Winston Churchill; Margaret Thatcher;
Napoleon Bonaparte; Otto von Bismarck; Charles de Gaulle; Willy Brandt; Helmut Kohl; Konrad Adenauer;
Vladimir Lenin; Joseph Stalin; Mikhail Gorbachev; Mao Zedong; Julius Caesar; Marcus Aurelius; Martin Luther;
Jeanne d'Arc; Joan of Arc; Christopher Columbus; Marco Polo; Ferdinand Magellan; Neil Armstrong; Yuri Gagarin;
Amelia Earhart; Charles Lindbergh; Henry Ford; Steve Jobs; Walt Disney; Charlie Chaplin; Marilyn Monroe;
Elvis Presley; John Lennon; Freddie Mercury; David Bowie; Louis Armstrong; Ella Fitzgerald; Maria Callas;
Anne Frank; Helen Keller; Mother Teresa; Emmeline Pankhurst; Mary Wollstonecraft; Harriet Tubman; Queen Victoria
`)

func nameSet(s string) map[string]bool {
	m := map[string]bool{}
	for _, n := range strings.Split(s, ";") {
		if n = strings.Join(strings.Fields(n), " "); n != "" {
			m[strings.ToLower(n)] = true
		}
	}
	return m
}

// isHistoricalFigure reports whether name, as found in a text, is on the
// list. A possessive form counts: "Astrid Lindgrens böcker", "Darwin's".
func isHistoricalFigure(name string) bool {
	n := strings.ToLower(strings.Join(strings.Fields(name), " "))
	if historicalFigures[n] {
		return true
	}
	for _, suffix := range []string{"'s", "’s", "s"} {
		if strings.HasSuffix(n, suffix) && historicalFigures[strings.TrimSuffix(n, suffix)] {
			return true
		}
	}
	return false
}
