package helpers

import "fmt"

// SecondsToDuration turns an int (seconds) into HH:MM:SS
func SecondsToDuration(input int) string {
    hours := 0
    minutes := 0
    seconds := input

    if seconds%60 != seconds {
        minutes = seconds / 60
        seconds %= 60
    }

    if minutes%60 != minutes {
        hours = minutes / 60
        minutes %= 60
    }

    return fmt.Sprintf("%02d:%02d:%02d", hours, minutes, seconds)
}
