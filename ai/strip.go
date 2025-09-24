package ai

import "strings"

// stripThoughts removes the first <think>...</think> tag and its content from the input string only if the string's prefix is <think>
func stripThoughts(input string) string {
	if strings.HasPrefix(input, "<think>") {
		endTag := "</think>"
		endIndex := strings.Index(input, endTag)
		if endIndex != -1 {
			return input[endIndex+len(endTag):]
		}
	}
	return input
}
