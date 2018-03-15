package helpers

import "unicode/utf8"

func RuneLength(input string) (length int) {
	return utf8.RuneCountInString(input)
}
