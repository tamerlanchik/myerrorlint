// external package for test
package b

// our error type
type someError struct{}

func (*someError) Error() string {
	return "123"
}

// define intercafe and its implementation to test ssa.Call for interface method from external pkg
type myInterface interface {
	GetSomeError() error
}

type myInterfaceImpl struct{}

func (o *myInterfaceImpl) GetSomeError() error {
	return &someError{}
}

// from ssa.Call with Method
func GetMyInterface() myInterface {
	return &myInterfaceImpl{}
}

func F() error {
	return &someError{}
}

func G() (error, string) {
	return &someError{}, "test"
}

type myConstError string

func (myConstError) Error() string {
	return "123"
}

const (
	ConstError = myConstError("")
)

func FunctionFromOtherPkg() error {
	return &someError{}
}
