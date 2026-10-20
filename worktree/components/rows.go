package components

// RowHeight returns the tallest cell height across a row.
func (r Rows) RowHeight() int {
	h := 1
	for _, c := range r {
		if c.Height() > h {
			h = c.Height()
		}
	}
	return h
}
