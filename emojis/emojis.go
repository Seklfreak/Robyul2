package emojis

import "strconv"

var list = map[string]string{
	"0":  `0⃣`,
	"1":  `1⃣`,
	"2":  `2⃣`,
	"3":  `3⃣`,
	"4":  `4⃣`,
	"5":  `5⃣`,
	"6":  `6⃣`,
	"7":  `7⃣`,
	"8":  `8⃣`,
	"9":  `9⃣`,
	"10": `🔟`,
}

var textList = map[string]string{
	"0":  `:zero:`,
	"1":  `:one:`,
	"2":  `:two:`,
	"3":  `:three:`,
	"4":  `:four:`,
	"5":  `:five:`,
	"6":  `:six:`,
	"7":  `:seven:`,
	"8":  `:eight:`,
	"9":  `:nine:`,
	"10": `:keycap_ten:`,
}

// revlist is the reverse version of list
var revlist map[string]string

func init() {
	revlist = make(map[string]string, len(list))
	for k, v := range list {
		revlist[v] = k
	}
}

// From returns the unicode emoji code for the symbol
func From(symbol string) string {
	return list[symbol]
}

// From returns the unicode emoji code for the symbol
func FromToText(symbol string) string {
	return textList[symbol]
}

// To returns the symbol from the emoji
func To(symbol string) string {
	return revlist[symbol]
}

// NumberFromEmoji returns the number that corresponds to
// the emoji
func ToNumber(emoji string) int {
	v, err := strconv.Atoi(revlist[emoji])
	if err != nil {
		v = -1
	}
	return v
}
