package funcutils

import (
	"fmt"
	"strings"

	"github.com/samber/lo"
)

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

// MultiError turn the errors into a single error if they are errors.
func MultiError(errs ...error) error {
	errStrings := lo.FilterMap(errs, func(err error, _ int) (string, bool) {
		if err == nil {
			return "", false
		}
		return fmt.Sprintf("- %q", err.Error()), true
	})
	if len(errStrings) == 0 {
		return nil
	}
	msg := strings.Join(errStrings, "\n")
	return fmt.Errorf("received the following errors:\n%s", msg)
}

func MultiErrFunc(funcs ...func() error) error {
	return MultiError(lo.Map(funcs, func(f func() error, _ int) error {
		return f()
	})...)
}
