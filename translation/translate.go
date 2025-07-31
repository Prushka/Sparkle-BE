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

func findInputLang(languages map[string]string) (string, string) {
	for _, chosenLanguage := range config.TheConfig.TranslationInputLanguage {
		if elem, ok := languages[chosenLanguage]; ok {
			discord.Infof("Using language: %s", chosenLanguage)
			return elem, chosenLanguage
		}
	}
	for key, value := range languages {
		discord.Infof("Using language: %s", key)
		return value, key
	}
	return "", ""
}

func Translate(media, inputDir, mediaFile, dest, languageWithCode, subtitleSuffix string, convertToVTT bool) error {
	ss := strings.Split(languageWithCode, ";")
	language := ss[0]
	languageCode := ss[1]

	stat, err := os.Stat(dest)
	statInput, _ := os.Stat(mediaFile)
	if err == nil && statInput.ModTime().Before(stat.ModTime()) {
		discord.Infof("SKIPPING: File already exists: %s", dest)
		return nil
	}
	files, err := os.ReadDir(inputDir)
	if err != nil {
		return err
	}
	langLengths := make(map[string]int)
	languages := make(map[string]string)
	languageHeaders := make(map[string]string)
	for _, file := range files {
		if strings.HasSuffix(file.Name(), fmt.Sprintf(".%s", subtitleSuffix)) && strings.Contains(file.Name(), "-") {
			if len(file.Name()) >= 7 {
				lang := strings.ToLower(file.Name()[len(file.Name())-7 : len(file.Name())-4])
				if lang == strings.ToLower(languageCode) {
					discord.Infof("SKIPPING: Subtitle with language %s already exists: %s",
						language,
						dest)
					return nil
				}
				fBytes, err := os.ReadFile(filepath.Join(inputDir, file.Name()))
				if err != nil {
					discord.Errorf("Error reading file: %v", err)
					continue
				}
				subtitles := string(fBytes)
				headers := ""
				if subtitleSuffix == "vtt" {
					subtitles = sanitizeInputVTT(subtitles)
				} else if subtitleSuffix == "ass" {
					headers, subtitles, err = sanitizeInputASS(subtitles)
					if err != nil {
						discord.Errorf("Error sanitizing input ass: %v", err)
						continue
					}
				}
				fLines := strings.Split(subtitles, "\n")
				if prev, ok := langLengths[lang]; !ok || prev < len(fLines) {
					langLengths[lang] = len(fLines)
					languages[lang] = subtitles
					languageHeaders[lang] = headers
				}
			}
		}
	}
	discord.Infof("%v", langLengths)
	if len(languages) == 0 {
		return fmt.Errorf("unable to find any %s subtitle", subtitleSuffix)
	}
	in, chosenLanguage := findInputLang(languages)
	translator := ai.NewGemini()
	if config.TheConfig.AiProvider == "openai" {
		translator = ai.NewOpenAI()
	}
	var translated string
	if subtitleSuffix == "vtt" {
		translated, err = TranslateSubtitlesWebVTT(translator, splitByCharacters(in, config.TheConfig.TranslationBatchLength, false),
			language, config.GetSystemMessage(chosenLanguage, language, media, config.WEBVTT))
		if err != nil {
			return err
		}
	} else if subtitleSuffix == "ass" {
		translated, err = TranslateSubtitlesASS(translator, splitByCharacters(in, config.TheConfig.TranslationBatchLength, true),
			language, config.GetSystemMessage(chosenLanguage, language, media, config.ASS))
		if err != nil {
			return err
		}
		translated = sanitizeOutputASS(languageHeaders[chosenLanguage], translated)
	} else {
		return fmt.Errorf("unknown subtitle type: %s", subtitleSuffix)
	}

	err = os.WriteFile(dest, []byte(translated), 0755)
	if err != nil {
		return err
	}

	// current subtitle is .ass, and we don't have .vtt translations to run
	if convertToVTT && subtitleSuffix == "ass" &&
		!strings.Contains(strings.Join(config.TheConfig.TranslationSubtitleTypes, ""),
			"vtt") {
		err = AssToVTT(dest)
		if err != nil {
			return err
		}
	}
	return nil
}

func limit(input []string, limit int) error {
	if len(input) > limit {
		return fmt.Errorf("too many split lines")
	}
	return nil
}

func TranslateSubtitlesASS(translator ai.AI, input []string, language, systemMessage string) (string, error) {
	err := limit(input, 20)
	if err != nil {
		return "", err
	}

	discord.Infof("[ASS] Translating to language: %s", language)

	ctx := context.Background()
	err = translator.StartChat(ctx, systemMessage)
	if err != nil {
		return "", err
	}

	var translated []string

	for idx, i := range input {
		inputLines := len(strings.Split(i, "\n"))
		discord.Infof("Processing index: %d/%d, Input length: %d, Input lines: %d",
			idx, len(input)-1, len(i), inputLines)
		result, err := ai.SendWithRetry(ctx, translator, i, func(result ai.Result) bool {
			t := result.Text()
			outputLines := len(normalizeBlock(strings.Split(t, "\n"), false))
			discord.Infof("Output length: %d, Output lines: %d",
				len(t),
				outputLines)
			return float64(outputLines)/float64(inputLines) >= config.TheConfig.TranslationOutputCutoff
		}, config.TheConfig.TranslationAttempts)
		if (!config.TheConfig.KeepTranslationAttempt && err != nil) || result == nil {
			return "", err
		}
		if err != nil {
			discord.Infof("Keeping longest translation attempt")
		}
		translated = append(translated, result.Text())
	}
	return strings.Join(translated, "\n"), nil
}

func TranslateSubtitlesWebVTT(translator ai.AI, input []string, language, systemMessage string) (string, error) {
	err := limit(input, 10)
	if err != nil {
		return "", err
	}

	discord.Infof("[WEBVTT] Translating to language: %s", language)

	ctx := context.Background()
	err = translator.StartChat(ctx, systemMessage)
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
			return float64(sanitizedTimeLines)/float64(inputTimeLines) >= config.TheConfig.TranslationOutputCutoff
		}, config.TheConfig.TranslationAttempts)
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
