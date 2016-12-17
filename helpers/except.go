// Except.go: Contains functions to make handling panics less PITA

package helpers

// Helper to reduce if-checks if panicking is allowed
// If $err is nil this is a no-op. Panics otherwise.
func Relax(err error) {
    if err != nil {
        panic(err)
    }
}
