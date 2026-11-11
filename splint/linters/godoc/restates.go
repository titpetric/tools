package godoc

import (
	"regexp"
	"strings"
	"unicode"

	"github.com/titpetric/tools/splint/model"
)

// A comment that says only what the signature says adds no information. The
// fix is a deletion, never a rewrite.
//
// This is the rule a ban list cannot express. A list says "not that". A
// writer told off for one phrase writes another. The sentence still says
// as little. One question has a reply a writer can act on: which nouns does
// the comment add to the declaration above it? Where the reply is none,
// delete the comment.
//
// # What the rule reads
//
// Every godoc opens on the symbol's name and states a verb, because the
// convention and the format rule both require it. Neither one adds
// information: "Name returns" is derivable from `func (d *Decl) Name()`.
//
// So the rule takes them off and reads the words that remain. A comment
// keeping no noun, or none the signature already names, has told a reader
// what the line below already tells them.
//
//	// Name returns the name.                 -> nothing left: flagged
//	// Read reads the frame.                  -> "frame": not in the signature
//	// Close releases the handle it holds.    -> "releases", "handle": kept
//
// # Why it skips a type
//
// A struct's comment names its fields and a reader wants it to. The rule
// reads functions, where the signature is one line and a comment restating
// a comment restating it reads as pure noise. A method is a func with a
// receiver and reads the same.

// restateVerbs are the words a godoc opens on after the symbol's name.
//
// Every one of them is derivable from the signature: a function with a return
// returns, a setter sets. VagueVerb's message lists most of them as the good
// verbs. A blessed verb adds no more information than a banned one.
var restateVerbs = words(`
is are was were be
returns return answers answer gives give provides provide
reports report says say states state tells tell
holds hold carries carry names name
reads read writes write sets set gets get
makes make builds build creates create returns
does do performs perform runs run executes execute
wraps wrap
`)

// stopWords hold grammar and no subject matter. They count as neither
// information nor restatement.
var stopWords = words(`
a an the this that these those it its they them their
of to in on at by with from into over under for and or but
as if then than when while where which who whose what why how
not no only just also even very more most less many much few several
each every both either any some all other another same such own
can could may might must shall should will would
there here one two three
`)

// camel splits an identifier at a lower-to-upper boundary: NewReader reads as
// new and reader.
var camel = regexp.MustCompile(`([a-z0-9])([A-Z])`)

// Restates reports whether a comment says only what the declaration says.
//
// It answers false for anything it cannot judge: a block of names, a type, a
// comment of more than one sentence. A rule about a comment that adds no
// information has no business guessing at one that adds some.
func Restates(decl *model.Declaration, doc string) bool {
	if decl.Kind != model.FuncKind {
		return false
	}
	if len(decl.Names) > 1 {
		return false
	}

	// One sentence, because a second sentence is where a comment says the
	// thing the signature cannot: what it refuses, what it costs, why it
	// exists.
	sentence, rest := firstSentence(doc)
	if sentence == "" || rest != "" {
		return false
	}

	left := content(sentence)
	if len(left) == 0 {
		return false
	}

	// The comment opens on the symbol's name, which the format rule already
	// requires and which says nothing the line below does not.
	if !strings.EqualFold(left[0], decl.Name) {
		return false
	}

	// A verb is free wherever it stands. "Len is how many results there are"
	// holds two of them and no noun beyond the receiver. The question this
	// rule asks is which nouns the comment adds.
	known := vocabulary(decl)
	for _, word := range left[1:] {
		word = strings.ToLower(word)
		if restateVerbs[word] || known[word] {
			continue
		}
		return false
	}

	return true
}

// vocabulary is every word the declaration itself states: the name it
// declares, the receiver it hangs off. The types and names of what it
// takes and answers.
func vocabulary(decl *model.Declaration) map[string]bool {
	known := map[string]bool{}

	add := func(text string) {
		for _, word := range content(camel.ReplaceAllString(text, "$1 $2")) {
			known[strings.ToLower(word)] = true
		}
	}

	add(decl.Name)
	add(decl.Receiver)
	add(decl.Type)
	for _, one := range decl.Arguments {
		add(one)
	}
	for _, one := range decl.Returns {
		add(one)
	}

	return known
}

// content is the words of a sentence that hold subject matter, with the
// grammar and the punctuation taken off.
func content(text string) []string {
	var found []string
	for _, word := range strings.Fields(text) {
		word = strings.Trim(word, ".,:;!?()[]{}\"'`*")
		if word == "" || stopWords[strings.ToLower(word)] {
			continue
		}
		if !unicode.IsLetter(rune(word[0])) {
			continue
		}
		found = append(found, word)
	}
	return found
}

// firstSentence splits a comment at its first full stop, and answers what
// follows it so a caller can tell a one-sentence comment from a longer one.
//
// A paragraph break ends the first sentence too: a comment whose first line
// is a summary and whose second paragraph is the detail has said more than
// the signature, whatever punctuation the first line used.
func firstSentence(doc string) (sentence, rest string) {
	doc = strings.TrimSpace(doc)
	if cut := strings.Index(doc, "\n\n"); cut >= 0 {
		return strings.TrimSpace(doc[:cut]), strings.TrimSpace(doc[cut:])
	}

	for i, r := range doc {
		if r != '.' && r != '!' && r != '?' {
			continue
		}

		// A dot inside a word is an abbreviation, a version or a selector.
		if i+1 < len(doc) && doc[i+1] != ' ' && doc[i+1] != '\n' {
			continue
		}
		return strings.TrimSpace(doc[:i+1]), strings.TrimSpace(doc[i+1:])
	}

	return doc, ""
}

// words reads a whitespace-separated list into a set.
func words(list string) map[string]bool {
	set := map[string]bool{}
	for _, one := range strings.Fields(list) {
		set[one] = true
	}
	return set
}
