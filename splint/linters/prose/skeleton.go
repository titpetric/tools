package prose

import (
	"regexp"
	"strings"
)

// function is the closed class: the words a language keeps a fixed stock of.
//
// Mask every other word and "the frame holds a value" and "the panel holds a
// draft" count as one shape. Leave them in and the report lists the nouns of
// the domain.
var function = words(`
a an the this that these those it its itself they them their theirs
he she his her him hers we us our ours you your i me my mine one ones
is are was were be been being am s re
do does did done doing have has had having
and or but so nor yet for if then than when while where which who whom whose
what why how whether because since unless until although though whereas as
of to in on at by with from into onto over under about across through
between against during before after above below beside besides beyond
within without toward towards upon off out up down per via
not no none never only just also even still already too very much more most
less least enough almost nearly rather quite far
each every both either neither any some all other another same such own
two three four five many few several first second next last
can could may might must shall should will would ought
there here
`)

// subordinator opens a clause holding a reason. One per comment states a
// fact and why. Three states a fact and cannot stop.
//
// A condition is off the list. "If n is negative" states a contract. The
// standard library writes that as often as anyone should.
var subordinator = words(`
because since so which where while although though unless until
whereas whose whom
`)

var (
	// span is a code span, an identifier and not prose.
	span = regexp.MustCompile("`[^`]*`")

	// link is a markdown address: the text is prose, the target is not.
	link = regexp.MustCompile(`\]\([^)]*\)`)

	// word is a word as prose counts one. Apostrophes and hyphens stay in, to
	// keep "doesn't" and "line-by-line" whole.
	word = regexp.MustCompile(`[A-Za-z][A-Za-z'’-]*`)

	// stop ends a sentence: the mark, then a space or a line end. "Go 1.27"
	// ends nothing.
	stop = regexp.MustCompile(`[.!?](?:\s|$)`)
)

// tokens lowercases a block and splits it into words. Code spans and link
// targets drop out first.
func tokens(text string) []string {
	text = span.ReplaceAllString(text, " ")
	text = link.ReplaceAllString(text, "] ")
	return word.FindAllString(strings.ToLower(text), -1)
}

// mask keeps a closed-class word and answers "X" for every other.
func mask(token string) string {
	if function[strings.Trim(token, "'’-")] {
		return token
	}
	return "X"
}

// content is how many slots of a gram hold subject matter.
func content(skeleton []string) int {
	n := 0
	for _, slot := range skeleton {
		if slot == "X" {
			n++
		}
	}
	return n
}

// words reads a whitespace-separated list into a set. The two word classes
// above stand as text and not as slice literals. A reader reads them as a
// list of words.
func words(list string) map[string]bool {
	set := map[string]bool{}
	for _, one := range strings.Fields(list) {
		set[one] = true
	}
	return set
}
