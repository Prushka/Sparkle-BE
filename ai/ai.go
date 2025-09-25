package ai

import (
	"Sparkle/config"
	"Sparkle/discord"
	"Sparkle/utils"
	"context"
	"fmt"
	"strings"
	"time"
)

type AI interface {
	StartChat(ctx context.Context, systemInstruction string) error
	Send(ctx context.Context, input string) (Result, error)
	GetLastExhausted() time.Time
	SetLastExhausted()
	IsLocal() bool
	ClearPreviousRun()
}

type Result interface {
	Usage() interface{}
	Text() string
	Response() interface{}
}

var OpenAIClis []AI
var GeminiClis []AI

func Init() {
	discord.Infof("Initializing AI clients")
	switch config.TheConfig.AiProvider {
	case "openai":
		if len(config.TheConfig.OpenAI) > 0 {
			discord.Infof("Initializing OpenAI %d clients", len(config.TheConfig.OpenAI))
			for _, key := range config.TheConfig.OpenAI {
				OpenAIClis = append(OpenAIClis, NewGPT(key))
			}
		} else if config.TheConfig.OpenAIUrl != "" {
			discord.Infof("No OpenAI keys found, found custom url, initializing without key for custom url")
			OpenAIClis = append(OpenAIClis, NewGPT(""))
		}
	case "gemini":
		if len(config.TheConfig.Gemini) > 0 {
			discord.Infof("Initializing Gemini %d clients", len(config.TheConfig.Gemini))
			for _, key := range config.TheConfig.Gemini {
				g, err := NewGemini(key)
				if err != nil {
					discord.Errorf("Unable to initialize gemini: %v", err)
					continue
				}
				GeminiClis = append(GeminiClis, g)
			}
		}
	}
}

func SendWithRetrySplit(ctx context.Context, systemMessage string,
	inputPairSlices []utils.PairSlice[string, int], processor func(inputPairSlice utils.PairSlice[string, int], output string) (string, error)) ([]string, error) {

	run := func(a AI) ([]string, error) {
		defaultLimit := 360000 / config.TheConfig.TranslationBatchLength
		if len(inputPairSlices) > defaultLimit {
			return nil, fmt.Errorf("too many split segments: %d/%d", len(inputPairSlices), defaultLimit)
		}

		var translated []string

		if err := a.StartChat(ctx, systemMessage); err != nil {
			return nil, err
		}
		for idx, inputSlice := range inputPairSlices {
			discord.Infof("Processing index: %d/%d",
				idx+1, len(inputPairSlices))
			result, err := SendWithRetry(ctx, a, strings.Join(inputSlice.LeftSlice(), "\n"),
				func(output string) (string, error) {
					processed, err := processor(inputSlice, output)
					return processed, err
				})
			if err != nil {
				return nil, err
			}
			translated = append(translated, result)
		}
		return translated, nil
	}

	exhausted := 0
	runners := GeminiClis
	if config.TheConfig.AiProvider == "openai" {
		runners = OpenAIClis
	}
	for i, runner := range runners {
		if time.Since(runner.GetLastExhausted()) < config.TheConfig.SleepAfterExhausted {
			exhausted++
			continue
		}
		discord.Infof("Running on client: %d", i)
		var res []string
		res, err := run(runner)
		if err == nil {
			return res, nil
		}
		discord.Errorf("Client %d failed with error: %+v", i, err)
		if isErrorExhausted(err) {
			exhausted++
			runner.SetLastExhausted()
		}
		if isErrorProhibitedContent(err) {
			discord.Errorf("Detected prohibited content, skipping...")
			return nil, err
		}
	}
	if exhausted == len(runners) {
		discord.Errorf("All clients exhausted, sleeping for %v", config.TheConfig.SleepAfterExhausted)
		time.Sleep(config.TheConfig.SleepAfterExhausted)
	}
	return nil, fmt.Errorf("all clients failed or exhausted")
}

func SendWithRetry(ctx context.Context, a AI, input string, processor func(output string) (string, error)) (string, error) {
	var err error
	attempts := config.TheConfig.TranslationAttempts
	for i := 1; i < attempts+1; i++ {
		discord.Infof("Attempt: %d", i)
		result, err := a.Send(ctx, input)
		if err != nil {
			discord.Errorf("Error on attempt %d: %v", i, err)
			if result != nil && result.Response() != nil && utils.AsJson(result.Response()) != "null" {
				fmt.Println(utils.AsJson(result.Response()))
			}
			if isErrorExhausted(err) || isErrorProhibitedContent(err) {
				return "", err
			}
		} else {
			processed, err := processor(result.Text())
			if err == nil {
				return processed, nil
			}
		}
		a.ClearPreviousRun()
	}
	return "", fmt.Errorf("failed after %d attempts | %v", attempts, err)
}
