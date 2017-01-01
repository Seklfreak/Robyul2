package helpers

import (
    "strings"
    "encoding/base64"
    "strconv"
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
            if paddings[cidx] < len(col) {
                paddings[cidx] = len(col)
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
    sb := ""
    for idx, content := range data {
        if idx == 0 {
            sb += start
        } else {
            sb += separator
        }

        sb += content + strings.Repeat(" ", paddings[idx] - len(content))
    }
    sb += end + "\n"

    return sb
}
