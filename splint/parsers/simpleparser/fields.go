package simpleparser

import (
	"reflect"
	"strings"

	"github.com/titpetric/tools/splint/model"
)

// structFields reads the fields of a struct body, which runs from the line the
// type opens on to the brace that closes it.
//
// A field declaring three names is three fields, and an embedded field
// declares none: the parser records it under the empty name, so a renderer
// tells it from a named one.
func structFields(src *source, open, close int) model.FieldList {
	// A struct written on the line the type opens on carries its fields
	// between the braces: "type source struct{ array *model.Array }".
	if open == close {
		return inlineStructFields(src, open)
	}

	var (
		fields  model.FieldList
		pending []string
	)

	for i := open + 1; i < close; i++ {
		code := strings.TrimSpace(src.codeLine(i))
		if code == "" {
			continue
		}

		// A name group may run over more than one line, as in "Alloc,",
		// "TotalAlloc," up to "PauseTotalNs uint64": the names accumulate until
		// the line carrying the type arrives, and share it.
		if names, ok := nameGroupLine(code); ok {
			pending = append(pending, names...)
			continue
		}

		// A field with a body of its own, an anonymous struct or interface,
		// reads as the whole of what it declares: the type is every line of it,
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

		names := strings.Split(name, ",")
		for j := range names {
			names[j] = strings.TrimSpace(names[j])
		}
		names = append(pending, names...)
		pending = nil

		for _, goName := range names {
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

// inlineStructFields reads the fields of a struct written on the line its type
// opens on, separated by semicolons.
func inlineStructFields(src *source, line int) model.FieldList {
	written := src.line(line)
	code := src.codeLine(line)

	open := strings.Index(code, "{")
	close := strings.LastIndex(code, "}")
	if open < 0 || close <= open || close >= len(written) {
		return nil
	}

	var fields model.FieldList
	for _, part := range splitInline(written[open+1 : close]) {
		partCode, _, _ := strip(part, openState{})
		name, typ, tag := splitField(partCode, part)
		if typ == "" {
			continue
		}

		for _, goName := range strings.Split(name, ",") {
			goName = strings.TrimSpace(goName)
			fields = append(fields, &model.Field{
				Name:     goName,
				Path:     fieldPath(src, line, goName),
				Type:     symbolType(typ),
				Tag:      tag,
				JSONName: jsonName(tag, goName),
			})
		}
	}

	return fields
}

// splitInline splits a one-line field list on the semicolons between fields,
// leaving a semicolon inside a struct tag alone.
func splitInline(list string) []string {
	var parts []string

	start, inTag := 0, false
	for i := 0; i < len(list); i++ {
		switch list[i] {
		case '`':
			inTag = !inTag
		case ';':
			if !inTag {
				parts = append(parts, strings.TrimSpace(list[start:i]))
				start = i + 1
			}
		}
	}

	if rest := strings.TrimSpace(list[start:]); rest != "" || len(parts) > 0 {
		parts = append(parts, rest)
	}
	return parts
}

// interfaceMethods reads the methods of an interface body. The parser
// records a method under its name, with its signature as the type, the way the
// model carries a method set.
func interfaceMethods(src *source, from, to int) model.FieldList {
	var fields model.FieldList

	// A body written on the line the type opens on carries its methods between
	// the braces, separated by semicolons: "interface{ GetCode() int }".
	if from == to {
		code := strings.TrimSpace(src.codeLine(from))
		open := strings.Index(code, "{")
		close := strings.LastIndex(code, "}")
		if open < 0 || close <= open {
			return nil
		}
		for _, method := range splitTop(code[open+1:close], ';') {
			if field := interfaceMethod(method); field != nil {
				fields = append(fields, field)
			}
		}
		return fields
	}

	for i := from + 1; i < to; i++ {
		if field := interfaceMethod(strings.TrimSpace(src.codeLine(i))); field != nil {
			fields = append(fields, field)
		}
	}

	return fields
}

// interfaceMethod reads one element of an interface body, and is nil for a
// blank line.
func interfaceMethod(code string) *model.Field {
	if code == "" {
		return nil
	}

	open := strings.Index(code, "(")
	if open <= 0 {
		// An embedded interface, which the collector records as the one
		// unnamed field it stands for.
		return &model.Field{Type: "interface"}
	}

	name := strings.TrimSpace(code[:open])
	close := matchParen(code, open)
	if name == "" || close < 0 {
		return nil
	}

	params := paramGroups(code[open+1 : close])
	results := resultGroups(strings.TrimSpace(code[close+1:]))

	return &model.Field{
		Name: name,
		Type: signature(name, params, results),
	}
}

// splitField separates one struct field into its names, its type and its tag.
//
// The stripped view is what the split reads, since a tag is a string and a
// comment may follow the field; the tag itself comes from the raw line,
// the only place it survives.
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

	// A name group is one identifier or more, separated by commas and the
	// spaces around them: "file, dir any" declares two fields of one type. The
	// group ends at the first identifier no comma follows, so a type holding a
	// space, "chan int", stays whole. Anything else is a type with no name of
	// its own, "*T" and "pkg.T" included.
	head, rest, ok := nameGroup(code)
	if !ok || rest == "" {
		return "", code, tag
	}
	return head, rest, tag
}

// nameGroup splits a field line into its comma separated names and the type
// that follows them, and reports whether the line opens with a name at all.
func nameGroup(code string) (names, typ string, ok bool) {
	i, last := 0, 0
	for {
		start := i
		for i < len(code) && isIdentifierByte(code[i]) {
			i++
		}
		if i == start {
			return "", "", false
		}
		last = i

		// A name the line runs on from with anything but a separator opens an
		// embedded type, "pkg.T" or "List[int]", which declares no name.
		if i < len(code) && code[i] != ' ' && code[i] != '\t' && code[i] != ',' {
			return "", "", false
		}
		for i < len(code) && (code[i] == ' ' || code[i] == '\t') {
			i++
		}
		if i < len(code) && code[i] == ',' {
			i++
			for i < len(code) && (code[i] == ' ' || code[i] == '\t') {
				i++
			}
			continue
		}

		return code[:last], strings.TrimSpace(code[i:]), true
	}
}

// nameGroupLine reads a line that holds field names and a comma but no type
// yet, the first line of a group that runs to the next one.
func nameGroupLine(code string) ([]string, bool) {
	if !strings.HasSuffix(code, ",") {
		return nil, false
	}

	group := strings.TrimSpace(strings.TrimSuffix(code, ","))
	if group == "" {
		return nil, false
	}

	var names []string
	for _, part := range strings.Split(group, ",") {
		part = strings.TrimSpace(part)
		if !isIdentifier(part) {
			return nil, false
		}
		names = append(names, part)
	}
	return names, true
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
// field carries beside it, as opposed to above it.
func commentOn(comment string) string {
	text := strings.TrimSpace(comment)
	switch {
	case strings.HasPrefix(text, "//"):
		// A line comment is text up to the end of the line, so a "*/" inside
		// it belongs to what was written.
		return strings.TrimSpace(strings.TrimPrefix(text, "//"))
	case strings.HasPrefix(text, "/*"):
		text = strings.TrimPrefix(text, "/*")
		text = strings.TrimSuffix(text, "*/")
		return strings.TrimSpace(text)
	}
	return text
}

// fieldPath is the field named from the type down, the form the model
// records so a lookup finds a field without its declaration.
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
// nothing and the parser records it as such.
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

// nestedField reads a field whose type the source spells out in place, and returns
// the name it declares and every line of its type.
//
// The collector prints the type through go/printer, which writes it back the
// way the author wrote it, so the model records the declaration itself and no
// summary of it.
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
