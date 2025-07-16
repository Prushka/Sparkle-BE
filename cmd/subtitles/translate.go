package main

import (
	"Sparkle/ai"
	"Sparkle/config"
	"Sparkle/discord"
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
			if i+1 >= len(lines) || strings.Contains(lines[i+1], "-->") {
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

func translate(media, inputDir string) error {
	files, err := os.ReadDir(inputDir)
	if err != nil {
		return err
	}
	langLengths := make(map[string]int)
	langs := make(map[string]string)
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
			fLines := strings.Split(string(fBytes), "\n")
			if prev, ok := langLengths[lang]; !ok || prev < len(fLines) {
				langLengths[lang] = len(fLines)
				langs[lang] = string(fBytes)
			}
		}
	}
	discord.Infof("%v", langLengths)
	if len(langs) == 0 {
		return fmt.Errorf("unable to find any webvtt")
	}
	assembled := fmt.Sprintf("Media: %s\n", media)
	count := 0
	if eng, ok := langs["eng"]; ok {
		discord.Infof("Using language: eng")
		assembled += fmt.Sprintf("Language: %s\n%s\n", "eng", eng)
		count++
	}
	for key, value := range langs {
		if count > 0 {
			break
		}
		discord.Infof("Using language: %s", key)
		assembled += fmt.Sprintf("Language: %s\n%s\n", key, value)
		count++
	}
	translator := ai.NewGemini()
	if config.TheConfig.AiProvider == "openai" {
	}
	translated, err := ai.TranslateSubtitles(translator, splitAssembled(assembled, 1000))
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(inputDir, outputVTT), []byte(translated), 0755)
}
