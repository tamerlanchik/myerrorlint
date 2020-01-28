package myerrorlint

import (
	"fmt"
	//"go/token"

	"golang.org/x/tools/go/analysis"
	"golang.org/x/tools/go/analysis/passes/buildssa"
	"golang.org/x/tools/go/ssa"
)

const Doc = `check for errors of wrong type returned from our functions`

var Analyzer = &analysis.Analyzer{
	Name:     "myerrorlint",
	Doc:      Doc,
	Requires: []*analysis.Analyzer{buildssa.Analyzer},
	Run:      run,
}

func run(pass *analysis.Pass) (interface{}, error) {
	ssainput := pass.ResultOf[buildssa.Analyzer].(*buildssa.SSA)
	for _, fn := range ssainput.SrcFuncs {
		runFunc(pass, fn)
	}
	return nil, nil
}

func runFunc(pass *analysis.Pass, fn *ssa.Function) {
	//reportf := func(category string, pos token.Pos, format string, args ...interface{}) {
	//	pass.Report(analysis.Diagnostic{
	//		Pos:      pos,
	//		Category: category,
	//		Message:  fmt.Sprintf(format, args...),
	//	})
	//}

	seen := make([]bool, len(fn.Blocks)) // seen[i] means visit should ignore block i
	var visit func(b *ssa.BasicBlock, stack []fact)
	visit = func(b *ssa.BasicBlock, stack []fact) {
		if seen[b.Index] {
			return
		}
		seen[b.Index] = true
		fmt.Printf("BasicBlock: %+v", b)

		for _, d := range b.Dominees() {
			visit(d, stack)
		}
	}

	// Visit the entry block.  No need to visit fn.Recover.
	if fn.Blocks != nil {
		visit(fn.Blocks[0], make([]fact, 0, 20))
	}
}

type fact struct {
	value  ssa.Value
	ourErr bool
}
