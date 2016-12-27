// Except.go: Contains functions to make handling panics less PITA

package helpers

import "reflect"

type Callback func()

// Softer form of relax
// Calls a callback instead of panicking
func SoftRelax(err error, cb Callback) {
    if err != nil {
        cb()
    }
}

// Helper to reduce if-checks if panicking is allowed
// If $err is nil this is a no-op. Panics otherwise.
func Relax(err error) {
    if err != nil {
        panic(err)
    }
}

// if a != b throw err
func RelaxAssertEqual(a interface{}, b interface{}, err error) {
    if !reflect.DeepEqual(a, b) {
        Relax(err)
    }
}

// if a == b throw err
func RelaxAssertUnequal(a interface{}, b interface{}, err error) {
    if reflect.DeepEqual(a, b) {
        Relax(err)
    }
}
