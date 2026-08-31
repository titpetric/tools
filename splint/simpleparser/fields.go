package simpleparser

import (
	"reflect"
	"strings"

	"github.com/titpetric/tools/splint/model"
)

// structFields reads the fields of a struct body, which runs from the line the
// type opens on to the brace that closes it.
//
// A field declaring several names is several fields, and an embedded field
// declares none: it is recorded under the empty name, which is how a renderer
// tells it from a named one.
func structFields(src *source, open, close int) model.FieldList {
	var fields model.FieldList

	for i := open + 1; i < close; i++ {
		code := strings.TrimSpace(src.codeLine(i))
		if code == "" {
			continue
		}

		// A field body of its own, an anonymous struct or interface, is read
		// as the one type it is and its inside is skipped.
		// A field with a body of its own, an anonymous struct or interface, is
		// read as the whole of what it declares: the type is every line of it,
		// and the tag sits on the line that closes it.
		skip, nested := i, false
		if strings.HasSuffix(code, "{") {
			skip = bodyEnd(src, i)
			nested = skip > i
		}

		written, comment := src.split(i)
		name, typ, tag := splitField(code, written)
		if nested {
			name, typ = nestedField(src, i, skip)
			tagged, _ := src.split(skip)
			tag = fieldTag(tagged)
		}
		if typ == "" {
			i = skip
			continue
		}

		for _, goName := range strings.Split(name, ",") {
			goName = strings.TrimSpace(goName)
			fields = append(fields, &model.Field{
				Doc:      docAbove(src, i),
				Comment:  commentOn(comment),
				Name:     goName,
				Path:     fieldPath(src, open, goName),
				Type:     symbolType(typ),
				Tag:      tag,
				JSONName: jsonName(tag, goName),
			})
		}

		i = skip
	}

	return fields
}

// interfaceMethods reads the methods of an interface body. A method is
// recorded under its name, with its signature as the type, which is how the
// model carries a method set.
func interfaceMethods(src *source, from, to int) model.FieldList {
	var fields model.FieldList

	for i := from + 1; i < to; i++ {
		code := strings.TrimSpace(src.codeLine(i))
		if code == "" {
			continue
		}

		open := strings.Index(code, "(")
		if open <= 0 {
			// An embedded interface, which the collector records as the one
			// unnamed field it is.
			fields = append(fields, &model.Field{Type: "interface"})
			continue
		}

		name := strings.TrimSpace(code[:open])
		close := matchParen(code, open)
		if name == "" || close < 0 {
			continue
		}

		params := paramGroups(code[open+1 : close])
		results := resultGroups(strings.TrimSpace(code[close+1:]))

		fields = append(fields, &model.Field{
			Name: name,
			Type: signature(name, params, results),
		})
	}

	return fields
}

// splitField separates one struct field into its names, its type and its tag.
//
// The stripped view is what the split reads, since a tag is a string and a
// comment may follow the field; the tag itself is taken from the line as
// written, which is the only place it survives.
func splitField(code, written string) (name, typ, tag string) {
	tag = fieldTag(written)

	// The tag and anything after it are not part of the type.
	if tick := strings.Index(written, "`"); tick >= 0 && tick < len(code) {
		code = strings.TrimSpace(code[:tick])
	}
	code = strings.TrimSpace(code)
	if code == "" {
		return "", "", tag
	}

	space := indexTop(code, ' ')
	if space < 0 {
		// An embedded field is a type with no name of its own.
		return "", code, tag
	}

	head := strings.TrimSpace(code[:space])
	rest := strings.TrimSpace(code[space+1:])
	if rest == "" {
		return "", head, tag
	}

	// Every name of a group has to be an identifier; anything else is a type
	// that happens to hold a space, such as "chan int" or "map[a]b".
	for _, part := range strings.Split(head, ",") {
		if !isIdentifier(strings.TrimSpace(part)) {
			return "", code, tag
		}
	}

	return head, rest, tag
}

// fieldTag returns the struct tag of a line, without its backticks.
func fieldTag(line string) string {
	open := strings.Index(line, "`")
	if open < 0 {
		return ""
	}
	close := strings.LastIndex(line, "`")
	if close <= open {
		return ""
	}
	return line[open+1 : close]
}

// commentOn returns the trailing comment of a line, which is the comment a
// field carries beside it rather than above it.
func commentOn(comment string) string {
	return strings.TrimSpace(strings.TrimSuffix(strings.TrimPrefix(strings.TrimSpace(comment), "//"), "*/"))
}

// fieldPath is the field named from the type down, which is what the model
// records so a field can be found without its declaration.
func fieldPath(src *source, open int, name string) string {
	code := strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(src.codeLine(open)), "type"))
	typeName, _, _ := strings.Cut(code, " ")
	typeName = trimTypeParams(strings.TrimSpace(typeName))

	if typeName == "" {
		return name
	}
	if name == "" {
		return typeName
	}
	return typeName + "." + name
}

// jsonName is the name a field encodes to, which is the json tag when it has
// one and the field name when it has not. A field tagged "-" encodes to
// nothing and is recorded as such.
func jsonName(tag, name string) string {
	value := reflect.StructTag(tag).Get("json")
	if value == "" {
		return name
	}
	value, _, _ = strings.Cut(value, ",")
	switch value {
	case "":
		return name
	case "-":
		return ""
	}
	return value
}

// nestedField reads a field whose type is written out in place, and returns
// the name it declares and every line of what it is.
//
// The collector prints the type through go/printer, which writes it back as it
// was written, so what the model records is the declaration itself rather than
// a summary of it.
func nestedField(src *source, open, close int) (name, typ string) {
	written, _ := src.split(open)
	head := strings.TrimSpace(written)

	space := indexTop(head, ' ')
	if space < 0 {
		return "", strings.TrimSpace(src.text(open, close))
	}
	if candidate := strings.TrimSpace(head[:space]); isIdentifier(candidate) {
		name = candidate
		head = strings.TrimSpace(head[space+1:])
	}

	lines := []string{head}
	for i := open + 1; i <= close; i++ {
		line, _ := src.split(i)
		lines = append(lines, strings.TrimRight(line, " \t"))
	}

	// The closing line carries the tag, which is not part of the type.
	last := lines[len(lines)-1]
	if tick := strings.Index(last, "`"); tick >= 0 {
		lines[len(lines)-1] = strings.TrimRight(last[:tick], " \t")
	}

	return name, strings.Join(lines, "\n")
}
