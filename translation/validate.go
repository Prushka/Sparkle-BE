package translation

import (
	"Sparkle/discord"
	"fmt"
	"os"
	"strings"
	"time"
)

const ASSTimeFormat = "15:04:05.00"

func correctTimestamps(headers string, input, output []string) []string {

}

func isASSOutputValid(headers string, output []string) bool {
	pos, err := findFormatPositions(headers)
	if err != nil {
		discord.Errorf("Unable to process format line: %+v", err)
		return false
	}
	normalizedOutput := normalizeBlock(output, false)
	if len(normalizedOutput) == 0 {
		discord.Errorf("Subtitle contains no dialogues")
		return false
	}
	for _, line := range normalizedOutput {
		startTimeStr := extractDialogueField(line, pos.Start, false)
		endTimeStr := extractDialogueField(line, pos.End, false)
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
