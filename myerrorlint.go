package myerrorlint

import (
	"fmt"
	"go/constant"
	"go/token"
	"go/types"
	"strings"

	"golang.org/x/tools/go/analysis"
	"golang.org/x/tools/go/analysis/passes/buildssa"
	"golang.org/x/tools/go/ssa"
)

const Doc = `check for errors of wrong type returned from our functions (allowed type defined in cfg)
Unknown cases:
	- Error from map, struct, slice - whould have to check all actions on that object,
	also we dont check fields of objects in return values so we would not be able to assume
	that our functions return correct objects. Use objects with allowed types instead of objects with error interface`
const Name = "myerrorlint"

// TODO: if some of func can return external errors (for example Unwrap of our error) they can be ignorred but their return values should not be returned by other functions
// TODO: check if func is a wrap function by its comments - need to somehow get function declaration tags for that

type Config struct {
	AllowedTypes              []string // if no type then only check that we return errors from our pkgs
	OurPackages               []string // linter assumes that functions from our packages return allowed errors. If run linter on all our packages it will be true
	ReportUnknown             bool     // report error if unknown case (error from map and such)
	AllowErrorfWrap           bool     // check for fmt.Errorf wrapped error
	WrapFuncWithFirstArgError []string // Wrap functions that take error as first param (like github.com/pkg/errors.Wrap)
}

func NewAnalyzerWithoutRun() *analysis.Analyzer {
	return &analysis.Analyzer{
		Name:     Name,
		Doc:      Doc,
		Requires: []*analysis.Analyzer{buildssa.Analyzer},
		//Run should be filled letter
	}
}

func NewAnalyzer(cfg Config) *analysis.Analyzer {
	return &analysis.Analyzer{
		Name:     Name,
		Doc:      Doc,
		Requires: []*analysis.Analyzer{buildssa.Analyzer},
		Run:      NewRun(cfg),
	}
}

// will use cfg later
func NewRun(cfg Config) func(pass *analysis.Pass) (interface{}, error) {
	return func(pass *analysis.Pass) (interface{}, error) {
		ssainput := pass.ResultOf[buildssa.Analyzer].(*buildssa.SSA)
		for _, fn := range ssainput.SrcFuncs {
			runFunc(pass, fn, &cfg)
		}
		return nil, nil
	}
}

var (
	errorType      types.Type
	errorInterface *types.Interface
)

func init() {
	errorType = types.Universe.Lookup("error").Type()
	errorInterface = errorType.Underlying().(*types.Interface)

}

func isErrorType(t types.Type) bool {
	return types.Implements(t, errorInterface)
}

// errorsBySignature return s indices of error type return variables
func errorsBySignature(sign *types.Signature) []int {
	results := sign.Results()
	var res []int = make([]int, 0)
	for i := 0; i < results.Len(); i++ {
		if isErrorType(results.At(i).Type()) {
			res = append(res, i)
		}
	}
	return res
}

func reportf(pass *analysis.Pass, pos token.Pos, format string, args ...interface{}) {
	pass.Report(analysis.Diagnostic{
		Pos:     pos,
		Message: fmt.Sprintf(format, args...),
	})
}

func isOurPkg(pkgName string, cfg *Config) bool {
	for _, ourPkgStr := range cfg.OurPackages {
		strLen := len(ourPkgStr)
		if ourPkgStr[strLen-1:strLen] == "/" {
			// dir of pkgs
			if strings.HasPrefix(pkgName, ourPkgStr) {
				return true
			}
		} else if ourPkgStr == pkgName {
			return true
		}
	}
	return false
}
func isAllowedErrorType(t types.Type, cfg *Config) bool {
	for _, allowedType := range cfg.AllowedTypes {
		if t.String() == allowedType {
			return true
		}
	}
	return false
}

