package logger

import (
    "fmt"
    "time"
)

func (c LogLevel) L(src string, msg string) {
    fmt.Printf(
        colors[c].Color("[%s] (%-7s) [%s.go] %s\n"),
        time.Now().Format("15:04:05 02-01-2006"),
        nicenames[c],
        src,
        msg,
    )
}
