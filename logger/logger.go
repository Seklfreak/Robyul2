package logger

import (
    "fmt"
    "time"
)

var (
    DEBUG_MODE = false
)

func (c LogLevel) L(src string, msg string) {
    if c == DEBUG && !DEBUG_MODE {
        return
    }

    fmt.Printf(
        colors[c].Color("[%s] (%-7s) [%s.go] %s\n"),
        time.Now().Format("15:04:05 02-01-2006"),
        nicenames[c],
        src,
        msg,
    )
}
