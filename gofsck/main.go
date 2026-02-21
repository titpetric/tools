package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"

	"golang.org/x/tools/go/analysis"
	"golang.org/x/tools/go/analysis/singlechecker"
	"golang.org/x/tools/go/packages"

	"github.com/titpetric/tools/gofsck/model"
	"github.com/titpetric/tools/gofsck/pkg/coverage"
	"github.com/titpetric/tools/gofsck/pkg/grouping"
	"github.com/titpetric/tools/gofsck/pkg/pairing"
	"github.com/titpetric/tools/gofsck/pkg/wraphandler"
)

var (
	outputFile = flag.String("output", "", "Write report to file (empty = stdout)")
	format     = flag.String("format", "text", "Output format: text or json")
	useChecker = flag.Bool("checker", false, "Use singlechecker mode (for linter integration)")
)

func main() {
	flag.Parse()

	// If using checker mode, run with singlechecker (runs all analyzers)
	if *useChecker {
		analyzers := New()
		// singlechecker expects a single analyzer, so run the first one (grouping)
		// In a real scenario, you'd want to compose them differently
		singlechecker.Main(analyzers[2]) // grouping is the linter analyzer
		return
	}

	pkgPaths := flag.Args()
	if len(pkgPaths) == 0 {
		pkgPaths = []string{"."}
	}

	// Load packages
	cfg := &packages.Config{
		Mode:  packages.NeedName | packages.NeedFiles | packages.NeedSyntax | packages.NeedImports | packages.NeedTypes | packages.NeedTypesSizes,
		Tests: true,
	}

	pkgs, err := packages.Load(cfg, pkgPaths...)
	if err != nil {
		log.Fatalf("failed to load packages: %s", err)
	}

	_ = &model.Config{} // config is reserved for future use

	// Run all analyzers and collect reports
	report := NewReport(pkgs)

	// Output results
	var output string
	if *format == "json" {
		data, err := json.MarshalIndent(report, "", "  ")
		if err != nil {
			log.Fatalf("failed to marshal report: %s", err)
		}
		output = string(data)
	} else {
		output = formatTextReport(report)
	}

	// Write to file or stdout
	if *outputFile != "" {
		err := os.WriteFile(*outputFile, []byte(output), 0644)
		if err != nil {
			log.Fatalf("failed to write output file: %s", err)
		}
		fmt.Printf("Report written to %s\n", *outputFile)
	} else {
		fmt.Println(output)
	}
}

// New returns all three analyzers as analysis.Analyzer types for singlechecker compatibility.
// Order: pairing, coverage, grouping
func New() []*analysis.Analyzer {
	return []*analysis.Analyzer{
		newPairingAnalyzer(),
		newCoverageAnalyzer(),
		grouping.NewAnalyzer(),
	}
}

// newPairingAnalyzer wraps the pairing analyzer as an analysis.Analyzer
func newPairingAnalyzer() *analysis.Analyzer {
	return &analysis.Analyzer{
		Name: "gofsck-pairing",
		Doc:  "checks file-test pairing to ensure implementation files have corresponding test files",
		Run:  runPairingAnalyzer,
	}
}

// newCoverageAnalyzer wraps the coverage analyzer as an analysis.Analyzer
func newCoverageAnalyzer() *analysis.Analyzer {
	return &analysis.Analyzer{
		Name: "gofsck-coverage",
		Doc:  "checks symbol-test coverage to identify untested exported symbols",
		Run:  runCoverageAnalyzer,
	}
}

// runPairingAnalyzer is a no-op for singlechecker (pairing needs packages.Package data)
func runPairingAnalyzer(pass *analysis.Pass) (interface{}, error) {
	// Pairing analysis requires package-level data, not AST-based analysis
	return nil, nil
}

// runCoverageAnalyzer is a no-op for singlechecker (coverage needs packages.Package data)
func runCoverageAnalyzer(pass *analysis.Pass) (interface{}, error) {
	// Coverage analysis requires package-level data, not AST-based analysis
	return nil, nil
}

