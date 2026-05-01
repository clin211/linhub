package shadow

import (
	"fmt"
	"strings"

	"golang.org/x/tools/go/analysis"
	"golang.org/x/tools/go/analysis/passes/shadow"
)

var (
	Analyzer         = shadow.Analyzer
	permittedShadows = []string{
		"ctx",
		"err",
		"pErr",
	}
)

func init() {
	oldRun := Analyzer.Run
	Analyzer.Run = func(p *analysis.Pass) (any, error) {
		pass := *p
		oldReport := p.Report
		pass.Report = func(diag analysis.Diagnostic) {
			for _, permittedShadow := range permittedShadows {
				if strings.HasPrefix(diag.Message, fmt.Sprintf("declaration of %q shadows declaration at line", permittedShadow)) {
					// 可以丢弃此次错误报告。
					return
				}
			}
			oldReport(diag)
		}
		return oldRun(&pass)
	}
}
