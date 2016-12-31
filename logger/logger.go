package logger

import "fmt"

// Log with [INF] prefix
func INF(s string) {
    fmt.Printf("[INF] %s\n", s)
}

// Log with [WRN] prefix
func WRN(s string) {
    fmt.Printf("[WRN] %s\n", s)
}

// Log with [ERR] prefix
func ERR(s string) {
    fmt.Printf("[ERR] %s\n", s)
}
