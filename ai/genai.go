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

const systemMessage = `You are an intelligent WEBVTT subtitle translator.
Input: WEBVTT(s) containing subtitles in one or two non‑Chinese languages.
Task:
1. Preserve every original timing cue exactly.
2. Replace each subtitle line with a context‑aware and natural Simplified Chinese translation, except for lines or phrases with intentionally untranslated content.
3. Do NOT omit any lines. Translate every single line from start to end.
Output: A single, valid, sanitized WEBVTT as plain text and nothing else, no extra notes, no markdown, formatted correctly and identically to the input except that subtitle text is now in Simplified Chinese.`

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

func sanitizeSegment(input string) string {
	lines := strings.Split(input, "\n")

	var filtered []string
	for _, line := range lines {
		curr := strings.ToLower(strings.TrimSpace(line))
		if strings.Contains(curr, "webvtt") || strings.Contains(curr, "```") {
			continue
		}
		filtered = append(filtered, line)
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

func limit(input []string) error {
	discord.Infof("Sending to Gemini: Chat segments: %d, Total input length: %d", len(input), len(strings.Join(input, "\n")))
	if len(input) > 10 {
		return fmt.Errorf("too many splitted")
	}
	return nil
}

func countVTTTimeLines(input string) int {
	lines := strings.Split(input, "\n")
	count := 0
	for _, s := range lines {
		if strings.Contains(s, "-->") {
			count++
		}
	}
	return count
}

// TODO: sanitize output, if end of sentence contains only one 句号，remove it
// TODO: add retry (max 3 times) when time lines no match (2% cutoff)
// TODO: add translator interface to support multiple ai providers
// TODO: finish o4-mini
// TODO: sanitize input webvtt, remove time with empty content, remove duplicate entries (same time and same content), (remove html tags <i></i> <b></b> ?? necessary?)

func TranslateSubtitlesGemini(input []string) (string, error) {
	err := limit(input)
	if err != nil {
		return "", err
	}

	ctx := context.Background()
	chat, err := GeminiCli.Chats.Create(ctx, config.TheConfig.GeminiModel, &genai.GenerateContentConfig{
		SystemInstruction: genai.NewContentFromText(systemMessage, genai.RoleUser)},
		[]*genai.Content{})
	if err != nil {
		return "", err
	}

	var translated []string

	for idx, i := range input {
		discord.Infof("Processing index: %d/%d, Input length: %d, Input lines: %d, Input time lines: %d",
			idx, len(input)-1, len(i), len(strings.Split(i, "\n")), countVTTTimeLines(i))
		result, err := chat.SendMessage(ctx, genai.Part{Text: i})
		if err != nil {
			return "", err
		}

		if len(result.Candidates) < 0 {
			return "", fmt.Errorf("unable to find candidate in response")
		}
		t := result.Candidates[0].Content.Parts[0].Text
		fmt.Printf("%v\n", utils.AsJson(result.UsageMetadata))
		sanitized := sanitizeSegment(t)
		translated = append(translated, sanitized)
		discord.Infof("Output length: %d, Output lines: %d, Output time lines: %d, Sanitized length: %d, Sanitized lines: %d, Sanitized time lines: %d",
			len(t),
			len(strings.Split(t, "\n")),
			countVTTTimeLines(t),
			len(sanitized),
			len(strings.Split(sanitized, "\n")),
			countVTTTimeLines(sanitized))
	}
	return "WEBVTT\n\n" + strings.Join(translated, "\n\n"), nil
}

//func TranslateSubtitlesOpenAI(input []string) (string, error) {
//	err := limit(input)
//	if err != nil {
//		return "", err
//	}
//
//	ctx := context.Background()
//	msgs := []openai.ChatCompletionMessageParamUnion{
//		openai.SystemMessage(systemMessage),
//		openai.UserMessage(input),
//	}
//	resp, err := OpenAICli.Chat.Completions.New(ctx, openai.ChatCompletionNewParams{
//		Model:    config.TheConfig.OpenAIModel,
//		Messages: msgs,
//	})
//	if err != nil {
//		return "", err
//	}
//
//	translated := resp.Choices[0].Message.Content
//	return translated, nil
//}