// NewReport creates an aggregated report by running all analyzers on the provided packages.
func NewReport(pkgs []*packages.Package) *model.AggregatedReport {
	report := &model.AggregatedReport{
		Analyzers: make([]*model.AnalyzerReport, 0),
		Errors:    make([]string, 0),
	}

	// Run pairing analyzer
	pairingAnalyzer := pairing.New()
	pairingResult, err := pairingAnalyzer.Analyze(pkgs)
	if err != nil {
		report.Errors = append(report.Errors, fmt.Sprintf("pairing: %v", err))
	} else {
		report.Analyzers = append(report.Analyzers, &model.AnalyzerReport{
			Name: "pairing",
			Type: "pairing",
			Data: pairingResult,
		})
	}

	// Run coverage analyzer
	coverageAnalyzer := coverage.New()
	coverageResult, err := coverageAnalyzer.Analyze(pkgs)
	if err != nil {
		report.Errors = append(report.Errors, fmt.Sprintf("coverage: %v", err))
	} else {
		report.Analyzers = append(report.Analyzers, &model.AnalyzerReport{
			Name: "coverage",
			Type: "coverage",
			Data: coverageResult,
		})
	}

	// Run wraphandler analyzer
	wraphandlerAnalyzer := wraphandler.New()
	wraphandlerResult, err := wraphandlerAnalyzer.Analyze(pkgs)
	if err != nil {
		report.Errors = append(report.Errors, fmt.Sprintf("wraphandler: %v", err))
	} else {
		report.Analyzers = append(report.Analyzers, &model.AnalyzerReport{
			Name: "wraphandler",
			Type: "wraphandler",
			Data: wraphandlerResult,
		})
	}

	// Run grouping analyzer
	groupingAnalyzer := grouping.New()
	groupingResult, err := groupingAnalyzer.Analyze(pkgs)
	if err != nil {
		report.Errors = append(report.Errors, fmt.Sprintf("grouping: %v", err))
	} else {
		report.Analyzers = append(report.Analyzers, &model.AnalyzerReport{
			Name: "grouping",
			Type: "grouping",
			Data: groupingResult,
		})
	}

	return report
}

// formatTextReport formats the aggregated report as human-readable text.
func formatTextReport(report *model.AggregatedReport) string {
	var output string

	for _, analyzer := range report.Analyzers {
		output += fmt.Sprintf("\n=== %s ===\n", analyzer.Name)

		switch analyzer.Name {
		case "pairing":
			if pr, ok := analyzer.Data.(*pairing.Report); ok {
				output += formatPairingReport(pr)
			}
		case "coverage":
			if cr, ok := analyzer.Data.(*coverage.Report); ok {
				output += formatCoverageReport(cr)
			}
		case "wraphandler":
			if wr, ok := analyzer.Data.(*wraphandler.Report); ok {
				output += formatWraphandlerReport(wr)
			}
		case "grouping":
			if gr, ok := analyzer.Data.(*grouping.Report); ok {
				output += formatGroupingReport(gr)
			}
		}
	}

	if len(report.Errors) > 0 {
		output += "\n=== Errors ===\n"
		for _, err := range report.Errors {
			output += fmt.Sprintf("- %s\n", err)
		}
	}

	return output
}

func formatPairingReport(pr *pairing.Report) string {
	return fmt.Sprintf(`Files:            %d
Tests:            %d
Paired:           %d
Standalone Files: %d
Standalone Tests: %d
`, pr.Files, pr.Tests, pr.Paired, pr.StandaloneFiles, pr.StandaloneTests)
}

func formatCoverageReport(cr *coverage.Report) string {
	output := fmt.Sprintf("Coverage Ratio:    %.2f%%\n", cr.CoverageRatio*100)
	output += fmt.Sprintf("Covered Symbols:   %d\n", len(cr.Symbols))
	output += fmt.Sprintf("Uncovered Symbols: %d\n", len(cr.Uncovered))
	output += fmt.Sprintf("Standalone Tests:  %d\n", len(cr.StandaloneTests))

	if len(cr.Uncovered) > 0 {
		output += "\nUncovered symbols:\n"
		for _, sym := range cr.Uncovered {
			output += fmt.Sprintf("  - %s\n", sym)
		}
	}

	return output
}

func formatWraphandlerReport(wr *wraphandler.Report) string {
	output := fmt.Sprintf("Handlers Passing: %d/%d\n", wr.Passing, wr.Total)

	if len(wr.Violations) > 0 {
		output += "\nViolations:\n"
		for _, v := range wr.Violations {
			output += fmt.Sprintf("  - %s:%d: %s\n", v.File, v.Line, v.Message)
		}
	}

	return output
}

func formatGroupingReport(gr *grouping.Report) string {
	output := fmt.Sprintf("Symbols:    %d\nPassing:    %d\nViolations: %d\n", gr.Total, gr.Passing, len(gr.Violations))

	if len(gr.Violations) > 0 {
		output += "\nViolations:\n"
		for _, v := range gr.Violations {
			output += fmt.Sprintf("  - %s:%d:%d: %s\n", v.File, v.Line, v.Column, v.Message)
		}
	}

	return output
}