func isWrapCall(call *ssa.CallCommon, cfg *Config) (isWrap bool, v ssa.Value) {
	function := call.StaticCallee()
	args := call.Args
	if cfg.AllowErrorfWrap && function.Name() == "Errorf" && function.Pkg.Pkg.Path() == "fmt" {
		// check if Errorf wraps error
		if len(args) != 2 {
			return false, nil
		}
		if fmtSlice, ok := args[1].(*ssa.Slice); ok {
			// has IndexAddr for every arg passed to ...interface{}
			for _, fmtPointer := range *fmtSlice.X.Referrers() {
				if idxAddr, ok := fmtPointer.(*ssa.IndexAddr); ok {
					// has command that stores interface to idxAddr
					if storeInstr, ok := (*idxAddr.Referrers())[0].(*ssa.Store); ok {
						if makeInterface, ok := storeInstr.Val.(*ssa.MakeInterface); ok {
							// finaly check arg is error
							if isErrorType(makeInterface.X.Type()) {
								return true, makeInterface.X
							}
						}
					}
				}
			}
		}
	}
	for _, allowedFunc := range cfg.WrapFuncWithFirstArgError {
		fullName := function.Pkg.Pkg.Path() + "." + function.Name()
		if allowedFunc == fullName {
			// wraps first param
			if len(args) > 1 {
				return true, args[0]
			}

		}
	}
	return false, nil
}

func retPos(v interface{ Pos() token.Pos }, defaultPos token.Pos) token.Pos {
	if v.Pos() == token.NoPos {
		return defaultPos
	}
	return v.Pos()
}

func checkCallInstruction(pass *analysis.Pass, v ssa.CallInstruction, cfg *Config, defaultPos token.Pos, seen map[ssa.Value]bool) {
	//https://godoc.org/golang.org/x/tools/go/ssa#CallCommon
	commonCall := v.Common()
	if commonCall.IsInvoke() {
		//call to interface method
		pkgName := commonCall.Method.Pkg().Path()
		if isOurPkg(pkgName, cfg) {
			return
		}
		reportf(pass, retPos(v, defaultPos), "error not from our pkg: %s", pkgName)
		return
	}
	function := commonCall.StaticCallee()
	if function != nil {
		if ok, wrappedErr := isWrapCall(commonCall, cfg); ok {
			// check that wrapped error is allowed
			allowedValue(pass, wrappedErr, cfg, retPos(v, defaultPos), seen)
			return
		} //else {
		//reportf(pass, retPos(v, defaultPos), "not wrap: %v", commonCall.StaticCallee().Pkg.Pkg.Path() +  )
		//}
		// (a) statically dispatched call to a package-level function, an anonymous function, or a method of a named type
		// (b) immediately applied function literal with free variables
		pkgName := function.Pkg.Pkg.Path()
		if isOurPkg(pkgName, cfg) {
			return
		}
		reportf(pass, retPos(v, defaultPos), "error not from our pkg: %s", pkgName)
		return
	}
	if blt, ok := commonCall.Value.(*ssa.Builtin); ok {
		reportf(pass, retPos(v, defaultPos), "error not from our pkg: builtin %s", blt.Name())
		return
	}
	// (d) any other value, indicating a dynamically dispatched function call.
	// not supported - we cant even check pkg for it
	reportf(pass, retPos(v, defaultPos), "dynamically dispatched function call: %v", commonCall)
}

