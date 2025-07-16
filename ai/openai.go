package ai

import (
	"Sparkle/config"
	"Sparkle/discord"
	"Sparkle/utils"
	"context"
	"fmt"
	"github.com/openai/openai-go"
)

type openaiTranslator struct {
	messages []openai.ChatCompletionMessageParamUnion
}

type openaiResponse struct {
	Response *openai.ChatCompletion
}

func NewOpenAI() Translator {
	return &openaiTranslator{
		messages: make([]openai.ChatCompletionMessageParamUnion, 0),
	}
}

func (r *openaiResponse) Usage() interface{} {
	return r.Response.Usage
}

func (r *openaiResponse) Text() string {
	if len(r.Response.Choices) == 0 {
		return ""
	}
	return r.Response.Choices[0].Message.Content
}

func (o *openaiTranslator) StartChat(_ context.Context, systemInstruction string) error {
	o.messages = []openai.ChatCompletionMessageParamUnion{
		openai.SystemMessage(systemInstruction),
	}
	return nil
}

func (o *openaiTranslator) Send(ctx context.Context, input string) (Result, error) {
	discord.Infof("Sending to OpenAI %s", config.TheConfig.OpenAIModel)

	if len(o.messages) == 0 {
		return nil, fmt.Errorf("chat not started, call StartChat first")
	}

	// Add a user message to the conversation history
	o.messages = append(o.messages, openai.UserMessage(input))

	resp, err := OpenAICli.Chat.Completions.New(ctx, openai.ChatCompletionNewParams{
		Model:    config.TheConfig.OpenAIModel,
		Messages: o.messages,
	})
	if err != nil {
		return nil, err
	}

	if len(resp.Choices) == 0 {
		return nil, fmt.Errorf("no choices found in response")
	}

	// Add assistant response to conversation history
	o.messages = append(o.messages, openai.AssistantMessage(resp.Choices[0].Message.Content))

	result := &openaiResponse{Response: resp}
	fmt.Printf("%v\n", utils.AsJson(result.Usage()))
	return result, nil
}
