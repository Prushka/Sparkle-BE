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
	discord.Infof("Sending to ChatGPT: %s", input)
	ctx := context.Background()
	msgs := []openai.ChatCompletionMessageParamUnion{
		openai.SystemMessage(`Role: You are an intelligent WEBVTT subtitle translator.
Input: WEBVTT(s) containing subtitles in one or two non‑Chinese languages.
Task:
1. Preserve every original timing cue exactly.
2. Replace each subtitle line with a context‑aware Chinese translation.
Output: A single, valid WEBVTT and nothing else, formatted identically to the input except that all subtitle text is now in Chinese.`),
		openai.UserMessage(input),
	}
	resp, err := OpenAICli.Chat.Completions.New(ctx, openai.ChatCompletionNewParams{
		Model:    "o3-mini",
		Messages: msgs,
	})
	if err != nil {
		return "", err
	}

	translated := resp.Choices[0].Message.Content
	return translated, nil
}
