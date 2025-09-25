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

func findInputLang(languages map[string]*ASSSubtitle) (*ASSSubtitle, string) {
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
	return nil, ""
}

func Translate(media, inputDir, mediaFile, dest, languageWithCode string, convertToVTT bool) error {
	ss := strings.Split(languageWithCode, "/")
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
	languages := make(map[string]*ASSSubtitle)
	for _, file := range files {
		if strings.HasSuffix(file.Name(), ".ass") && strings.Contains(file.Name(), "-") {
			var lang string
			source := filepath.Join(inputDir, file.Name())
			if len(file.Name()) >= 7 {
				lang = strings.ToLower(file.Name()[len(file.Name())-7 : len(file.Name())-4])
				if lang == strings.ToLower(languageCode) {
					discord.Infof("SKIPPING: Subtitle with language %s already exists: %s",
						language,
						dest)
					_, err = utils.CopyFile(source, dest)
					return err
				}
			} else {
				// when language is unknown, subtitle becomes format: 3-.ext
				lang = "unknown"
				discord.Infof("Subtitle with unknown language, proceeding: %s", source)
			}
			fBytes, err := os.ReadFile(source)
			if err != nil {
				discord.Errorf("Error reading file: %v", err)
				continue
			}
			subtitles := string(fBytes)
			sub, err := sanitizeInputASS(subtitles)
			if err != nil {
				discord.Errorf("Error sanitizing input ass: %v", err)
				continue
			}
			fLines := strings.Split(subtitles, "\n")
			if prev, ok := langLengths[lang]; !ok || prev < len(fLines) {
				langLengths[lang] = len(fLines)
				languages[lang] = sub
			}
		}
	}
	discord.Infof("%v", langLengths)
	if len(languages) == 0 {
		return fmt.Errorf("unable to find any .ass subtitle")
	}
	chosenSub, chosenLanguage := findInputLang(languages)
	var translated string
	translated, err = TranslateSubtitlesASS(chosenSub,
		language, config.GetSystemMessage(chosenLanguage, language, media))
	if err != nil {
		return err
	}
	translated = chosenSub.sanitizeOutput(translated)

	err = os.WriteFile(dest, []byte(translated), 0755)
	if err != nil {
		return err
	}

	if convertToVTT {
		err = AssToVTT(dest)
		if err != nil {
			return err
		}
	}
	return nil
}

func TranslateSubtitlesASS(sub *ASSSubtitle, language, systemMessage string) (string, error) {
	discord.Infof("[ASS] Translating to language: %s", language)

	ctx := context.Background()
	inputsPairs := splitByCharacters(sub.distilledDialogues, config.TheConfig.TranslationBatchLength)
	translated, err := ai.SendWithRetrySplit(ctx, systemMessage, inputsPairs,
		func(inputPairSlice utils.PairSlice[string, int], output string) (string, error) {
			t := strings.Split(output, "\n")
			outputLinesCount := len(t)
			discord.Infof("Output length: %d, Output lines: %d, Input lines: %d",
				len(strings.Join(t, "\n")),
				outputLinesCount, len(inputPairSlice))
			post, err := sub.process(inputPairSlice, t)
			if err != nil {
				return "", err
			}
			return post, nil
		})
	if err != nil {
		return "", err
	}
	if len(translated) == 0 {
		return "", fmt.Errorf("unable to find any translation results")
	}
	return strings.Join(translated, "\n"), nil
}
