package helpers

import (
	"fmt"
	"math"
	"strconv"
	"strings"
	"time"
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
		return strconv.FormatFloat(math.Floor(duration.Hours()/24), 'f', 0, 64) + " days ago"
	} else {
		return "Less than a Day ago"
	}
}

// http://stackoverflow.com/a/36531443
func HumanizedTimesSince(a time.Time) (year, month, day, hour, min, sec int) {
	b := time.Now()
	if a.Location() != b.Location() {
		b = b.In(a.Location())
	}
	if a.After(b) {
		a, b = b, a
	}
	y1, M1, d1 := a.Date()
	y2, M2, d2 := b.Date()

	h1, m1, s1 := a.Clock()
	h2, m2, s2 := b.Clock()

	year = int(y2 - y1)
	month = int(M2 - M1)
	day = int(d2 - d1)
	hour = int(h2 - h1)
	min = int(m2 - m1)
	sec = int(s2 - s1)

	// Normalize negative values
	if sec < 0 {
		sec += 60
		min--
	}
	if min < 0 {
		min += 60
		hour--
	}
	if hour < 0 {
		hour += 24
		day--
	}
	if day < 0 {
		// days in month:
		t := time.Date(y1, M1, 32, 0, 0, 0, 0, time.UTC)
		day += 32 - t.Day()
		month--
	}
	if month < 0 {
		month += 12
		year--
	}

	return
}

func HumanizedTimesSinceText(since time.Time) string {
	sinceText := ""
	yearsSince, monthsSince, daysSince, hoursSince, minutesSince, secondsSince := HumanizedTimesSince(since)
	if yearsSince >= 0 || monthsSince >= 0 || daysSince >= 0 || hoursSince >= 0 || minutesSince >= 0 || secondsSince >= 0 {
		if yearsSince > 0 {
			sinceText += fmt.Sprintf(", %d years", yearsSince)
		}
		if monthsSince > 0 {
			sinceText += fmt.Sprintf(", %d months", monthsSince)
		}
		if daysSince > 0 {
			sinceText += fmt.Sprintf(", %d days", daysSince)
		}
		if hoursSince > 0 {
			sinceText += fmt.Sprintf(", %d hours", hoursSince)
		}
		if minutesSince > 0 {
			sinceText += fmt.Sprintf(", %d minutes", minutesSince)
		}
		if secondsSince > 0 {
			sinceText += fmt.Sprintf(", %d seconds", secondsSince)
		}
		sinceText = strings.Replace(sinceText, ", ", "", 1)
		sinceText = Rev(strings.Replace(Rev(sinceText), Rev(", "), Rev(" and "), 1))
	}
	return sinceText
}

func HumanizeDuration(d time.Duration) (result string) {
	hours := int(d.Hours())
	minutes := int(d.Minutes()) - (hours * 60)
	seconds := int(d.Seconds()) - (minutes * 60) - (hours * 60 * 60)

	if hours > 0 {
		days := hours / 24
		hoursLeft := hours % 24
		if days > 0 {
			result += strconv.Itoa(days) + "d"
		}
		if hoursLeft > 0 {
			result += strconv.Itoa(hoursLeft) + "h"
		}
	}
	if minutes > 0 {
		result += strconv.Itoa(minutes) + "m"
	}
	if seconds > 0 {
		result += strconv.Itoa(seconds) + "s"
	}
	return result
}
