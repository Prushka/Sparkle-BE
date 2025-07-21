package translation

import (
	"Sparkle/utils"
	"github.com/labstack/gommon/log"
	"regexp"
	"strings"
	"unicode"
)

const isStyleCutoff = 100

func sanitizeOutputVTT(input string) string {
	return trimCommas(trimLinesPreserveTags(
		removeSingleFullStops(removeSingleFullStops(sanitizeSegment(input), '。'), '，')))
}

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

	return strings.Join(filtered[start:end+1], "\n")
}

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

func splitByCharacters(assembled string, atChar int, skipVTTCutOff bool) []string {
	lines := strings.Split(assembled, "\n")

	var (
		result       []string
		currentLines []string
		count        int
	)

	for i, line := range lines {
		if count >= atChar {
			if i+1 >= len(lines) || utils.IsWebVTTTimeRangeLine(lines[i+1]) || skipVTTCutOff {
				result = append(result, strings.Join(currentLines, "\n"))
				currentLines = nil
				count = 0
				continue
			}
		}

		currentLines = append(currentLines, line)
		count += len(line)
	}

	if len(currentLines) > 0 {
		result = append(result, strings.Join(currentLines, "\n"))
	}

	return result
}

func sanitizeInputVTT(input string) string {
	return "WEBVTT\n" + sanitizeBlocks(sanitizeBlocks(input, true), false)
}

// TODO: context aware seasons

// sanitizeBlocks removes contiguous duplicate blocks and empty blocks from text.
// A block starts with a time range line and ends at either the last line
// or the next time range line.
// Two blocks are considered identical if they are identical after removing all empty lines.
// Only run the following when contiguousOnly is true:
// If the last block contains the same content as the current block, same end and start time, merge two time
// if the last block contains the same time as the current block, different content, merge content, new line in between
func sanitizeBlocks(input string, contiguousOnly bool) string {
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
	// if blockStarts[0] > 0 {
	// 	resultLines = append(resultLines, normalizeBlock(lines[:blockStarts[0]])...)
	// }

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
		normalizedBlockSlice := normalizeBlock(block, false)
		normalizedBlock := strings.Join(normalizedBlockSlice, "\n")

		styleCharsInBlock := countDigitsAndSpecialChars(normalizedBlock)

		if styleCharsInBlock >= isStyleCutoff {
			log.Debugf("Prob styled block: %d | %s", styleCharsInBlock, normalizedBlock)
		}

		HTMLStrippedNormalizedBlock := normalizeBlock(block, true)

		if (len(HTMLStrippedNormalizedBlock) > 1) && // non-empty content
			(i == 0 || normalizedBlock != lastNormalizedBlock) && // not duplicate block (same time and content)
			(styleCharsInBlock < isStyleCutoff) { // not a style block
			// we are trying to add this block
			if lastNormalizedBlock != "" && !contiguousOnly {
				splitCurr := strings.Split(normalizedBlock, "\n")
				currTimeLine := splitCurr[0]
				splitLast := strings.Split(lastNormalizedBlock, "\n")
				lastTimeLine := splitLast[0]
				var (
					currTimeStart string
					currTimeEnd   string
					lastTimeStart string
					lastTimeEnd   string
				)
				curr := utils.WebvttTimeRangeRegex.FindStringSubmatch(currTimeLine)
				last := utils.WebvttTimeRangeRegex.FindStringSubmatch(lastTimeLine)
				if curr != nil && last != nil {
					currTimeStart = curr[1]
					currTimeEnd = curr[3]
					lastTimeStart = last[1]
					lastTimeEnd = last[3]
					currContent := splitCurr[1:]
					lastContent := splitLast[1:]
					if curr[0] == last[0] { // same time
						resultLines = append(resultLines, normalizedBlockSlice[1:]...)
						lastNormalizedBlock = lastNormalizedBlock + strings.Join(normalizedBlockSlice[1:], "\n")
						continue
					} else if currTimeStart == lastTimeEnd && strings.Join(currContent, "") == strings.Join(lastContent, "") {
						// same content, and last end time = curr start time
						for j := len(resultLines) - 1; j >= 0; j-- {
							if utils.IsWebVTTTimeRangeLine(resultLines[j]) { // find the lastBlock in result lines
								resultLines[j] = lastTimeStart + " --> " + currTimeEnd
								lastNormalizedBlock = resultLines[j] + "\n" + strings.Join(strings.Split(lastNormalizedBlock, "\n")[1:], "\n")
								break
							}
						}
						continue
					}
				}
			}

			// only add the result when we aren't skipping the current block
			resultLines = append(resultLines, "")
			resultLines = append(resultLines, normalizedBlockSlice...)
			lastNormalizedBlock = normalizedBlock
		}
	}

	return strings.Join(resultLines, "\n")
}

// countDigitsAndSpecialChars counts the number of characters in a block
// that may belong to a style.
func countDigitsAndSpecialChars(s string) int {
	count := 0
	for _, r := range s {
		if unicode.IsDigit(r) {
			count++
			continue
		}
		switch r {
		case '{', '}', '\\', '&', '*', '(', ')':
			count++
		}
	}
	return count
}

// normalizeBlock removes empty lines from a block and returns it as a string slice
// HTML tags are unwrapped first if treatHTML is set to true
func normalizeBlock(block []string, treatHTML bool) []string {
	var nonEmptyLines []string
	for _, line := range block {
		if treatHTML {
			if m := trimLineRE.FindStringSubmatch(line); m != nil {
				line = m[2]
			}
		}
		if strings.TrimSpace(line) != "" {
			nonEmptyLines = append(nonEmptyLines, line)
		}
	}
	return nonEmptyLines
}
