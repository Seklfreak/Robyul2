package logger

import "fmt"

// INF logs with [INF] prefix
func INF(s string) {
    fmt.Printf("[INF] %s\n", s)
}

// WRN logs with [WRN] prefix
func WRN(s string) {
    fmt.Printf("[WRN] %s\n", s)
}

// ERR logs with [ERR] prefix
func ERR(s string) {
    fmt.Printf("[ERR] %s\n", s)
}
