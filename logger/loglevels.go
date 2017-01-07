package logger

import (
    "github.com/ttacon/chalk"
)

type LogLevel int

const (
    ERROR LogLevel = iota
    WARNING
    PLUGIN
    INFO
    VERBOSE
)

var nicenames = map[LogLevel]string{
    ERROR:   "ERROR",
    WARNING: "WARNING",
    PLUGIN:  "PLUGIN",
    INFO:    "INFO",
    VERBOSE: "VERBOSE",
}

var colors = map[LogLevel]chalk.Color{
    ERROR:   chalk.Red,
    WARNING: chalk.Yellow,
    PLUGIN:  chalk.Cyan,
    INFO:    chalk.Green,
    VERBOSE: chalk.White,
}
