package ai

import (
	"Sparkle/config"
	"Sparkle/discord"
	"context"
	"fmt"
	"github.com/labstack/gommon/log"
	"github.com/openai/openai-go"
	"github.com/openai/openai-go/option"
	"google.golang.org/genai"
	"strings"
)

var OpenAICli openai.Client
var GeminiCli *genai.Client

const systemMessage = `You are an intelligent WEBVTT subtitle translator.
Input: WEBVTT(s) containing subtitles in one or two non‑Chinese languages.
Task:
1. Preserve every original timing cue exactly.
2. Replace each subtitle line with a context‑aware and natural Simplified Chinese translation, except for lines or phrases with intentionally untranslated content.
3. Do not omit any lines. Translate every single line from start to end.
4. Do not add any additional 句号 at the end of each line.
Output: A single, valid, sanitized WEBVTT and nothing else, no extra notes, formatted correctly and identically to the input except that subtitle text is now in Simplified Chinese.`

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

func SanitizeSegment(input string) string {
	lines := strings.Split(input, "\n")

	var filtered []string
	for _, line := range lines {
		if strings.EqualFold(strings.TrimSpace(line), "WEBVTT") {
			filtered = append(filtered, "\n")
		} else {
			filtered = append(filtered, line)
		}
	}

	start := 0
	for start < len(filtered) && strings.TrimSpace(filtered[start]) == "" {
		start++
	}

	end := len(filtered) - 1
	for end >= start && strings.TrimSpace(filtered[end]) == "" {
		end--
	}

	if start > end {
		return ""
	}

	return strings.Join(filtered[start:end+1], "\n")
}

func TranslateSubtitlesGemini(input []string) (string, error) {
	discord.Infof("Sending to Gemini: segments: %d, length: %d", len(input), len(strings.Join(input, "\n")))
	if len(input) > 10 {
		return "", fmt.Errorf("too many splitted")
	}
	ctx := context.Background()

	chat, err := GeminiCli.Chats.Create(ctx, "gemini-2.5-pro", &genai.GenerateContentConfig{
		SystemInstruction: genai.NewContentFromText(systemMessage, genai.RoleUser)},
		[]*genai.Content{})
	if err != nil {
		return "", err
	}

	var translated []string

	for idx, i := range input {
		log.Infof("Processing index: %d/%d, length: %d", idx, len(input), len(i))
		result, err := chat.SendMessage(ctx, genai.Part{Text: i})
		if err != nil {
			return "", err
		}

		if len(result.Candidates) < 0 {
			return "", fmt.Errorf("unable to find candidate in response")
		}
		curr := SanitizeSegment(result.Candidates[0].Content.Parts[0].Text)
		translated = append(translated, curr)
	}
	return "WEBVTT\n\n" + strings.Join(translated, "\n\n"), nil
}

//
//func TranslateSubtitlesOpenAI(input []string) (string, error) {
//	discord.Infof("Sending to ChatGPT")
//	ctx := context.Background()
//	msgs := []openai.ChatCompletionMessageParamUnion{
//		openai.SystemMessage(systemMessage),
//		openai.UserMessage(input),
//	}
//	resp, err := OpenAICli.Chat.Completions.New(ctx, openai.ChatCompletionNewParams{
//		Model:    "o4-mini",
//		Messages: msgs,
//	})
//	if err != nil {
//		return "", err
//	}
//
//	translated := resp.Choices[0].Message.Content
//	return translated, nil
//}
