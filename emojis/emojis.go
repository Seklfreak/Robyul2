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
