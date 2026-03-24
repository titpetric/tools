package components

// Latest formats the latest git tag.
func Latest(tag string) Cell {
	if tag == "" {
		return nil
	}
	return Cell{ColorWhite + tag + ColorReset}
}
