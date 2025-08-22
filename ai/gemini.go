package ai

import (
	"Sparkle/config"
	"Sparkle/discord"
	"Sparkle/utils"
	"context"
	"fmt"
	"strings"
	"time"

	"google.golang.org/genai"
)

type gemini struct {
	chat          *genai.Chat
	client        *genai.Client
	LastExhausted time.Time
}

type geminiResponse struct {
	response *genai.GenerateContentResponse
}

func NewGemini(apiKey string) (AI, error) {
	ctx := context.Background()
	cli, err := genai.NewClient(ctx, &genai.ClientConfig{
		APIKey: apiKey,
	})
	if err != nil {
		return nil, err
	}
	return &gemini{client: cli}, nil
}

func (g *geminiResponse) Usage() interface{} {
	if g.response == nil {
		return nil
	}
	return g.response.UsageMetadata
}

func (g *geminiResponse) Text() string {
	if g.response == nil || len(g.response.Candidates) == 0 || len(g.response.Candidates[0].Content.Parts) == 0 {
		return ""
	}
	return g.response.Candidates[0].Content.Parts[0].Text
}

func (g *geminiResponse) Response() interface{} {
	return g.response
}

func (g *gemini) GetLastExhausted() time.Time {
	return g.LastExhausted
}

func (g *gemini) SetLastExhausted() {
	g.LastExhausted = time.Now()
}

func (g *gemini) StartChat(ctx context.Context, systemInstruction string) error {
	chat, err := g.client.Chats.Create(ctx, config.TheConfig.GeminiModel, &genai.GenerateContentConfig{
		SystemInstruction: genai.NewContentFromText(systemInstruction, genai.RoleUser)},
		[]*genai.Content{})
	g.chat = chat
	return err
}

func isErrorExhausted(err error) bool {
	return strings.Contains(err.Error(), "RESOURCE_EXHAUSTED")
}

func isErrorProhibitedContent(err error) bool {
	return strings.Contains(err.Error(), "PROHIBITED_CONTENT")
}

func (g *gemini) Send(ctx context.Context, input string) (Result, error) {
	discord.Infof("Sending to Gemini %s", config.TheConfig.GeminiModel)

	if g.chat == nil {
		return nil, fmt.Errorf("chat not started, call StartChat first")
	}

	resp, err := g.chat.SendMessage(ctx, genai.Part{Text: input})
	result := &geminiResponse{response: resp}
	if err != nil {
		if isErrorExhausted(err) {
			return result, err
		}
		if strings.Contains(err.Error(), "try again later") {
			discord.Errorf("Gemini unavaialble, sleeping..., %v", err)
			time.Sleep(15 * time.Minute)
		}
		return result, err
	}
	if result.Text() == "" {
		err = fmt.Errorf("no candidates found in response")
		if strings.Contains(fmt.Sprintf("%s", utils.AsJson(resp)), "PROHIBITED_CONTENT") {
			err = fmt.Errorf("PROHIBITED_CONTENT")
		}
		return result, err
	}
	if result.Usage() != nil {
		fmt.Printf("%v\n", utils.AsJson(result.Usage()))
	}
	return result, err
}
