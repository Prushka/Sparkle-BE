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
	isLocal  bool
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
	t := r.response.Choices[0].Message.Content
	if r.isLocal {
		return utils.KeepOnlySubtitles(t)
	}
	return t
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

func (o *gpt) ClearPreviousRun() {
	if len(o.messages) > 1 {
		// remove the last two messages (user and assistant)
		o.messages = o.messages[:len(o.messages)-2]
	}
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

	systemMessage := o.messages[0]
	if len(o.messages)-1 > config.TheConfig.HistoryCount*2 {
		o.messages = append([]openai.ChatCompletionMessageParamUnion{systemMessage}, o.messages[len(o.messages)-config.TheConfig.HistoryCount*2:]...)
	}

	resp, err := o.client.Chat.Completions.New(ctx, openai.ChatCompletionNewParams{
		Model:    config.TheConfig.OpenAIModel,
		Messages: append(o.messages, openai.UserMessage(input)),
	})
	result := &gptResponse{response: resp, isLocal: o.isLocal}
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
	o.messages = append(o.messages, openai.UserMessage(input))
	o.messages = append(o.messages, openai.AssistantMessage(resultText))

	if result.Usage() != nil {
		fmt.Printf("%v\n", utils.AsJson(result.Usage()))
	}
	return result, nil
}
