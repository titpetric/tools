# Calibration

A rule that fires on the Go standard library is measuring taste, not error.

Go's team writes Go for a living. A construction they write freely belongs to
the language. A rule against it bans Go. One they never write belongs to the
writer.

That is the test every rule here answers to. It costs one command: copy a
dozen standard library packages into a directory, point vale at them with this
style, and count.

```bash
mkdir /tmp/std && cd /tmp/std
for p in strings bytes io os errors sort bufio net/http encoding/json time fmt regexp; do
  mkdir -p $(dirname $p); cp -r $(go env GOROOT)/src/$p $(dirname $p)/
done
docker run --rm -v /tmp/std:/w -w /w registry.cdn.si/vale --output=line . | wc -l
```

## What the measurement found

Twelve packages, **10,361 findings**. Half of them came from one rule.

| Rule | Before | After | What the count was measuring |
|---|---|---|---|
| `Passive` | 5,142 | removed | 1,198 of them were the BSD licence header in every file: `is governed by`, `can be found in`. The rest was Go's contract idiom: `ErrNegative is returned`, `the file is closed`. Passive voice is an inverted tell, which a human technical writer writes freely and a model avoids. |
| `TooWordy` | 1,704 | 211 | 118 of its 216 tokens were single words. It flagged `multiple`, `equivalent`, `maximum`, `implement`, `subsequent`, `additional`, `However`: vocabulary, never wordiness. `it is` accounted for another 447, and `It is safe to call Close twice` is how Go states a contract. |
| `WasState` | 461 | 44 | A bare `was` matched `the first non-EOF error that was encountered`. A past participle in a relative clause is the only way to say a thing already happened. The rule now takes the auxiliary opening a clause, and the 44 survivors are real: `Was a bug:`, `Used to be issue 8305`. |
| `Assumption` | 516 | 222 | `should be` was 293 of it. `the buffer should be at least n bytes` states a requirement, never a state of mind. |
| `VagueVerb` | 349 | 159 | `gives` and `giving` were 74. Go documents a return that way. `gives back` stays, because nothing returns backwards. |
| `NegativeParallelism` | 205 | ~50 | `instead of` and `rather than` were 153 between them. Both are ordinary English. |
| `Anthropomorphism` | 230 | 123 | `wants to` was 93, nearly every one of them `if you want to`, which addresses a reader and animates no code. |

**10,361 to 2,565.** Every cut is one of those counts. `Passive` and
`TooWordy` were both verbatim write-good vendoring, calibrated against no
corpus at all.

## What the measurement protects

Nineteen of these rules fire **zero** times on the standard library. Those are
the well-aimed ones. They are the reason the style is worth running.

The three added most recently were measured before they were written, not
after:

| Rule | Rate against the standard library |
|---|---|
| `Posture` | `sits on` 240x, `stands on` 17x, `lives in` 5x |
| `OwnAppositive` | `of its own` 98x, `on its own` 17x |
| `QuantifierSubject` | `has none` 124x, `and none` 12x, `neither is` 11x |

Four candidates failed the same test and never went in: `none is` at 3.1x,
`both are` at 1.4x, bare `its own` at 11.9x over 305 standard library uses, and
`either is`, which this tree never writes.

## Two things the tokens cannot do

**A wrapped phrase needs `\s+`.** A literal space never matches a newline, so
`'\bmatters because\b'` passes a comment wrapped after "matters". 1,020 tokens
across 35 rules held a literal space, and every one of them passed a wrapped
phrase in silence. They are swept now, for 45 more findings across this
workspace and no more silent misses.

**RE2 reads neither lookahead nor lookbehind.** `on its own line` is the bare
possessive and legal. `measured on its own.` is the appositive and not. With
no lookahead the rule matches the punctuation instead of asserting it. Where even
that fails, the gap is written into the rule file: see `WasState.yml`.

## What a regex cannot reach at all

Whether a comment is true, and whether it says anything.

`splint --linters=prose` answers the second from the other direction. It masks
the subject matter out of every sentence, counts what remains, and reports the
shapes a tree writes more often per word than the standard library does. The findings
come out of the measurement and not a list. That is how `beside` turned up at
95 uses here against **one** in two million words of Go comments.

`splint --linters=godoc` holds the rule a ban list cannot express. A comment
whose content words all come off the signature above it says nothing. The fix
is deletion instead of a rewrite. It fires on 0.36% of documented
functions in the standard library, and reading those found no false one.
