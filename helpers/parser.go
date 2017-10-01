package helpers

import (
	"strings"
	"unicode"
)

// source: https://stackoverflow.com/a/44282136
func ParseKeyValueString(text string) (data map[string]string) {
	lastQuote := rune(0)
	f := func(c rune) bool {
		switch {
		case c == lastQuote:
			lastQuote = rune(0)
			return false
		case lastQuote != rune(0):
			return false
		case unicode.In(c, unicode.Quotation_Mark):
			lastQuote = c
			return false
		default:
			return unicode.IsSpace(c)
		}
	}

	// splitting string by space but considering quoted section
	items := strings.FieldsFunc(text, f)

	// create and fill the map
	data = make(map[string]string)
	for _, item := range items {
		x := strings.Split(item, "=")
		data[x[0]] = x[1]
	}
	return data
}
