package splint

import (
	"context"

	"github.com/titpetric/tools/splint/model"
)

// Parser reads source and produces the document every linter works on.
//
// There is more than one way to fill it. The ast parser builds a syntax tree
// and is exact; the simple parser reads lines and braces, is faster, and keeps
// going through source that does not compile. Both are constructed as
// New(Options) and both return this, so which one a program uses is an import
// and nothing more.
type Parser interface {
	Parse(ctx context.Context) (*model.DocumentRoot, error)
}
