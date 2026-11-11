package prose

// Limits is the floor a finding clears to reach the report.
type Limits struct {
	// Count is the tally a shape needs. Once is an accident.
	Count int

	// Ratio is the margin over the baseline's rate.
	Ratio float64

	// Score is the evidence. Corpus work draws the line at 15.13: a gap
	// scoring higher turns up by chance once in ten thousand comparisons.
	Score float64
}
