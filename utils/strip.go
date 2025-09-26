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
		if HasTimePrefix(line) {
			outputLines = append(outputLines, line)
		}
	}
	return strings.Join(outputLines, "\n")
}

// HasTimePrefix checks if the input string has a prefix
// that can be parsed by ASSTimeFormat.
func HasTimePrefix(s string) bool {
	// The prefix to be checked must be at least as long as the time format.
	if len(s) < len(ASSTimeFormat) {
		return false
	}

	// Extract the prefix of the same length as the format.
	prefix := s[:len(ASSTimeFormat)]

	// Attempt to parse the prefix.
	_, err := time.Parse(ASSTimeFormat, prefix)

	// If there is no error, the prefix is a valid time in the given format.
	return err == nil
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
