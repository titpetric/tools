package prose

// This file holds the reasoning. The functions say what they do. One reader
// meets the why once here.

// # What the measure is for
//
// A prose linter is a list of banned phrases. A list catches only what
// somebody thought to name.
//
// The deeper fault is that a writer told "not that" writes around it. Ban a
// word and the next draft holds a synonym. No step along the way tests whether
// the sentence states what the code does.
//
// So this holds no list. It compares one body of writing against another and
// reports what the first leans on. The findings come out of the measurement.
// A construction nobody had noticed then ranks beside one everybody argues
// about.
//
// # Why the mask covers the subject matter
//
// Every word outside the closed class becomes "X" before the count.
//
// Two trees are about different things. Count their words plain and the
// report lists their nouns: compilers in one, editors in the next, invoices
// in a third. Masked, "the frame holds a value" and "the panel holds a draft"
// are one shape, and what remains to compare is the syntax.
//
// A gram needs two function words. One leaves a content word and its
// neighbours, which states no shape.
//
// A one-word gram is the exception and needs its slot closed-class. That is
// how a single preposition shows up. A word the baseline never writes is the
// strongest evidence here, and such a word has no neighbour to pair with.
//
// # Why the ranking is not by ratio
//
// Six uses in twenty thousand words against one in two million reads as six
// hundred times the rate. The small sample accounts for most of it.
// Rank on the ratio and the report opens with noise.
//
// The ranking takes Dunning's log-likelihood, which weighs the gap against
// the prose behind it. A hundred uses at ten times the rate then outrank a
// handful at six hundred. The ratio still prints: a reader understands a
// ratio and nobody reads a log-likelihood. The report prints it for a reader,
// never for a decision.
//
// # Why the standard library is the baseline
//
// Two million words of comments by a team that writes Go for a living. A
// construction they write freely belongs to the language. A rule against
// it bans Go. One they never write belongs to the writer.
//
// The second direction is the useful one. A rule measured against the
// standard library before anyone writes it cannot be about taste. Four
// candidate rules fell at that test while this package went in.
//
// # Why the densities matter more than the shapes
//
// A banned phrase has a synonym. A sentence of thirty words has no synonym of
// ten.
//
// The length and clause measures are the half no rewrite dodges: how far a
// comment block runs, how far a sentence runs, how often a clause attaches a
// reason, how often two of them land in one block. A writer who has to stop
// welding reasons onto finished sentences has to decide which fact the
// sentence is about. No word substitution reaches that decision.
//
// The lengths compare doc comments alone, since the baseline is doc comments.
// A markdown paragraph runs as long as the thought behind it. The standard
// library holds no paragraph to judge that against.
//
// The shapes read the markdown too. A shape's rate is per word. A
// construction counts wherever it stands.
//
// # Why placement answers to no baseline
//
// The shapes and the densities measure how a sentence reads. Placement
// measures where a comment sits, which a corpus of Go comments says nothing
// about: the standard library holds no configuration file.
//
// A YAML file documents its own keys through their names. One line above a
// top-level key names the block; a nested key, a list item and a value have
// the key above them for context and take none. The argument for a nested
// key belongs in the document beside the file, where it is read and revised
// as a document. So the rule is structural, reports from the depth the
// yamlprose walk recorded, and runs whether or not a baseline loaded.
//
// # What this does not reach
//
// Whether a comment is true, and whether it says anything at all.
//
// A comment asserting a cause the code does not have measures exactly like
// one that is right. So does a comment defining a word where it should state
// what the function does.
//
// A measure reaches past neither. A reader still has to read. This says where
// to look.
