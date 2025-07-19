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
