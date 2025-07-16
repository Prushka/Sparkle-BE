package ai

import "strings"

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

	return TrimPeriods(strings.Join(filtered[start:end+1], "\n"))
}

// TrimPeriods modifies a string by finding lines with "-->" and removing
// a single "。" from the end of the last non-empty line before it.
// If the preceding line ends with multiple "。", it is left unchanged.
func TrimPeriods(input string) string {
	lines := strings.Split(input, "\n")

	for i := 0; i < len(lines); i++ {
		if strings.Contains(lines[i], "-->") {
			// Look for the last non-empty line before the current line
			lastNonEmptyIdx := -1
			for j := i - 1; j >= 0; j-- {
				if strings.TrimSpace(lines[j]) != "" {
					lastNonEmptyIdx = j
					break
				}
			}

			// Skip if no non-empty line was found
			if lastNonEmptyIdx == -1 {
				continue
			}

			line := lines[lastNonEmptyIdx]

			// Check if line ends with exactly one "。"
			if strings.HasSuffix(line, "。") {
				// Count how many "。" characters at the end
				endingPart := line
				for len(endingPart) > 0 && strings.HasSuffix(endingPart, "。") {
					endingPart = endingPart[:len(endingPart)-len("。")]
				}

				periodsAtEnd := (len(line) - len(endingPart)) / len("。")

				// If exactly one "。" is at the end, remove it
				if periodsAtEnd == 1 {
					lines[lastNonEmptyIdx] = line[:len(line)-len("。")]
				}
			}
		}
	}

	return strings.Join(lines, "\n")
}
