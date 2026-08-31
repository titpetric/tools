package model

// Field holds details about a field definition.
type Field struct {
	// Name is the name of the field.
	Name string `json:"Name" yaml:"Name"`

	// Type is the literal type of the Go field.
	Type string `json:"Type,omitempty" yaml:"Type,omitempty"`

	// Path is the go path of this field starting from root object.
	Path string `json:"Path" yaml:"Path"`

	// Doc holds the field doc.
	Doc string `json:"Doc,omitempty" yaml:"Doc,omitempty"`

	// Comment holds the field comment text.
	Comment string `json:"Comment,omitempty" yaml:"Comment,omitempty"`

	// Tag is the go tag, unmodified.
	Tag string `json:"Tag,omitempty" yaml:"Tag,omitempty"`

	// JSONName is the corresponding json name of the field.
	// It's cleared if it's set to `-` (unexported).
	JSONName string `json:"JSONName" yaml:"JSONName"`

	// MapKey is the map key type, if this field is a map.
	MapKey string `json:"MapKey,omitempty" yaml:"MapKey,omitempty"`
}

func (f *Field) TypeRef() string {
	return TypeRef(f.Type)
}
