package model

// DeclarationKind is an enum of go symbol types.
type DeclarationKind string

// The kinds of declaration a document records.
const (
	StructKind  DeclarationKind = "struct"
	ImportKind                  = "import"
	ConstKind                   = "const"
	TypeKind                    = "type"
	FuncKind                    = "func"
	VarKind                     = "var"
	CommentKind                 = "comment"
)

// String is the kind as written.
func (d DeclarationKind) String() string {
	return string(d)
}
