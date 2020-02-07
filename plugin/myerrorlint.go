package main

import (
	linter "github.com/Rikkuru/myerrorlint"
	"golang.org/x/tools/go/analysis"
)

type analyzerPlugin struct{}

// This must be implemented
func (*analyzerPlugin) GetAnalyzers() []*analysis.Analyzer {
	return []*analysis.Analyzer{
		linter.NewAnalyzer(linter.Config{OurPackages: []string{}, AllowedTypes: []string{}, ReportUnknown: true}),
	}
}

// This must be defined and named 'AnalyzerPlugin'
var AnalyzerPlugin analyzerPlugin
