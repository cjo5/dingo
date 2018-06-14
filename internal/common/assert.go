package common

import "fmt"

// Assert expr is true, otherwise panic.
func Assert(expr bool, format string, args ...interface{}) {
	if !expr {
		panic(fmt.Sprintf(format, args...))
	}
}
