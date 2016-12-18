package helpers

import "strings"

// Draws an ASCII table
// Inspired by MySQL
func DrawTable(headers []string, rows [][]string) string {
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
    sb += drawContent("|", "|", paddings, headers)
    sb += drawLine("+", "-", "+", paddings, headers)

    // Draw content
    for _, row := range rows {
        sb += drawContent("|", "|", paddings, row)
    }

    // Draw bottom border
    sb += drawLine("+", "-", "+", paddings, headers)

    // End code tag
    sb += "```"

    return sb
}

// Draws a line with given paddings and chars (eg "+-----+-----+-----+")
func drawLine(start string, mid string, end string, paddings []int, data []string) string {
    sb := ""
    for idx := range data {
        sb += start + strings.Repeat(mid, paddings[idx])
    }
    sb += end + "\n"

    return sb
}

// Draws content with padding and custom separators (eg "|A    |B    |C    |")
func drawContent(separator string, end string, paddings []int, data []string) string {
    sb := ""
    for idx, header := range data {
        sb += separator + header + strings.Repeat(" ", paddings[idx] - len(header))
    }
    sb += end + "\n"

    return sb
}
