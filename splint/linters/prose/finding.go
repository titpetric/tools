package prose

// Finding is one construction a tree leans on, beside the baseline's use of
// the same shape.
type Finding struct {
	// Skeleton is the shape. Phrase is its first instance, at File and Line.
	Skeleton string
	Phrase   string
	File     string
	Line     int

	// Count and Baseline are the two corpora's tallies.
	Count    int
	Baseline int

	// Ratio divides the tree's rate by the baseline's. Absent marks a shape
	// the baseline never used, where that division has no answer.
	Ratio  float64
	Absent bool

	// Score weighs the gap against the prose behind it. Six uses in twenty
	// thousand words is a high rate on thin evidence. Thirty is the same rate
	// and settles it.
	Score float64
}
