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

func SendWithRetry(ctx context.Context, a AI, input string, pass func(result Result) bool, attempts int) (Result, error) {
	var err error
	var attempted []Result
	for i := 1; i < attempts+1; i++ {
		discord.Infof("Attempt: %d", i)
		result, err := a.Send(ctx, input)
		if err != nil {
			discord.Errorf("Error on attempt %d: %v", i, err)
			if result != nil && result.Response() != nil {
				fmt.Println(utils.AsJson(result.Response()))
			}
		}
		if err == nil {
			attempted = append(attempted, result)
			if pass(result) {
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
