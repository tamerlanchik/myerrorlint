// external tests for myerrorlint pkg
package myerrorlint_test

import (
	"testing"

	linter "github.com/Rikkuru/myerrorlint"
	"golang.org/x/tools/go/analysis/analysistest"
)

func Test(t *testing.T) {
	testdata := analysistest.TestData()
	analizer := linter.NewAnalyzer(linter.Config{
		AllowedTypes:              []string{"a.myError"},
		OurPackages:               []string{"a"},
		ReportUnknown:             true,
		AllowErrorfWrap:           true,
		WrapFuncWithFirstArgError: []string{"a.Wrap"}})
	analysistest.Run(t, testdata, analizer, "a")
}
