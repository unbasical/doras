package funcutils

import (
	"fmt"
)

// PanicOrLogOnErr does what its name suggests.
func PanicOrLogOnErr(f func() error, panicOnErr bool, msg string) {
	if err := f(); err != nil {
		if panicOnErr {
			panic(fmt.Sprintf("%s: %s", msg, err))
		}
		fmt.Printf("%s: %s\n", msg, err.Error())
	}
}

// IdentityFunc returns the input.
func IdentityFunc[T any](t T) func() T {
	return func() T {
		return t
	}
}
