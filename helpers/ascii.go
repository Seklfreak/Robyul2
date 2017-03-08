package helpers

import (
    "encoding/base64"
    "fmt"
    "golang.org/x/text/unicode/norm"
    "regexp"
    "strconv"
    "strings"
)

// BtoA is a polyfill for javascript's window#btoa()
func BtoA(s string) string {
    b64 := base64.URLEncoding.WithPadding(base64.NoPadding)
    src := []byte(s)
    buf := make([]byte, b64.EncodedLen(len(src)))
    b64.Encode(buf, src)

    return string(buf)
}

// DrawTable draws a fancy ASCII table
// Inspired by MySQL
func DrawTable(headers []string, rows [][]string) string {
    // Whether we hit discord's limit or not
    contentsOmitted := false
    rowsPrinted := 0

    // Result container
    sb := ""

    // Determine biggest padding for each col
    // First headers, then rows
    paddings := make([]int, len(headers))

    for idx, header := range headers {
        if paddings[idx] < len(header) {
            paddings[idx] = len(header)
        }
    }

    for _, row := range rows {
        for cidx, col := range row {
            tmp := norm.NFC.String(col)
            length := len(tmp)

            if paddings[cidx] < length {
                paddings[cidx] = length
            }
        }
    }

    // Make this a code tag
    sb += "```\n"

    // Draw header
    sb += drawLine("+", "-", "+", paddings, headers)
    sb += drawContent("|", "|", "|", paddings, headers)
    sb += drawLine("+", "-", "+", paddings, headers)

    // Draw content
    for _, row := range rows {
        // If we're about to hit discord's limit print ... to indicate there's more we can't show
        if len(sb) >= 1600 {
            contentsOmitted = true

            dummyRow := make([]string, len(headers))
            for idx := range dummyRow {
                dummyRow[idx] = "..."
            }

            sb += drawContent("|", "|", "|", paddings, dummyRow)
            break
        }

        // Else print row
        rowsPrinted++
        sb += drawContent("|", "|", "|", paddings, row)
    }

    // Draw bottom border
    sb += drawLine("+", "-", "+", paddings, headers)

    // If we hit discord's limit let the user know
    if contentsOmitted {
        rowCount := len(rows) - rowsPrinted
        sb += strconv.Itoa(rowCount)

        if rowCount == 1 {
            sb += " row"
        } else {
            sb += " rows"
        }

        sb += " omitted because of discord's message size limit.\n"
    }

    // End code tag
    sb += "```"

    return sb
}

// drawLine draws a line with given paddings and chars (eg "+-----+-----+-----+")
func drawLine(start string, mid string, end string, paddings []int, data []string) string {
    sb := ""
    for idx := range data {
        sb += start + strings.Repeat(mid, paddings[idx])
    }
    sb += end + "\n"

    return sb
}

// drawContent draws content with padding and custom separators (eg "|A    |B    |C    |")
func drawContent(start string, separator string, end string, paddings []int, data []string) string {
    sanitizer := regexp.MustCompile(
        `[\r\n\t\f\v\x{2028}\x{2029}]+`,
    )
    unifier := regexp.MustCompile(
        `[` +
            `\x{0020}\x{00A0}\x{1680}\x{180E}` +
            `\x{2000}\x{2001}\x{2002}\x{2003}` +
            `\x{2004}\x{2005}\x{2006}\x{2007}` +
            `\x{2008}\x{2009}\x{200A}\x{200B}` +
            `\x{202F}\x{205F}\x{3000}\x{FEFF}` +
            `\x{2423}\x{2422}\x{2420}` + // "visible" spaces
            `]+`,
    )

    sb := ""
    for idx, content := range data {
        if idx == 0 {
            sb += start
        } else {
            sb += separator
        }

        content = norm.NFC.String(content)
        content = sanitizer.ReplaceAllString(content, "")
        content = unifier.ReplaceAllString(content, " ")
        sb += fmt.Sprintf(
            "%-"+strconv.Itoa(paddings[idx])+"s",
            content,
        )
    }

    sb += end + "\n"

    return sb
}
