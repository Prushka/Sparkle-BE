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
	"unicode"
)

type FormatPositions struct {
	Text        int
	Start       int
	End         int
	TotalCommas int
}

func findFormatPositions(input string) (pos FormatPositions, err error) {
	pos.Text = -1
	pos.Start = -1
	pos.End = -1
	pos.TotalCommas = -1
	for _, line := range strings.Split(input, "\n") {
		if isFormatLine(line) {
			pos.Text = findField(line, "text")
			pos.Start = findField(line, "start")
			pos.End = findField(line, "end")
			pos.TotalCommas = countCommas(line)
			if pos.TotalCommas <= 0 || pos.Text < 0 || pos.Start < 0 || pos.End < 0 {
				err = fmt.Errorf("unable to preprocess format line headers: %+v", pos)
			}
			return
		}
	}
	err = fmt.Errorf("format line not found")
	return
}

func sanitizeInputASS(input string) (string, string, error) {
	lines := strings.Split(strings.ReplaceAll(input, string(rune(0)), ""), "\n")
	var resultLines []string
	var dialogueLines []string
	pos, err := findFormatPositions(input)
	if err != nil {
		return "", "", err
	}
	counts := make(map[string]int)
	for _, line := range lines {
		counts[line]++
	}
	for _, line := range lines {
		if isDialogueLine(line) && isTranslatableText(line, pos, counts) {
			dialogueLines = append(dialogueLines, RemoveComments(sanitizeDialogueLineTime(line, pos.Start, pos.End)))
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

func countCommas(input string) int {
	headerLine := strings.ReplaceAll(strings.TrimPrefix(strings.ToLower(input), "format:"), " ", "")
	headerLine = strings.ReplaceAll(strings.ReplaceAll(headerLine, "\n", ""), "\r", "")
	return strings.Count(headerLine, ",")
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

var overrideBlockRegex = regexp.MustCompile(`\{[^}]*}`)
var strictOverrideBlockRegex = regexp.MustCompile(`\{[^}]*\\[^}]*}`)

// hardVisualEffectRegex finds tags that are almost always non-translatable inside a { } block.
var hardVisualEffectRegex = regexp.MustCompile(`\{[^}]*(?:\\p[1-9]|\\clip|\\iclip)[^}]*}`)

// animationTagRegex finds tags that might be used on translatable text inside a { } block.
var animationTagRegex = regexp.MustCompile(`\{[^}]*(?:\\t|\\move)[^}]*}`)

var weakAnimationTags = []*regexp.Regexp{
	regexp.MustCompile(`\{[^}]*\\fad[^}]*}`),
	regexp.MustCompile(`\{[^}]*\\pos[^}]*}`),
	regexp.MustCompile(`\{[^}]*\\blur[^}]*}`),
	regexp.MustCompile(`\{[^}]*\\alpha[^}]*}`),
}

// isTranslatableText checks if an ASS dialogue line contains meaningful, translatable text.
// It returns false for drawing commands, visual effects, or lines with very short durations.
func isTranslatableText(dialogueLine string, pos FormatPositions, counts map[string]int) bool {

	textPart := extractDialogueField(dialogueLine, pos.Text, true)
	startTimeStr := extractDialogueField(dialogueLine, pos.Start, false)
	endTimeStr := extractDialogueField(dialogueLine, pos.End, false)

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

	blockCount := len(overrideBlockRegex.FindAllString(textPart, -1))
	wordCount := len(strings.Fields(cleanText))

	weakCount := 0
	for _, weak := range weakAnimationTags {
		if weak.MatchString(textPart) {
			weakCount++
		}
	}

	if weakCount >= 3 && !strings.Contains(cleanText, " ") {
		return false
	}

	if weakCount >= 2 && wordCount > 0 && blockCount > wordCount*3 && blockCount > 9 {
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
		if wordCount > 0 && blockCount > wordCount+1 { // Allow one block for overall styling
			return false
		}

		// If there are more than 3 override blocks, it's likely a visual effect.
		if blockCount > 3 {
			return false
		}
	}

	// The line repeats itself more than 5 times, and is short
	if counts[dialogueLine] > 5 && (len(cleanText) < 6 || wordCount < 2) {
		return false
	}

	if likelySign(cleanText) {
		return false
	}

	// TODO: what about japanese moving animation (room label) translatable, very long text
	return true
}

func likelySign(input string) bool {
	if len(input) > 4 {
		return false
	}
	for _, r := range input {
		if !unicode.IsLetter(r) || r > unicode.MaxASCII {
			return false
		}
		if !unicode.IsLower(r) {
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

	dest := utils.ReplaceExtension(file, ".vtt")
	cmd := exec.Command(config.TheConfig.Ffmpeg, "-y", "-i", tmp, "-c:s", "webvtt",
		dest)
	_, err = utils.RunCommand(cmd)
	if err != nil {
		return err
	}

	vttOutput, err := os.ReadFile(dest)
	if err != nil {
		return err
	}
	var cleaned []string
	for _, line := range strings.Split(string(vttOutput), "\n") {
		cleaned = append(cleaned, strictOverrideBlockRegex.ReplaceAllString(line, ""))
	}
	err = os.WriteFile(dest, []byte(strings.Join(cleaned, "\n")), 0644)
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
