package translation

import (
	"Sparkle/ai"
	"Sparkle/config"
	"Sparkle/discord"
	"Sparkle/utils"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

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

func Translate(media, inputDir string) error {
	files, err := os.ReadDir(inputDir)
	if err != nil {
		return err
	}
	langLengths := make(map[string]int)
	languages := make(map[string]string)
	for _, file := range files {
		if strings.HasSuffix(file.Name(), ".vtt") {
			discord.Infof(file.Name())
		}
		if len(file.Name()) >= 7 {
			lang := file.Name()[len(file.Name())-7 : len(file.Name())-4]
			fBytes, err := os.ReadFile(filepath.Join(inputDir, file.Name()))
			if err != nil {
				discord.Errorf("Error reading file: %v", err)
			}
			webvtt := sanitizeWebVTT(string(fBytes))
			fLines := strings.Split(webvtt, "\n")
			if prev, ok := langLengths[lang]; !ok || prev < len(fLines) {
				langLengths[lang] = len(fLines)
				languages[lang] = webvtt
			}
		}
	}
	discord.Infof("%v", langLengths)
	if len(languages) == 0 {
		return fmt.Errorf("unable to find any webvtt")
	}
	assembled := fmt.Sprintf("Media: %s\n", media)
	count := 0
	if eng, ok := languages["eng"]; ok {
		discord.Infof("Using language: eng")
		assembled += fmt.Sprintf("Language: %s\n%s\n", "eng", eng)
		count++
	}
	for key, value := range languages {
		if count > 0 {
			break
		}
		discord.Infof("Using language: %s", key)
		assembled += fmt.Sprintf("Language: %s\n%s\n", key, value)
		count++
	}
	translator := ai.NewGemini()
	if config.TheConfig.AiProvider == "openai" {
		translator = ai.NewOpenAI()
	}
	translated, err := ai.TranslateSubtitles(translator, splitAssembled(assembled, 1000))
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(inputDir, config.GetOutputVTT("")), []byte(translated), 0755)
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
