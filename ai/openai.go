package ai

import (
	"Sparkle/config"
	"Sparkle/discord"
	"Sparkle/utils"
	"context"
	"fmt"
	"strings"
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

func NewOpenAI(apiKey string) AI {
	options := []option.RequestOption{
		option.WithAPIKey(apiKey),
	}
	if config.TheConfig.AIUrl != "" {
		options = append(options, option.WithBaseURL(config.TheConfig.AIUrl))
	}
	return &gpt{
		messages: make([]openai.ChatCompletionMessageParamUnion, 0),
		client: openai.NewClient(
			options...,
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
	t := r.response.Choices[0].Message.Content
	return utils.KeepOnlySubtitles(t)
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

func (o *gpt) StartChat(systemInstruction string) error {
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

// isErrorExhausted checks if the error is a gemini key exhausted error
func isErrorExhausted(err error) bool {
	if err == nil {
		return false
	}
	return strings.Contains(err.Error(), "RESOURCE_EXHAUSTED")
}

// isErrorProhibitedContent checks if the error is a gemini prohibited content error
func isErrorProhibitedContent(err error) bool {
	if err == nil {
		return false
	}
	return strings.Contains(err.Error(), "PROHIBITED_CONTENT")
}

// isErrorModelUnavailable checks if the error is a gemini model unavailable error
func isErrorModelUnavailable(err error) bool {
	if err == nil {
		return false
	}
	return strings.Contains(err.Error(), "try again later") || strings.Contains(err.Error(), "Service Unavailable")
}

func (o *gpt) Send(oCtx context.Context, input string) (Result, error) {
	ctx, cancel := context.WithTimeout(oCtx, time.Minute*30)
	defer cancel()
	now := time.Now()
	var err error
	defer func() {
		if isErrorExhausted(err) {
			return
		}
		utils.MakeUpSleep(now)
	}()
	discord.Infof("Sending to %s", config.TheConfig.AIModel)

	if len(o.messages) == 0 {
		return nil, fmt.Errorf("chat not started, call StartChat first")
	}

	systemMessage := o.messages[0]
	if len(o.messages)-1 > config.TheConfig.HistoryCount*2 {
		o.messages = append([]openai.ChatCompletionMessageParamUnion{systemMessage}, o.messages[len(o.messages)-config.TheConfig.HistoryCount*2:]...)
	}

	resp, err := o.client.Chat.Completions.New(ctx, openai.ChatCompletionNewParams{
		Model:    config.TheConfig.AIModel,
		Messages: append(o.messages, openai.UserMessage(input)),
	})
	result := &gptResponse{response: resp}
	if err != nil {
		if isErrorModelUnavailable(err) {
			discord.Errorf("Gemini unavaialble, sleeping..., %v", err)
			time.Sleep(5 * time.Minute)
		}
		return result, err
	}

	resultText := result.Text()
	if resultText == "" {
		err = fmt.Errorf("no candidates found in response")
		if strings.Contains(fmt.Sprintf("%s", utils.AsJson(resp)), "PROHIBITED_CONTENT") {
			err = fmt.Errorf("PROHIBITED_CONTENT")
		}
		return result, err
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
