package ai

import (
	"Sparkle/discord"
	"Sparkle/utils"
	"context"
	"fmt"
)

type Translator interface {
	StartChat(ctx context.Context, systemInstruction string) error
	Send(ctx context.Context, input string) (Result, error)
}

type Result interface {
	Usage() interface{}
	Text() string
	Response() interface{}
}

func SendWithRetry(ctx context.Context, translator Translator, input string, pass func(result Result) bool, attempts int) (Result, error) {
	var err error
	for i := 1; i < attempts+1; i++ {
		discord.Infof("Attempt: %d", i)
		result, err := translator.Send(ctx, input)
		if err != nil {
			discord.Errorf("Error on attempt %d: %v", i, err)
			fmt.Println(utils.AsJson(result.Response()))
		}
		if err == nil && pass(result) {
			return result, nil
		}
	}
	return nil, fmt.Errorf("failed after %d attempts | %v", attempts, err)
}
