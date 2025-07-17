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
	response *openai.ChatCompletion
}

func NewOpenAI() AI {
	return &openaiTranslator{
		messages: make([]openai.ChatCompletionMessageParamUnion, 0),
	}
}

func (r *openaiResponse) Usage() interface{} {
	return r.response.Usage
}

func (r *openaiResponse) Text() string {
	if len(r.response.Choices) == 0 {
		return ""
	}
	return r.response.Choices[0].Message.Content
}

func (r *openaiResponse) Response() interface{} {
	return r.response
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
	result := &openaiResponse{response: resp}
	if err != nil {
		return nil, err
	}

	if len(resp.Choices) == 0 {
		return nil, fmt.Errorf("no choices found in response")
	}

	// Add assistant response to conversation history
	o.messages = append(o.messages, openai.AssistantMessage(resp.Choices[0].Message.Content))

	fmt.Printf("%v\n", utils.AsJson(result.Usage()))
	return result, nil
}
