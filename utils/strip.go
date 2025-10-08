package utils

import (
	"strings"
	"time"
)

const ASSTimeFormat = "15:04:05.00"

// KeepOnlySubtitles removes the first <think>...</think> tag and its content from the input string only if the string's prefix is <think>
// it also removes anything that's not a subtitle line
func KeepOnlySubtitles(input string) string {
	inputLines := RemoveEmptyLinesAndTrimSpaces(strings.Split(input, "\n"))
	var outputLines []string
	for _, line := range inputLines {
		if HasValidTime(line) {
			outputLines = append(outputLines, line)
		}
	}
	return strings.Join(outputLines, "\n")
}

// HasValidTime checks if the input string has a prefix
// that can be parsed by ASSTimeFormat.
func HasValidTime(s string) bool {
	split := strings.SplitN(s, ",", 3)
	if len(split) < 3 {
		return false
	}
	start := split[0]
	end := split[1]
	_, errStart := time.Parse(ASSTimeFormat, start)
	_, errEnd := time.Parse(ASSTimeFormat, end)
	return errStart == nil || errEnd == nil // at least one of them should be valid to be corrected
}

func RemoveEmptyLinesAndTrimSpaces(block []string) []string {
	var nonEmptyLines []string
	for _, line := range block {
		trimmed := strings.TrimSpace(line)
		if trimmed != "" {
			nonEmptyLines = append(nonEmptyLines, trimmed)
		}
	}
	return nonEmptyLines
}
