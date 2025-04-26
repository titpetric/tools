package main

import (
	"github.com/titpetric/tools/gofsck/pkg/gofsck"
	"golang.org/x/tools/go/analysis"
)

func New(conf any) ([]*analysis.Analyzer, error) {
	check := gofsck.NewAnalyzer()
	return []*analysis.Analyzer{check}, nil
}
