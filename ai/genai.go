package ai

import (
	"Sparkle/config"
	"Sparkle/discord"
	"Sparkle/utils"
	"context"
	"fmt"
	"github.com/openai/openai-go"
	"github.com/openai/openai-go/option"
	"google.golang.org/genai"
	"strings"
)

var OpenAICli openai.Client
var GeminiCli *genai.Client

func Init() {
	discord.Infof("Initializing AI clients")
	if config.TheConfig.OpenAI != "" {
		discord.Infof("Initializing OpenAI")
		OpenAICli = openai.NewClient(
			option.WithAPIKey(config.TheConfig.OpenAI),
		)
	}
	if config.TheConfig.Gemini != "" {
		discord.Infof("Initializing Gemini")
		ctx := context.Background()
		var err error
		GeminiCli, err = genai.NewClient(ctx, &genai.ClientConfig{
			APIKey: config.TheConfig.Gemini,
		})
		if err != nil {
			discord.Errorf("Unable to initialize gemini: %v", err)
		}
	}
}

func limit(input []string) error {
	discord.Infof("Sending to Gemini: Chat segments: %d, Total input length: %d", len(input), len(strings.Join(input, "\n")))
	if len(input) > 10 {
		return fmt.Errorf("too many splitted")
	}
	return nil
}

// TODO: remove html tags <i></i> <b></b> ?? necessary?

func TranslateSubtitles(translator Translator, input []string, language string) (string, error) {
	err := limit(input)
	if err != nil {
		return "", err
	}

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
		result, err := SendWithRetry(ctx, translator, i, func(result Result) bool {
			t := result.Text()
			sanitized := sanitizeSegment(t)
			sanitizedTimeLines := utils.CountVTTTimeLines(sanitized)

			discord.Infof("Output length: %d, Output lines: %d, Output time lines: %d, Sanitized length: %d, Sanitized lines: %d, Sanitized time lines: %d",
				len(t),
				len(strings.Split(t, "\n")),
				utils.CountVTTTimeLines(t),
				len(sanitized),
				len(strings.Split(sanitized, "\n")),
				sanitizedTimeLines)
			return float64(sanitizedTimeLines)/float64(inputTimeLines) >= 0.98
		}, 3)
		if err != nil {
			return "", err
		}
		sanitized := sanitizeSegment(result.Text())
		translated = append(translated, sanitized)
	}
	return "WEBVTT\n\n" + strings.Join(translated, "\n\n"), nil
}
