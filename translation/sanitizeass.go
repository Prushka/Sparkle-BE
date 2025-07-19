package translation

import "strings"

// only translate when Default or English in dialogue block
// (and for those blocks, send only text)
func sanitizeInputASS(input string) (string, string) {
	lines := strings.Split(input, "\n")
	var resultLines []string
	var dialogueLines []string
	for _, line := range lines {
		if shouldTranslate(line) {
			dialogueLines = append(dialogueLines, line)
		} else {
			resultLines = append(resultLines, line)
		}
	}
	return strings.Join(resultLines, "\n"), strings.Join(dialogueLines, "\n")
}

func shouldTranslate(input string) bool {
	return strings.Contains(strings.ToLower(input), "dialogue") &&
		strings.Contains(strings.ToLower(input), ":") &&
		strings.Contains(strings.ToLower(input), ".") &&
		strings.Contains(strings.ToLower(input), ",") &&
		(strings.Contains(strings.ToLower(input), "default") ||
			strings.Contains(strings.ToLower(input), "english"))
}

func isFormatLine(input string) bool {
	return strings.Contains(strings.ToLower(input), "format") &&
		strings.Contains(strings.ToLower(input), "start") &&
		strings.Contains(strings.ToLower(input), "end") &&
		strings.Contains(strings.ToLower(input), "text")
}

func sanitizeOutputASS(headers, translated string) string {
	headerLines := normalizeBlock(strings.Split(headers, "\n"), false)
	translatedLines := normalizeBlock(strings.Split(removeSingleFullStops(translated), "\n"), false)
	for i, l := range translatedLines {
		runes := []rune(l)
		n := len(runes)
		if n > 0 && runes[n-1] == '，' {
			// only remove if it's a single full stop (not preceded by another)
			if n < 2 || runes[n-2] != '，' {
				// drop the last rune ("，")
				translatedLines[i] = string(runes[:n-1])
			}
		} else if n > 0 && runes[n-1] == '。' {
			// only remove if it's a single full stop (not preceded by another)
			if n < 2 || runes[n-2] != '。' {
				// drop the last rune ("。")
				translatedLines[i] = string(runes[:n-1])
			}
		}
	}
	for i, line := range headerLines {
		if isFormatLine(line) {
			headerLines[i] = line + "\n" + strings.Join(translatedLines, "\n")
		}
	}
	return strings.Join(headerLines, "\n")
}
