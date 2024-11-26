package funcutils

import "fmt"

func PanicOrLogOnErr(f func() error, panicOnErr bool, msg string) {
	err := f()
	if err != nil {
		if panicOnErr {
			panic(fmt.Sprintf("%s: %s", msg, err))
		} else {
			fmt.Printf("%s: %s\n", msg, err.Error())
		}
	}
}

func IdentityFunc[T any](t T) func() T {
	return func() T {
		return t
	}
}
