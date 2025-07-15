package genai

import (
	"Sparkle/config"
	"Sparkle/discord"
	"context"
	"github.com/openai/openai-go"
	"github.com/openai/openai-go/option"
)

var OpenAICli openai.Client

func InitOpenAI() {
	OpenAICli = openai.NewClient(
		option.WithAPIKey(config.TheConfig.OpenAI),
	)
}

func TranslateSubtitles(input string) (string, error) {
	discord.Infof("Sending to ChatGPT")
	ctx := context.Background()
	msgs := []openai.ChatCompletionMessageParamUnion{
		openai.SystemMessage(`Role: You are an intelligent WEBVTT subtitle translator.
Input: WEBVTT(s) containing subtitles in one or two non‑Chinese languages.
Task:
1. Preserve every original timing cue exactly.
2. Replace each subtitle line with a context‑aware Simplified Chinese translation, except for lines or phrases with intentionally untranslated content.
3. Do not omit any lines. Translate every single line from start to end.
4. Do not add any additional 句号 at the end of each line.
Output: A single, valid, sanitized WEBVTT and nothing else, no extra notes, formatted correctly and identically to the input except that subtitle text is now in Simplified Chinese.`),
		openai.UserMessage(input),
	}
	resp, err := OpenAICli.Chat.Completions.New(ctx, openai.ChatCompletionNewParams{
		Model:    "o4-mini",
		Messages: msgs,
	})
	if err != nil {
		return "", err
	}

	translated := resp.Choices[0].Message.Content
	return translated, nil
}
