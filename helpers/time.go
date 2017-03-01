package helpers

import (
	"fmt"
	"time"
	"strconv"
)

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

func Rev(s string) string {
	runes := []rune(s)
	for i, j := 0, len(runes)-1; i < j; i, j = i+1, j-1 {
		runes[i], runes[j] = runes[j], runes[i]
	}
	return string(runes)
}

func SinceInDaysText(timeThen time.Time) string {
	duration := time.Since(timeThen)
	if duration.Hours() >= 24 {
		return strconv.FormatFloat(duration.Hours()/24, 'f', 0, 64) + " days ago"
	} else {
		return "Less then a Day ago"
	}
}
