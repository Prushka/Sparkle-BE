package translation

import (
	"Sparkle/ai"
	"Sparkle/config"
	"Sparkle/discord"
	"Sparkle/utils"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

func Translate(media, inputDir, dest, language string) error {
	if _, err := os.Stat(dest); err == nil {
		discord.Infof("SKIPPING: File already exists: %s", dest)
		return nil
	}
	files, err := os.ReadDir(inputDir)
	if err != nil {
		return err
	}
	langLengths := make(map[string]int)
	languages := make(map[string]string)
	for _, file := range files {
		if strings.HasSuffix(file.Name(), ".vtt") && !strings.HasPrefix(file.Name(), "ai") {
			discord.Infof(file.Name())
			if len(file.Name()) >= 7 {
				lang := file.Name()[len(file.Name())-7 : len(file.Name())-4]
				fBytes, err := os.ReadFile(filepath.Join(inputDir, file.Name()))
				if err != nil {
					discord.Errorf("Error reading file: %v", err)
				}
				webvtt := sanitizeInputVTT(string(fBytes))
				fLines := strings.Split(webvtt, "\n")
				if prev, ok := langLengths[lang]; !ok || prev < len(fLines) {
					langLengths[lang] = len(fLines)
					languages[lang] = webvtt
				}
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
	translated, err := TranslateSubtitles(translator, splitAssembled(assembled, 1000), language)
	if err != nil {
		return err
	}
	return os.WriteFile(dest, []byte(translated), 0755)
}

func limit(input []string) error {
	if len(input) > 10 {
		return fmt.Errorf("too many splitted")
	}
	return nil
}

func TranslateSubtitles(translator ai.AI, input []string, language string) (string, error) {
	err := limit(input)
	if err != nil {
		return "", err
	}

	discord.Infof("Translating to language: %s", language)

	ctx := context.Background()
	err = translator.StartChat(ctx, config.GetSystemMessage(language))
	if err != nil {
		return "", err
	}

	var translated []string

	for idx, i := range input {
		inputTimeLines := utils.CountVTTTimeLines(i)
		discord.Infof("Processing index: %d/%d, Input length: %d, Input lines: %d, Input time lines: %d",
			idx, len(input)-1, len(i), len(strings.Split(i, "\n")), inputTimeLines)
		result, err := ai.SendWithRetry(ctx, translator, i, func(result ai.Result) bool {
			t := result.Text()
			sanitized := sanitizeOutputVTT(t)
			sanitizedTimeLines := utils.CountVTTTimeLines(sanitized)

			discord.Infof("Output length: %d, Output lines: %d, Output time lines: %d, Sanitized length: %d, Sanitized lines: %d, Sanitized time lines: %d",
				len(t),
				len(strings.Split(t, "\n")),
				utils.CountVTTTimeLines(t),
				len(sanitized),
				len(strings.Split(sanitized, "\n")),
				sanitizedTimeLines)
			return float64(sanitizedTimeLines)/float64(inputTimeLines) >= 0.98
		}, 2)
		if (!config.TheConfig.KeepTranslationAttempt && err != nil) || result == nil {
			return "", err
		}
		if err != nil {
			discord.Infof("Keeping longest translation attempt")
		}
		sanitized := sanitizeOutputVTT(result.Text())
		translated = append(translated, sanitized)
	}
	return "WEBVTT\n\n" + strings.Join(translated, "\n\n"), nil
}
