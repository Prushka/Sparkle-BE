package ai

import (
	"Sparkle/config"
	"Sparkle/discord"
	"Sparkle/utils"
	"context"
	"fmt"
	"time"

	"github.com/openai/openai-go"
	"github.com/openai/openai-go/option"
)

type gpt struct {
	messages      []openai.ChatCompletionMessageParamUnion
	client        openai.Client
	LastExhausted time.Time
	isLocal       bool
}

type gptResponse struct {
	response *openai.ChatCompletion
}

func NewGPT(apiKey string) AI {
	options := []option.RequestOption{
		option.WithAPIKey(apiKey),
	}
	isCustom := false
	if config.TheConfig.OpenAIUrl != "" {
		options = append(options, option.WithBaseURL(config.TheConfig.OpenAIUrl))
		isCustom = true
	}
	return &gpt{
		messages: make([]openai.ChatCompletionMessageParamUnion, 0),
		client: openai.NewClient(
			options...,
		),
		isLocal: isCustom,
	}
}

func (r *gptResponse) Usage() interface{} {
	if r.response == nil {
		return nil
	}
	return r.response.Usage
}

func (r *gptResponse) Text() string {
	if r.response == nil || len(r.response.Choices) == 0 {
		return ""
	}
	return r.response.Choices[0].Message.Content
}

func (r *gptResponse) Response() interface{} {
	return r.response
}

func (o *gpt) IsLocal() bool {
	return o.isLocal
}

func (o *gpt) GetLastExhausted() time.Time {
	return o.LastExhausted
}

func (o *gpt) SetLastExhausted() {
	o.LastExhausted = time.Now()
}

func (o *gpt) StartChat(_ context.Context, systemInstruction string) error {
	o.messages = []openai.ChatCompletionMessageParamUnion{
		openai.SystemMessage(systemInstruction),
	}
	return nil
}

func (o *gpt) Send(ctx context.Context, input string) (Result, error) {
	now := time.Now()
	defer func() {
		if !o.isLocal {
			utils.MakeUpSleep(now)
		}
	}()
	discord.Infof("Sending to OpenAI %s", config.TheConfig.OpenAIModel)

	if len(o.messages) == 0 {
		return nil, fmt.Errorf("chat not started, call StartChat first")
	}

	if config.TheConfig.NoHistory {
		o.messages = o.messages[:1]
	}

	o.messages = append(o.messages, openai.UserMessage(input))

	resp, err := o.client.Chat.Completions.New(ctx, openai.ChatCompletionNewParams{
		Model:    config.TheConfig.OpenAIModel,
		Messages: o.messages,
	})
	result := &gptResponse{response: resp}
	if err != nil {
		return result, err
	}

	resultText := result.Text()
	if resultText == "" {
		return result, fmt.Errorf("no choices found in response")
	}
	if config.TheConfig.Debug {
		fmt.Println(resultText)
	}
	o.messages = append(o.messages, openai.AssistantMessage(resultText))

	if result.Usage() != nil {
		fmt.Printf("%v\n", utils.AsJson(result.Usage()))
	}
	return result, nil
}
