package translation

import (
	"Sparkle/config"
	"Sparkle/discord"
	"Sparkle/utils"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

func sanitizeInputASS(input string) (string, string, error) {
	lines := strings.Split(input, "\n")
	var resultLines []string
	var dialogueLines []string
	var start, end, text int
	for _, line := range lines {
		if isFormatLine(line) {
			// Find the indices of the fields in the format line
			text = findField(line, "text")
			start = findField(line, "start")
			end = findField(line, "end")
			if text < 0 || start < 0 || end < 0 {
				return "", "", fmt.Errorf("invalid format line: %s, start: %d, end: %d, text: %d", line, start, end, text)
			}
			resultLines = append(resultLines, line)
		} else if text > 0 && isDialogueLine(line) && isTranslatableText(line, start, end, text) {
			dialogueLines = append(dialogueLines, RemoveComments(sanitizeDialogueLineTime(line, start, end)))
		} else {
			resultLines = append(resultLines, line)
		}
	}
	return strings.Join(resultLines, "\n"), strings.Join(dialogueLines, "\n"), nil
}

func isDialogueLine(input string) bool {
	return strings.Contains(strings.ToLower(input), "dialogue:") &&
		strings.Contains(strings.ToLower(input), ":") &&
		strings.Contains(strings.ToLower(input), ".") &&
		strings.Contains(strings.ToLower(input), ",")
}

func isFormatLine(input string) bool {
	return strings.Contains(strings.ToLower(input), "format") &&
		strings.Contains(strings.ToLower(input), "start") &&
		strings.Contains(strings.ToLower(input), "end") &&
		strings.Contains(strings.ToLower(input), "text")
}

func sanitizeOutputASS(headers, translated string) string {
	headerLines := strings.Split(headers, "\n")
	translatedLines := normalizeBlock(strings.Split(
		removeSingleFullStops(removeSingleFullStops(translated, '。'), '，'), "\n"),
		false)
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

func findField(input, field string) int {
	// Remove the "Format: " prefix and any leading/trailing whitespace
	headerLine := strings.ReplaceAll(strings.TrimPrefix(strings.ToLower(input), "format:"), " ", "")
	headerLine = strings.ReplaceAll(strings.ReplaceAll(headerLine, "\n", ""), "\r", "")

	// Split the remaining string by the comma delimiter
	headers := strings.Split(headerLine, ",")

	for i, header := range headers {
		if header == strings.ToLower(field) {
			return i
		}
	}
	return -1
}

func extractDialogueField(line string, idx int, tillEnd bool) string {
	s := strings.Split(strings.TrimSpace(strings.TrimPrefix(line, "dialogue:")), ",")
	if len(s) > idx {
		field := strings.TrimSpace(s[idx])
		if tillEnd {
			if idx+1 < len(s) {
				return strings.TrimSpace(strings.Join(s[idx:], ","))
			}
		}
		return field
	}
	return ""
}

var overrideBlockRegex = regexp.MustCompile(`\{[^}]*}`)

// hardVisualEffectRegex finds tags that are almost always non-translatable inside a { } block.
var hardVisualEffectRegex = regexp.MustCompile(`\{[^}]*(?:\\p[1-9]|\\clip|\\iclip)[^}]*}`)

// animationTagRegex finds tags that might be used on translatable text inside a { } block.
var animationTagRegex = regexp.MustCompile(`\{[^}]*(?:\\t|\\move)[^}]*}`)

func sanitizeTime(timeStr string) string {
	// Check if the time is negative
	if strings.HasPrefix(timeStr, "-") {
		timeStr = timeStr[1:] // Remove the negative sign for parsing
	}

	// Split the time into hours, minutes, seconds, and milliseconds
	parts := strings.Split(timeStr, ":")

	// Ensure minutes and seconds have 2 digits
	if len(parts) >= 2 && len(parts[1]) == 1 {
		parts[1] = "0" + parts[1] // Add leading zero to minutes
	}
	if len(parts) >= 3 {
		secondsParts := strings.Split(parts[2], ".")
		if len(secondsParts[0]) == 1 {
			secondsParts[0] = "0" + secondsParts[0] // Add leading zero to seconds
		}
		parts[2] = strings.Join(secondsParts, ".")
	}

	// Reassemble the time string
	timeStr = strings.Join(parts, ":")

	// If milliseconds have more than 2 digits, trim to 2 digits
	if idx := strings.LastIndex(timeStr, "."); idx != -1 {
		// Check if milliseconds have more than 2 digits
		if len(timeStr) > idx+3 {
			timeStr = timeStr[:idx+3] // Trim to 2 digits of milliseconds
		} else if len(timeStr) == idx+2 {
			// If milliseconds have 1 digit, add a 0 to make it 2 digits
			timeStr = timeStr + "0"
		}
	}

	return timeStr
}

func sanitizeDialogueLineTime(dialogueLine string, start, end int) string {
	startTimeStr := extractDialogueField(dialogueLine, start, false)
	endTimeStr := extractDialogueField(dialogueLine, end, false)

	startTimeSanitized := sanitizeTime(startTimeStr)
	endTimeSanitized := sanitizeTime(endTimeStr)
	if startTimeSanitized != startTimeStr {
		dialogueLine = strings.ReplaceAll(dialogueLine, startTimeStr, startTimeSanitized)
	}
	if endTimeSanitized != endTimeStr {
		dialogueLine = strings.ReplaceAll(dialogueLine, endTimeStr, endTimeSanitized)
	}
	return dialogueLine
}

// isTranslatableText checks if an ASS dialogue line contains meaningful, translatable text.
// It returns false for drawing commands, visual effects, or lines with very short durations.
func isTranslatableText(dialogueLine string, start, end, text int) bool {

	textPart := extractDialogueField(dialogueLine, text, true)
	startTimeStr := extractDialogueField(dialogueLine, start, false)
	endTimeStr := extractDialogueField(dialogueLine, end, false)

	// Heuristic 1: Check for drawing commands, clipping, or animation within the override block.
	if hardVisualEffectRegex.MatchString(textPart) {
		return false
	}

	// Heuristic 2: Check the duration. Short durations often indicate visual effects.
	startTime, err1 := time.Parse(ASSTimeFormat, sanitizeTime(startTimeStr))
	endTime, err2 := time.Parse(ASSTimeFormat, sanitizeTime(endTimeStr))

	if err1 == nil && err2 == nil {
		duration := endTime.Sub(startTime)
		// Lines displayed for less than half a second are likely not for reading.
		if duration < 280*time.Millisecond {
			return false
		}
	} else {
		discord.Errorf("Failed to parse time, start: %s, end: %s, %s, %v, %v", startTimeStr, endTimeStr, dialogueLine, err1, err2)
		return false
	}

	// Heuristic 3: Check the actual text content after stripping style overrides.
	cleanText := overrideBlockRegex.ReplaceAllString(textPart, "")
	cleanText = strings.TrimSpace(cleanText)

	if len(cleanText) == 0 {
		// No text content.
		return false
	}

	// Lines with only 1 character are often signs or effects, not dialogue.
	if len(cleanText) < 2 {
		return false
	}

	// Heuristic 4: Check for animation. If found, apply stricter content rules.
	if animationTagRegex.MatchString(textPart) {
		if len(cleanText) < 5 {
			// Lines with very short text and animation tags are likely visual effects.
			return false
		}

		// Animated lines with very short text are likely effects.
		// We check for more than one word as a simple heuristic.
		if !strings.Contains(cleanText, " ") && len(cleanText) < 8 {
			return false
		}

		// Per-character animation (many override blocks) is a strong sign of a visual effect.
		// If there are more override blocks than words, it's probably an effect.
		blockCount := len(overrideBlockRegex.FindAllString(textPart, -1))
		wordCount := len(strings.Fields(cleanText))
		if wordCount > 0 && blockCount > wordCount+1 { // Allow one block for overall styling
			return false
		}

		// If there are more than 3 override blocks, it's likely a visual effect.
		if blockCount > 3 {
			return false
		}
	}

	return true
}

// RemoveComments removes comment blocks from an ASS dialogue line's text part.
// It identifies comments as any {}-enclosed block that does not contain a backslash '\',
// thus preserving valid override tag blocks.
func RemoveComments(dialogueText string) string {
	// The replacer function is called for each match found by the regex.
	replacer := func(block string) string {
		// If the block does NOT contain a backslash, it's a comment. Replace it with nothing.
		if !strings.Contains(block, `\`) {
			return ""
		}
		// Otherwise, it's an override tag block. Keep it unchanged.
		return block
	}

	return overrideBlockRegex.ReplaceAllStringFunc(dialogueText, replacer)
}

func AssToVTT(file string) error {
	fBytes, err := os.ReadFile(file)
	if err != nil {
		return err
	}
	headers, translatable, err := sanitizeInputASS(string(fBytes))
	if err != nil {
		return err
	}
	var resultLines []string
	for _, h := range strings.Split(headers, "\n") {
		if !isDialogueLine(h) {
			resultLines = append(resultLines, h)
		} else {
			break
		}
	}
	out := strings.Join(resultLines, "\n") + "\n" + translatable
	tmp := addTempSuffix(file)
	if err := os.WriteFile(tmp, []byte(out), 0644); err != nil {
		return fmt.Errorf("failed to write converted file: %w", err)
	}

	defer func() {
		if err := os.Remove(tmp); err != nil {
			discord.Errorf("Error removing temporary file %s: %v", tmp, err)
		}
	}()

	cmd := exec.Command(config.TheConfig.Ffmpeg, "-y", "-i", tmp, "-c:s", "webvtt",
		strings.ReplaceAll(file, ".ass", ".vtt"))
	_, err = utils.RunCommand(cmd)
	if err != nil {
		return err
	}

	return nil
}

// addTempSuffix inserts "_temp" before the file’s extension.
func addTempSuffix(path string) string {
	ext := filepath.Ext(path)             // ".csv", ".gz", or ""
	base := strings.TrimSuffix(path, ext) // everything except the extension
	return base + "_temp" + ext
}
