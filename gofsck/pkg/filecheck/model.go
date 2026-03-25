package filecheck

// Report contains file size statistics results.
type Report struct {
	Scanned []ScannedGroup `json:"scanned"`
	Rating  float64        `json:"rating"`
}

// ScannedGroup holds statistics for a single file extension.
type ScannedGroup struct {
	Ext       string  `json:"ext"`
	Files     int     `json:"files"`
	Histogram []int   `json:"histogram"`
	Score     float64 `json:"score"`
}
