package translation

import (
	"Sparkle/discord"
	"fmt"
	"os"
	"strings"
	"time"

	mapset "github.com/deckarep/golang-set/v2"
)

const ASSTimeFormat = "15:04:05.00"

func correctTimestamps(headers, inputStr, outputStr string) []string {
	input := normalizeBlock(strings.Split(inputStr, "\n"), false)
	output := normalizeBlock(strings.Split(outputStr, "\n"), false)

	pos, err := findFormatPositions(headers)
	if err != nil {
		discord.Errorf("Unable to process format line when correcting timestamps: %+v", err)
		return output
	}
	inputStarts := make(map[string]mapset.Set[string])
	inputEnds := make(map[string]mapset.Set[string])
	for _, line := range input {
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

	for i, line := range output {
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
			output[i] = strings.ReplaceAll(line, startTimeStr, corrStartTime.ToSlice()[0])
			discord.Infof("Corrected: %s -> %s", line, output[i])
		} else if err2 != nil { // only end time malformed
			corrEndTime, ok := inputStarts[startTimeStr]
			if !ok || corrEndTime == nil || corrEndTime.Cardinality() != 1 {
				discord.Errorf("Malformed end time but can't find correct end time: %s, %+v", line, corrEndTime)
				continue
			}
			output[i] = strings.ReplaceAll(line, endTimeStr, corrEndTime.ToSlice()[0])
			discord.Infof("Corrected: %s -> %s", line, output[i])
		}
	}
	return output
}

func matchStartEndTimes(headers string, input []string, output []string) bool {
	if len(input) != len(output) {
		discord.Errorf("Subtitle line count mismatch with input: expected %d, got %d", len(input), len(output))
		return false
	}
	pos, _ := findFormatPositions(headers)
	for i := range output {
		inputLine := input[i]
		outputLine := output[i]
		inputStartTimeStr := extractDialogueField(inputLine, pos.Start, false)
		inputEndTimeStr := extractDialogueField(inputLine, pos.End, false)
		_, err1 := time.Parse(ASSTimeFormat, inputStartTimeStr)
		_, err2 := time.Parse(ASSTimeFormat, inputEndTimeStr)
		if err1 != nil || err2 != nil {
			discord.Errorf("Input subtitle time is malformed: %s", inputLine)
			return false
		}

		outputStartTimeStr := extractDialogueField(outputLine, pos.Start, false)
		outputEndTimeStr := extractDialogueField(outputLine, pos.End, false)
		_, err1 = time.Parse(ASSTimeFormat, outputStartTimeStr)
		_, err2 = time.Parse(ASSTimeFormat, outputEndTimeStr)
		if err1 != nil || err2 != nil {
			discord.Errorf("Output subtitle time is malformed: %s", outputLine)
			return false
		}
		if inputStartTimeStr != outputStartTimeStr {
			discord.Errorf("Subtitle start time mismatch with input: expected %s, got %s", inputStartTimeStr, outputStartTimeStr)
			return false
		}
		if inputEndTimeStr != outputEndTimeStr {
			discord.Errorf("Subtitle end time mismatch with input: expected %s, got %s", inputEndTimeStr, outputEndTimeStr)
			return false
		}
	}
	return true
}

func isASSOutputValid(headers string, input []string, output []string) bool {
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
		commas := strings.Count(line, ",")
		if commas < pos.TotalCommas {
			discord.Errorf("Subtitle contains less commas than format line: %s, expected: %d, got: %d",
				line, pos.TotalCommas, commas)
			return false
		}
		textStr := strings.TrimSpace(extractDialogueField(line, pos.Text, true))
		if len(textStr) == 0 {
			discord.Errorf("Subtitle dialogue line has no text: %s", line)
			return false
		}
	}
	if len(input) > 0 {
		if !matchStartEndTimes(headers, input, normalizedOutput) {
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
	dialogueLines := len(normalizeBlock(strings.Split(dialogue, "\n"), false))
	if dialogueLines < 2 {
		fmt.Printf("subtitle doesn't contain any dialogue (%d lines): %s\n", dialogueLines, filePath)
		return nil
	}
	valid := isASSOutputValid(headers, nil, strings.Split(dialogue, "\n"))
	if !valid {
		fmt.Printf("%s is invalid\n", filePath)
	}
	return nil
}
