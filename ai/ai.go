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

type AI interface {
	StartChat(ctx context.Context, systemInstruction string) error
	Send(ctx context.Context, input string) (Result, error)
}

type Result interface {
	Usage() interface{}
	Text() string
	Response() interface{}
}

var OpenAICli openai.Client
var GeminiClis []*genai.Client

func Init() {
	discord.Infof("Initializing AI clients")
	if config.TheConfig.OpenAI != "" {
		discord.Infof("Initializing OpenAI")
		OpenAICli = openai.NewClient(
			option.WithAPIKey(config.TheConfig.OpenAI),
		)
	}
	if len(config.TheConfig.Gemini) > 0 {
		for _, g := range config.TheConfig.Gemini {
			discord.Infof("Initializing Gemini")
			ctx := context.Background()
			cli, err := genai.NewClient(ctx, &genai.ClientConfig{
				APIKey: g,
			})
			if err != nil {
				discord.Errorf("Unable to initialize gemini: %v", err)
				continue
			}
			GeminiClis = append(GeminiClis, cli)
		}
	}
}

func limit(input []string, limit int) error {
	if len(input) > limit {
		return fmt.Errorf("too many split segments")
	}
	return nil
}

func SendWithRetrySplit(ctx context.Context, systemMessage string,
	inputs []string, pass func(input string, result Result) bool, timelinesCounter func(input string) int,
	postProcessor func(input string) string) ([]string, error) {
	err := limit(inputs, 15)
	if err != nil {
		return nil, err
	}

	run := func(a AI) ([]string, error) {
		var translated []string

		err = a.StartChat(ctx, systemMessage)
		if err != nil {
			return nil, err
		}
		for idx, input := range inputs {
			inputLines := timelinesCounter(input)
			discord.Infof("Processing index: %d/%d, Input length: %d, Input timelines: %d",
				idx, len(inputs)-1, len(input), inputLines)
			result, err := SendWithRetry(ctx, a, input, pass)
			if err != nil || result == nil {
				return nil, err
			}
			translated = append(translated, postProcessor(result.Text()))
		}
		return translated, nil
	}

	for i, cli := range GeminiClis {
		res, err := run(NewGemini(cli))
		if err == nil {
			return res, nil
		}
		discord.Errorf("Cli %d failed with error: %+v", i, err)
	}
	return nil, err
}

func SendWithRetry(ctx context.Context, a AI, input string, pass func(input string, result Result) bool) (Result, error) {
	var err error
	var attempted []Result
	attempts := config.TheConfig.TranslationAttempts
	for i := 1; i < attempts+1; i++ {
		discord.Infof("Attempt: %d", i)
		result, err := a.Send(ctx, input)
		if err != nil {
			discord.Errorf("Error on attempt %d: %v", i, err)
			if result != nil && result.Response() != nil {
				fmt.Println(utils.AsJson(result.Response()))
			}
			if strings.Contains(err.Error(), "RESOURCE_EXHAUSTED") {
				return result, err
			}
		} else {
			attempted = append(attempted, result)
			if pass(input, result) {
				return result, nil
			}
		}
	}
	longest := 0
	var longestResult Result
	for _, a := range attempted {
		if len(a.Text()) > longest {
			longest = len(a.Text())
			longestResult = a
		}
	}
	return longestResult, fmt.Errorf("failed after %d attempts | %v", attempts, err)
}
