package errcheck

import (
	_ "embed"

	"github.com/kisielk/errcheck/errcheck"
)

var Analyzer = errcheck.Analyzer

// go:embed errcheck_excludes.txt
var excludesContent string

func init() {
	if err := Analyzer.Flags.Set("excludes", excludesContent); err != nil {
		panic(err)
	}
}
