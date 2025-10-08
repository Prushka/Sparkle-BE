package translation

import (
	"Sparkle/discord"
	"Sparkle/utils"
	"fmt"
	"os"
	"strings"
	"time"
)

func (sub *ASSSubtitle) process(inputPairSlice utils.PairSlice[string, int], out []string) (string, error) {
	output := utils.RemoveEmptyLinesAndTrimSpaces(out)
	if len(output) == 0 {
		return "", fmt.Errorf("subtitle contains no dialogues")
	}
	if len(inputPairSlice) != len(output) {
		return "", fmt.Errorf("subtitle line count mismatch with input: expected %d, got %d", len(inputPairSlice), len(output))
	}
	res := make([]string, len(output))
	for i := range output {
		inputLinePair := inputPairSlice[i]
		inputLine := inputLinePair.Left
		outputLine := output[i]
		inputParts := strings.SplitN(inputLine, ",", 3)
		outputParts := strings.SplitN(outputLine, ",", 3)
		if len(inputParts) != 3 || len(outputParts) != 3 {
			return "", fmt.Errorf("subtitle line has less commas than expected, input: %s, output: %s", inputLine, outputLine)
		}
		inputStartTimeStr := inputParts[0]
		inputEndTimeStr := inputParts[1]
		_, err1 := time.Parse(utils.ASSTimeFormat, inputStartTimeStr)
		_, err2 := time.Parse(utils.ASSTimeFormat, inputEndTimeStr)
		if err1 != nil || err2 != nil {
			return "", fmt.Errorf("input subtitle time is malformed: %s", inputLine)
		}
		outputStartTimeStr := outputParts[0]
		outputEndTimeStr := outputParts[1]
		outputTextStr := strings.TrimSpace(outputParts[2])
		if len(outputTextStr) == 0 {
			return "", fmt.Errorf("subtitle dialogue line has no text: %s", outputLine)
		}
		_, err1 = time.Parse(utils.ASSTimeFormat, outputStartTimeStr)
		_, err2 = time.Parse(utils.ASSTimeFormat, outputEndTimeStr)
		if err1 != nil || err2 != nil {
			return "", fmt.Errorf("output subtitle time is malformed: %s", outputLine)
		}
		if inputStartTimeStr != outputStartTimeStr {
			discord.Errorf("%s", inputLine)
			discord.Errorf("%s", outputLine)
			return "", fmt.Errorf("subtitle start time mismatch with input: expected %s, got %s", inputStartTimeStr, outputStartTimeStr)
		}
		if inputEndTimeStr != outputEndTimeStr {
			discord.Errorf("%s", inputLine)
			discord.Errorf("%s", outputLine)
			return "", fmt.Errorf("subtitle end time mismatch with input: expected %s, got %s", inputEndTimeStr, outputEndTimeStr)
		}

		oriInput := sub.dialogues[inputLinePair.Right]
		inputLineSplit := strings.Split(oriInput, ",")
		if len(inputLineSplit) <= sub.pos.Text {
			return "", fmt.Errorf("unable to find text field in input line: %s", inputLine)
		}
		res[i] = strings.Join(append(inputLineSplit[:sub.pos.Text], outputTextStr), ",")
	}
	return strings.Join(res, "\n"), nil
}

func isASSFileValid(filePath string) error {
	// Read the content of the .ass file
	content, err := os.ReadFile(filePath)
	if err != nil {
		return err
	}
	sub, err := sanitizeInputASS(string(content))
	if err != nil {
		return err
	}
	if len(sub.dialogues) < 2 {
		fmt.Printf("subtitle doesn't contain any dialogue (%d lines): %s\n", len(sub.dialogues), filePath)
		return nil
	}
	normalizedOutput := utils.RemoveEmptyLinesAndTrimSpaces(sub.dialogues)
	if len(normalizedOutput) == 0 {
		return fmt.Errorf("subtitle contains no dialogues")
	}
	for _, line := range normalizedOutput {
		commas := strings.Count(line, ",")
		if commas < sub.pos.TotalCommas {
			return fmt.Errorf("subtitle contains less commas than format line: %s, expected: %d, got: %d",
				line, sub.pos.TotalCommas, commas)
		}
		textStr := strings.TrimSpace(extractDialogueField(line, sub.pos.Text, true))
		if len(textStr) == 0 {
			return fmt.Errorf("subtitle dialogue line has no text: %s", line)
		}
	}
	return nil
}
