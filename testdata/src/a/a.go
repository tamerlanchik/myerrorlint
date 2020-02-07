package a

import (
	"b"
	"fmt"
)

// our error type
type myError string

func (myError) Error() string {
	return "123"
}

type notMyError struct{}

func (*notMyError) Error() string {
	return "123"
}

// from MakeInterface
func fWithCorrectType() (error, string) {
	return myError(""), "teststring"
}

// from MakeInterface
func fWithIncorrectType() error {
	return &notMyError{} //want `not our type error: \*a.notMyError`
}

// retrun type that implements error
func fWithCorrectType2() myError {
	return myError("")
}

// retrun type that implements error
func fWithIncorrectType2() *notMyError {
	return &notMyError{} //want `not our type error: \*a.notMyError`
}

// define interface and its implementation to test ssa.Call for interface method
type myInterface interface {
	GetSomeError() error
}

type myInterfaceImpl struct{}

func (o *myInterfaceImpl) GetSomeError() error {
	return myError("")
}

func getMyInterface() myInterface {
	return &myInterfaceImpl{}
}

// end of interface implementation

// from ssa.Call with Method
func fWithCorrectTypeFromInterfaceMethod() error {
	return b.GetMyInterface().GetSomeError() // want "error not from our pkg: b"
}

// from ssa.Call with Method
func fWithInorrectTypeFromInterfaceMethod() error {
	return getMyInterface().GetSomeError()
}

// from ssa.Call with StaticCallee
// fWithIncorrectType is in our pkg so we assume it returns correct error
func fWithCorrectTypeFromCall() error {
	return fWithIncorrectType()
}

// from ssa.Call with StaticCallee
func fWithInorrectTypeFromCall() error {
	return b.F() // want "error not from our pkg: b"
}

// from ssa.Call with StaticCallee
func fWithCorrectTypeFromCall2() error {
	errF := func() error {
		return myError("")
	}
	return errF()
}

// from ssa.Extract from Call
func fWithCorrectTypeFromExtractFromCall() error {
	err, _ := fWithCorrectType()
	return err
}

// from ssa.Extract from Call
func fWithIncorrectTypeFromExtractFromCall() error {
	err, _ := b.G() // want "error not from our pkg: b"
	return err
}

var mapOfOurErrors = map[int]myError{
	1: myError(""),
}

var mapOfErrors = map[int]error{
	1: myError(""),
}

// dont check error interfaces in map
func fWithIncorrectTypeFromMap() error {
	myMap := map[int]error{1: myError("")}
	return myMap[2] // want "not our type error in map lookup: error"
}

func fWithCorrectTypeFromMap() error {
	myMap := map[int]myError{1: myError("")}
	return myMap[2]
}

// from global map
func fWithCorrectTypeFromMap2() error {
	return mapOfOurErrors[1]
}

// from global map
func fWithIncorrectTypeFromMap2() error {
	return mapOfErrors[1] // want "not our type error in map lookup: error"
}

var globError error = myError("")
var globOurError myError = myError("")

func fWithCorrectGlobError() error {
	return globOurError
}

// cant check were glogal error is from
// in our pkg globals should have our type
func fWithIncorrectGlobError() error {
	return globError // want "cant check error type for global: globError"
}

const (
	ourConstError = myError("")
)

func fWithCorrectConstError() error {
	return ourConstError
}

func fWithInorrectConstError() error {
	return b.ConstError // want "not our type error: b.myConstError"
}

// wrap function saves original error
func fWithCorrectWrappedError() error {
	return fmt.Errorf("err: %s, %w, %d", "str", myError(""), 11)
}

var dynFuncFromOtherPkg = b.FunctionFromOtherPkg

func fWithIncorrectDynCall() error {
	return dynFuncFromOtherPkg() // want `dynamically dispatched function call: t0\(\)`
}

func fWithNilError() error {
	return nil
}

func fWithErrorFromParams(err error) error { // want "cant check error type for parameter err : error"
	if err != nil {
		return err
	}
	return nil
}

func fWithOurErrorFromParams(err myError) error {
	return err
}

func fErrorFromStruct() error {
	s := struct{ err error }{err: myError("")}
	return s.err // want "cant check error type for struct field"
}

var globErrorStruct = struct{ err error }{err: myError("")}

func fErrorFromGlobStruct() error {
	return globErrorStruct.err // want "cant check error type for struct field"
}

func fCorrectErrorFromStruct() error {
	s := struct{ err myError }{err: myError("")}
	return s.err
}

func fErrorFromSlice() error {
	s := []error{myError("")}
	return s[0] // want "cant check error type for slice element"
}

func fOurErrorFromSlice() error {
	s := []myError{myError("")}
	return s[0]
}
