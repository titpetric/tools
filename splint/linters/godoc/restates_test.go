package godoc

import (
	"testing"

	"github.com/titpetric/tools/splint/model"
)

// fn builds a documented method on a Linter, the shape the rule fires on in
// practice. The receiver widens the vocabulary. A comment naming the
// receiver's own type adds nothing.
func fn(name, doc string, args, returns []string) *model.Declaration {
	return &model.Declaration{
		Kind:      model.FuncKind,
		Name:      name,
		Receiver:  "Linter",
		Doc:       doc,
		Arguments: args,
		Returns:   returns,
	}
}

func TestRestatesReadsWhatTheCommentAdds(t *testing.T) {
	for name, one := range map[string]struct {
		decl *model.Declaration
		want bool
	}{
		"nothing but the name and a verb": {
			fn("Name", "Name is the linter name.", nil, []string{"string"}),
			true,
		},
		"nothing but the name": {
			fn("New", "New returns the linter.", nil, []string{"*Linter"}),
			true,
		},
		"a return type the comment repeats": {
			&model.Declaration{
				Kind:     model.FuncKind,
				Name:     "Float64",
				Receiver: "Number",
				Doc:      "Float64 returns the number as a float64.",
				Returns:  []string{"float64", "error"},
			},
			true,
		},
		"a noun the signature does not hold": {
			fn("Read", "Read reads the frame.", nil, []string{"error"}),
			false,
		},
		"a reason beyond the signature": {
			fn("Close", "Close releases the handle it holds.", nil, []string{"error"}),
			false,
		},
		"an argument the comment names and explains": {
			fn("Write", "Write puts the value in the frame.", []string{"value"}, []string{"error"}),
			false,
		},
		"a second sentence": {
			fn("Name", "Name is the linter name. It labels every issue.", nil, []string{"string"}),
			false,
		},
		"a second paragraph": {
			fn("Name", "Name is the linter name.\n\nIt labels every issue the run reports.", nil, []string{"string"}),
			false,
		},
		"a comment opening on another symbol": {
			fn("Name", "Linter answers its own name.", nil, []string{"string"}),
			false,
		},
	} {
		t.Run(name, func(t *testing.T) {
			if got := Restates(one.decl, one.decl.Doc); got != one.want {
				t.Errorf("Restates() = %v, want %v for %q", got, one.want, one.decl.Doc)
			}
		})
	}
}

func TestRestatesReadsFunctionsOnly(t *testing.T) {
	for name, decl := range map[string]*model.Declaration{
		"a type": {
			Kind: model.TypeKind,
			Name: "Linter",
			Doc:  "Linter is the linter.",
		},
		"a const": {
			Kind: model.ConstKind,
			Name: "Name",
			Doc:  "Name is the name.",
		},
		"a block of names": {
			Kind:  model.FuncKind,
			Name:  "Name",
			Names: []string{"Name", "Other"},
			Doc:   "Name is the linter name.",
		},
	} {
		t.Run(name, func(t *testing.T) {
			if Restates(decl, decl.Doc) {
				t.Errorf("Restates() read a %s", name)
			}
		})
	}
}

func TestRestatesReadsTheReceiverAndTheCamelCase(t *testing.T) {
	receiver := &model.Declaration{
		Kind:     model.FuncKind,
		Name:     "Len",
		Receiver: "Results",
		Doc:      "Len is how many results there are.",
		Returns:  []string{"int"},
	}
	if !Restates(receiver, receiver.Doc) {
		t.Errorf("Restates() kept a comment naming only its receiver: %q", receiver.Doc)
	}

	split := &model.Declaration{
		Kind:    model.FuncKind,
		Name:    "ReadFile",
		Doc:     "ReadFile reads the file.",
		Returns: []string{"error"},
	}
	if !Restates(split, split.Doc) {
		t.Errorf("Restates() did not split the camel case: %q", split.Doc)
	}
}

func TestRestateMessage(t *testing.T) {
	if got := restateMessage(fn("Name", "", nil, nil)); got == "" || !contains(got, "say what the signature cannot") {
		t.Errorf("the message for an exported symbol reads %q", got)
	}
	if got := restateMessage(fn("name", "", nil, nil)); !contains(got, "delete it") {
		t.Errorf("the message for an unexported symbol reads %q", got)
	}
}

func TestFirstSentence(t *testing.T) {
	for doc, want := range map[string][2]string{
		"One sentence.":                      {"One sentence.", ""},
		"One. Two.":                          {"One.", "Two."},
		"A summary.\n\nThe detail.":          {"A summary.", "The detail."},
		"Go 1.27 holds the fix.":             {"Go 1.27 holds the fix.", ""},
		"No closing mark":                    {"No closing mark", ""},
		"A path like io.Reader stays whole.": {"A path like io.Reader stays whole.", ""},
	} {
		sentence, rest := firstSentence(doc)
		if sentence != want[0] || rest != want[1] {
			t.Errorf("firstSentence(%q) = (%q, %q), want (%q, %q)", doc, sentence, rest, want[0], want[1])
		}
	}
}

// contains is strings.Contains, named so the case above reads as a sentence.
func contains(haystack, needle string) bool {
	return len(haystack) >= len(needle) && indexOf(haystack, needle) >= 0
}

func indexOf(haystack, needle string) int {
	for i := 0; i+len(needle) <= len(haystack); i++ {
		if haystack[i:i+len(needle)] == needle {
			return i
		}
	}
	return -1
}
