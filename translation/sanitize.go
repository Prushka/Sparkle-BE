package translation

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

	return trimCommas(trimLinesPreserveTags(removeSingleFullStops(strings.Join(filtered[start:end+1], "\n"))))
}

// removeSingleFullStops replace any lone '。' with space while preserving contiguous runs of '。'
func removeSingleFullStops(input string) string {
	var b strings.Builder
	runes := []rune(input)

	for i := 0; i < len(runes); {
		if runes[i] == '。' {
			// count how many consecutive '。' we have
			j := i + 1
			for j < len(runes) && runes[j] == '。' {
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

var trimLineRE = regexp.MustCompile(
	`^((?:<[^>]+>)*)` + // group1: zero or more opening tags at the start
		`(.*?)` + // group2: minimal content in between
		`((?:</[^>]+>)*)$`, // group3: zero or more closing tags at the end
)

// trimLinePreserveTags removes all leading and trailing spaces from the content
// of a single line but ignores any outer HTML tags.
func trimLinePreserveTags(line string) string {
	if m := trimLineRE.FindStringSubmatch(line); m != nil {
		leadingTags := m[1]
		innerContent := m[2]
		trailingTags := m[3]
		trimmed := strings.TrimSpace(innerContent)
		return leadingTags + trimmed + trailingTags
	}
	// fallback (regex always matches, but just in case):
	return strings.TrimSpace(line)
}

// trimLinesPreserveTags applies TrimLinePreserveTags to every line in input,
// preserving line breaks.
func trimLinesPreserveTags(input string) string {
	lines := strings.Split(input, "\n")
	for i, ln := range lines {
		lines[i] = trimLinePreserveTags(ln)
	}
	return strings.Join(lines, "\n")
}

// trimCommas scans the input text line by line.
// Whenever it finds a time range line, it looks back for the most recent non-empty line before it.
// If that line ends with exactly one Chinese full stop "，"
// (and not multiple), it removes that final "，"—respecting any trailing HTML tags
// like </i></b>.
func trimCommas(text string) string {
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
				if n > 0 && runes[n-1] == '，' {
					// only remove if it's a single full stop (not preceded by another)
					if n < 2 || runes[n-2] != '，' {
						// drop the last rune ("，")
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

func splitAssembled(assembled string, atLine int) []string {
	lines := strings.Split(assembled, "\n")

	var (
		result       []string
		currentLines []string
		count        int
	)

	for i, line := range lines {
		if strings.TrimSpace(line) == "" && count >= atLine {
			if i+1 >= len(lines) || utils.IsWebVTTTimeRangeLine(lines[i+1]) {
				result = append(result, strings.Join(currentLines, "\n"))
				currentLines = nil
				count = 0
				continue
			}
		}

		currentLines = append(currentLines, line)
		count++
	}

	if len(currentLines) > 0 {
		result = append(result, strings.Join(currentLines, "\n"))
	}

	return result
}

// sanitizeWebVTT removes contiguous duplicate blocks and empty blocks from text.
// A block starts with a time range line and ends at either the last line
// or the next time range line.
// Two blocks are considered identical if they are identical after removing all empty lines.
func sanitizeWebVTT(input string) string {
	if input == "" {
		return ""
	}

	lines := strings.Split(input, "\n")
	var resultLines []string

	// Find all block start indices
	var blockStarts []int
	for i, line := range lines {
		if utils.IsWebVTTTimeRangeLine(line) {
			blockStarts = append(blockStarts, i)
		}
	}

	// If no blocks found, return original input
	if len(blockStarts) == 0 {
		return input
	}

	// Add lines before the first block
	if blockStarts[0] > 0 {
		resultLines = append(resultLines, lines[:blockStarts[0]]...)
	}

	var lastNormalizedBlock string
	for i, start := range blockStarts {
		// Determine the end of the block
		end := len(lines)
		if i+1 < len(blockStarts) {
			end = blockStarts[i+1]
		}

		// Extract the block
		block := lines[start:end]

		// Normalize the block for comparison (remove empty lines)
		normalizedBlock := normalizeBlock(block)

		// Only add the block if it's different from the last one
		if (len(strings.Split(normalizedBlock, "\n")) > 1) &&
			(i == 0 || normalizedBlock != lastNormalizedBlock) {
			resultLines = append(resultLines, block...)
			lastNormalizedBlock = normalizedBlock
		}
	}

	return strings.Join(resultLines, "\n")
}

// normalizeBlock removes empty lines from a block and returns it as a string
func normalizeBlock(block []string) string {
	var nonEmptyLines []string
	for _, line := range block {
		if strings.TrimSpace(line) != "" {
			nonEmptyLines = append(nonEmptyLines, line)
		}
	}
	return strings.Join(nonEmptyLines, "\n")
}
