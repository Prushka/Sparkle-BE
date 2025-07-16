package ai

import (
	"Sparkle/utils"
	"regexp"
	"strings"
)

// sanitizeSegment removes lines with WEBVTT or ```
// and trims leading and trailing empty lines.
func sanitizeSegment(input string) string {
	lines := strings.Split(input, "\n")

	var filtered []string
	for _, line := range lines {
		curr := strings.ToLower(strings.TrimSpace(line))
		if strings.Contains(curr, "webvtt") || strings.Contains(curr, "```") {
			continue
		}
		filtered = append(filtered, line)
	}

	start := 0
	for start < len(filtered) && strings.TrimSpace(filtered[start]) == "" {
		start++
	}

	end := len(filtered) - 1
	for end >= start && strings.TrimSpace(filtered[end]) == "" {
		end--
	}

	if start > end {
		return ""
	}

	return trimPeriods(strings.Join(filtered[start:end+1], "\n"))
}

// trimPeriods scans the input text line by line.
// Whenever it finds a time range line, it looks back for the most recent non-empty line before it.
// If that line ends with exactly one Chinese full stop "。"
// (and not multiple), it removes that final "。"—respecting any trailing HTML tags
// like </i></b>.
func trimPeriods(text string) string {
	lines := strings.Split(text+"\n", "\n")

	// regex to capture any trailing HTML closing tags, e.g. </b></i>
	tagRe := regexp.MustCompile(`(?i)(</[^>]+>)+\s*$`)

	for i, line := range lines {
		if utils.IsWebVTTTimeRangeLine(line) || i == len(lines)-1 {
			// scan backwards for the last non-empty line
			for j := i - 1; j >= 0; j-- {
				if utils.IsWebVTTTimeRangeLine(lines[j]) {
					break
				}
				if strings.TrimSpace(lines[j]) == "" {
					continue
				}
				// separate content from trailing tags
				tags := tagRe.FindString(lines[j])
				content := strings.TrimSuffix(lines[j], tags)

				// work rune-wise to handle multibyte characters correctly
				runes := []rune(content)
				n := len(runes)
				if n > 0 && runes[n-1] == '。' {
					// only remove if it's a single full stop (not preceded by another)
					if n < 2 || runes[n-2] != '。' {
						// drop the last rune ("。")
						content = string(runes[:n-1])
						lines[j] = content + tags
					}
				}
				break // only modify the first non-empty line before the timecode
			}
		}
	}

	return strings.Join(lines[:len(lines)-1], "\n")
}
