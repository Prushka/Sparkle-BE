package translation

import (
	"Sparkle/discord"
	"fmt"
	"os"
	"strings"
	"time"
)

const ASSTimeFormat = "15:04:05.00"

func isASSOutputValid(headers string, output []string) bool {
	var start, end, text int
	for _, line := range strings.Split(headers, "\n") {
		if isFormatLine(line) {
			// Find the indices of the fields in the format line
			text = findField(line, "text")
			start = findField(line, "start")
			end = findField(line, "end")
			if text < 0 || start < 0 || end < 0 {
				return false
			}
			break
		}
	}
	if len(normalizeBlock(output, false)) == 0 {
		discord.Errorf("Subtitle contains no dialogues")
		return false
	}
	for _, line := range output {
		if strings.TrimSpace(line) == "" {
			continue
		}
		startTimeStr := extractDialogueField(line, start, false)
		endTimeStr := extractDialogueField(line, end, false)
		startTime, err1 := time.Parse(ASSTimeFormat, startTimeStr)
		endTime, err2 := time.Parse(ASSTimeFormat, endTimeStr)
		if err1 != nil || err2 != nil {
			// time is malformed
			discord.Errorf("Subtitle is malformed: %s", line)
			return false
		}
		duration := endTime.Sub(startTime)
		if duration > 2*time.Minute {
			// subtitle sticks
			discord.Errorf("Subtitle duration is too long: %s, %+v", line, duration)
			return false
		}
	}
	return true
}

func isASSFileValid(filePath string) error {
	// Read the content of the .ass file
	content, err := os.ReadFile(filePath)
	if err != nil {
		return err
	}
	// Call the Validate function with the file content
	headers, dialogue, err := sanitizeInputASS(string(content))
	if err != nil {
		return err
	}
	valid := isASSOutputValid(headers, strings.Split(dialogue, "\n"))
	if !valid {
		fmt.Printf("%s is invalid\n", filePath)
	}
	return nil
}
