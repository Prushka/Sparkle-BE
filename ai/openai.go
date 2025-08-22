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
}

type gptResponse struct {
	response *openai.ChatCompletion
}

func NewGPT(apiKey string) AI {
	return &gpt{
		messages: make([]openai.ChatCompletionMessageParamUnion, 0),
		client: openai.NewClient(
			option.WithAPIKey(apiKey),
		),
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
	discord.Infof("Sending to OpenAI %s", config.TheConfig.OpenAIModel)

	if len(o.messages) == 0 {
		return nil, fmt.Errorf("chat not started, call StartChat first")
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

	if result.Text() == "" {
		return result, fmt.Errorf("no choices found in response")
	}
	o.messages = append(o.messages, openai.AssistantMessage(resp.Choices[0].Message.Content))

	if result.Usage() != nil {
		fmt.Printf("%v\n", utils.AsJson(result.Usage()))
	}
	return result, nil
}
