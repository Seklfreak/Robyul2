package logger

import "fmt"

func INF(s string) {
	fmt.Printf("[INF] %s\n", s)
}

func WRN(s string) {
	fmt.Printf("[WRN] %s\n", s)
}

func ERR(s string) {
	fmt.Printf("[ERR] %s\n", s)
}
