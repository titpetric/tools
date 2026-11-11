package yamlprose

import (
	"regexp"
	"strings"
)

// A configuration file holds far more data than prose: a path, a module, a
// version, a shell command, an identifier, a glob. No rule about writing
// judges any of them, and reporting them would bury the sentences nobody has
// linted yet.
//
// So Sentence asks three things of a value, structurally and with no schema
// in hand. It runs to three words or more. No word holds the punctuation a
// path or a command holds. At least one word belongs to the closed class.
//
// The last question does the work. A sentence holds a preposition or an
// article, and "go install ./cmd/splint" holds neither. A three-word label
// with a conjunction in it passes. Three words of English are three words of
// English.

// Sentence reports whether a YAML value reads as writing or as data. The
// three questions it asks are the comment above this one.
func Sentence(value string) bool {
	value = strings.TrimSpace(value)

	words := strings.Fields(value)
	if len(words) < 3 {
		return false
	}
	if command.MatchString(value) {
		return false
	}

	var held int
	for _, word := range words {
		if data.MatchString(word) {
			return false
		}
		if function[strings.ToLower(strings.Trim(word, ".,:;!?()\"'`"))] {
			held++
		}
	}

	return held > 0
}

var (
	// data is the punctuation of a path, a module, a flag, a template or a
	// variable. One word holding any of it marks the value as something a
	// program reads.
	//
	// A full stop and a comma are absent: a sentence ends in one and holds
	// the other. The slash catches a path. The digits around a dot catch
	// a version.
	data = regexp.MustCompile(`[/\\${}<>|*=]|^-{1,2}\w|@|::|\d+\.\d+|\.(?:go|md|yml|yaml|json|sql|php|sh|ts|js|css|html|tpl|puml|svg)$`)

	// command is the shape of a line somebody runs. A value opening on one of
	// these is a command, whatever its flags hold.
	command = regexp.MustCompile(`^(?:go|git|docker|npm|npx|task|atkins|splint|mdox|php|composer|make|bash|sh|curl|cd|rm|cp|mv|find|cat|echo|test|if|for|while)\b`)
)

// function is the closed class: the words a language keeps a fixed stock of.
//
// A sentence holds one. The list runs shorter than the measure's own. The
// question here is whether somebody wrote a sentence. The measure asks
// afterwards what shape it has.
var function = words(`
a an the this that these those it its they them their
is are was were be been being am
and or but so nor for if then than when while where which who whose what why how
of to in on at by with from into over under about across through
between against during before after above below beside within without
not no never only just also even still already too very more most less
each every both either any some all other another same such own
can could may might must shall should will would
there here
`)

// words reads a whitespace-separated list into a set.
func words(list string) map[string]bool {
	set := map[string]bool{}
	for _, one := range strings.Fields(list) {
		set[one] = true
	}
	return set
}
