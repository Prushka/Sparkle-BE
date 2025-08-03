package translation

import (
	"Sparkle/discord"
	"fmt"
	mapset "github.com/deckarep/golang-set/v2"
	"os"
	"strings"
	"time"
)

const ASSTimeFormat = "15:04:05.00"

func correctTimestamps(headers string, input, output []string) []string {
	pos, err := findFormatPositions(headers)
	if err != nil {
		discord.Errorf("Unable to process format line when correcting timestamps: %+v", err)
		return output
	}
	inputStarts := make(map[string]mapset.Set[string])
	inputEnds := make(map[string]mapset.Set[string])
	normalizedInput := normalizeBlock(input, false)
	for _, line := range normalizedInput {
		startTimeStr := extractDialogueField(line, pos.Start, false)
		endTimeStr := extractDialogueField(line, pos.End, false)
		_, err1 := time.Parse(ASSTimeFormat, startTimeStr)
		_, err2 := time.Parse(ASSTimeFormat, endTimeStr)
		if err1 != nil || err2 != nil {
			discord.Errorf("Input subtitle is malformed: %s", line)
			return output
		}
		if _, ok := inputStarts[startTimeStr]; !ok {
			inputStarts[startTimeStr] = mapset.NewSet[string]()
		}
		if _, ok := inputEnds[endTimeStr]; !ok {
			inputEnds[endTimeStr] = mapset.NewSet[string]()
		}
		inputStarts[startTimeStr].Add(endTimeStr)
		inputEnds[endTimeStr].Add(startTimeStr)
	}

	normalizedOutput := normalizeBlock(output, false)
	for i, line := range normalizedOutput {
		startTimeStr := extractDialogueField(line, pos.Start, false)
		endTimeStr := extractDialogueField(line, pos.End, false)
		_, err1 := time.Parse(ASSTimeFormat, startTimeStr)
		_, err2 := time.Parse(ASSTimeFormat, endTimeStr)
		if err1 != nil && err2 != nil {
			discord.Errorf("Unable to correct malformed subtitle due to malformed start and end time: %s", line)
			continue
		}
		if err1 != nil { // only start time malformed
			corrStartTime, ok := inputEnds[endTimeStr]
			if !ok || corrStartTime == nil || corrStartTime.Cardinality() != 1 {
				discord.Errorf("Malformed start time but can't find correct start time: %s, %+v", line, corrStartTime)
				continue
			}
			normalizedOutput[i] = strings.ReplaceAll(line, startTimeStr, corrStartTime.ToSlice()[0])
		} else if err2 != nil { // only end time malformed
			corrEndTime, ok := inputStarts[startTimeStr]
			if !ok || corrEndTime == nil || corrEndTime.Cardinality() != 1 {
				discord.Errorf("Malformed end time but can't find correct end time: %s, %+v", line, corrEndTime)
				continue
			}
			normalizedOutput[i] = strings.ReplaceAll(line, endTimeStr, corrEndTime.ToSlice()[0])
		}
	}
	return normalizedOutput
}

func isASSOutputValid(headers string, output []string) bool {
	pos, err := findFormatPositions(headers)
	if err != nil {
		discord.Errorf("Unable to process format line when validating ASS: %+v", err)
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
