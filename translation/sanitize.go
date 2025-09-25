package translation

import (
	"Sparkle/utils"
	"strings"
)

// removeSingleFullStops replace any lone [char] with space while preserving contiguous runs of [char]
func removeSingleFullStops(input string, char rune) string {
	var b strings.Builder
	runes := []rune(input)

	for i := 0; i < len(runes); {
		if runes[i] == char {
			// count how many consecutive [char] we have
			j := i + 1
			for j < len(runes) && runes[j] == char {
				j++
			}
			count := j - i

			// if it's a run of 2 or more, write them; otherwise write a space
			if count > 1 {
				b.WriteString(string(runes[i:j]))
			} else {
				b.WriteString(" ")
			}
			i = j
		} else {
			b.WriteRune(runes[i])
			i++
		}
	}

	return b.String()
}

func splitByCharacters(lines []string, atChar int) []utils.PairSlice[string, int] {
	var (
		result       []utils.PairSlice[string, int]
		currentLines utils.PairSlice[string, int]
		count        int
	)

	for i, line := range lines {
		currentLines = append(currentLines, utils.Pair[string, int]{Left: line, Right: i})
		count += len(line)
		if count >= atChar {
			result = append(result, currentLines)
			currentLines = nil
			count = 0
		}
	}

	if len(currentLines) > 0 {
		result = append(result, currentLines)
	}

	return result
}

// TODO: context aware seasons

func removeEmptyLinesAndTrimSpaces(block []string) []string {
	var nonEmptyLines []string
	for _, line := range block {
		trimmed := strings.TrimSpace(line)
		if trimmed != "" {
			nonEmptyLines = append(nonEmptyLines, trimmed)
		}
	}
	return nonEmptyLines
}