// check if error value is allowed
// if error is returned if value is unsupperted as of yet
// defaultPos - pos to return in case value has no pos (const)
func allowedValue(pass *analysis.Pass, v ssa.Value, cfg *Config, defaultPos token.Pos, seen map[ssa.Value]bool) {
	if seen[v] {
		return
	}
	seen[v] = true
	if v.Type() == errorType {
		// "error" type
		// follow from where we got that interface
		// - from our function call - ok
		// - from external function call - not ok
		// - MakeInterface from allowed type - ok
		// - MakeInterface from disallowed type - not ok
		// - nill  - ok
		// - from map/slice/struct - not ok
		// - from global - not ok
		switch v := v.(type) {
		case *ssa.MakeInterface: // var err error = sometype{}
			allowedValue(pass, v.X, cfg, retPos(v, defaultPos), seen)
		case *ssa.ChangeType:
			allowedValue(pass, v.X, cfg, retPos(v, defaultPos), seen)
		case *ssa.Phi: // alternatives
			for _, altV := range v.Edges {
				allowedValue(pass, altV, cfg, retPos(v, defaultPos), seen)
			}
		case ssa.CallInstruction:
			checkCallInstruction(pass, v, cfg, defaultPos, seen)
		case *ssa.Extract:
			switch tuple := v.Tuple.(type) {
			case ssa.CallInstruction:
				checkCallInstruction(pass, tuple, cfg, defaultPos, seen)
				return
			default:
				if cfg.ReportUnknown {
					reportf(pass, retPos(v, defaultPos), "[warn] unsupported case for extract value=%#v", v)
				}
			}
		case *ssa.Lookup: // err = somemap[key]
			// cant check all errors in map (especially for global var)
			reportf(pass, retPos(v, defaultPos), "not our type error in map lookup: %s", v.Type().String())
		case *ssa.UnOp:
			if v.Op == token.MUL {
				switch xValue := v.X.(type) {
				case *ssa.Global:
					// use of global var
					reportf(pass, retPos(v, defaultPos), "cant check error type for global: %s", xValue.Name())
				case *ssa.Alloc:
					for _, instr := range *xValue.Referrers() {
						if store, ok := instr.(*ssa.Store); ok {
							allowedValue(pass, store.Val, cfg, retPos(store, defaultPos), seen)
						}
					}
				case *ssa.FieldAddr:
					reportf(pass, retPos(v, defaultPos), "cant check error type for struct field")
				case *ssa.IndexAddr:
					reportf(pass, retPos(v, defaultPos), "cant check error type for slice element")
				default:
					reportf(pass, retPos(v, defaultPos), "[warn] unsupported case for error from UnOp with value=%#v", xValue)
				}
				return
			}
			if cfg.ReportUnknown {
				reportf(pass, retPos(v, defaultPos), "[warn] unsupported case for error value=%#v", v)
			}
		case *ssa.Const:
			if v.Value == constant.Value(nil) {
				//nill error interface
				return
			}
			reportf(pass, retPos(v, defaultPos), "[warn] unsupported case for error const=%#v", v)
		case *ssa.Parameter:
			reportf(pass, retPos(v, defaultPos), "cant check error type for %v", v)
		default:
			//ssa.Field - unsupported - would not be able to check it if it has X=*ssa.Call (error from struct returned by other func)
			if cfg.ReportUnknown {
				reportf(pass, retPos(v, defaultPos), "[warn] unsupported case for error value=%#v", v)
			}
		}
		return
	}
	if isAllowedErrorType(v.Type(), cfg) {
		return
	}
	reportf(pass, retPos(v, defaultPos), "not our type error: %s", v.Type().String())
}

func runFunc(pass *analysis.Pass, fn *ssa.Function, cfg *Config) {
	errorsAtReturn := errorsBySignature(fn.Signature)
	if len(errorsAtReturn) == 0 {
		// function doen not return error
		// will not check it
		return
	}

	seen := make([]bool, len(fn.Blocks)) // seen[i] means visit should ignore block i
	var visit func(b *ssa.BasicBlock)
	visit = func(b *ssa.BasicBlock) {
		if seen[b.Index] {
			return
		}
		seen[b.Index] = true
		for _, instr := range b.Instrs {
			if retInstr, ok := instr.(*ssa.Return); ok {
				operands := retInstr.Operands([]*ssa.Value(nil))
				for _, i := range errorsAtReturn {
					value := operands[i]
					seenValue := make(map[ssa.Value]bool)
					allowedValue(pass, *value, cfg, retInstr.Pos(), seenValue)
				}
			}
		}

		for _, d := range b.Dominees() {
			visit(d)
		}
	}

	// Visit the entry block.  No need to visit fn.Recover.
	if fn.Blocks != nil {
		visit(fn.Blocks[0])
	}
}
