package main

import (
	"golang.org/x/tools/go/analysis/singlechecker"

	"github.com/titpetric/tools/gofsck/pkg/gofsck"
)

func main() {
	singlechecker.Main(gofsck.NewAnalyzer())
}
