package ai

import (
	"Sparkle/config"
	"Sparkle/discord"
	"context"
	"fmt"
	"google.golang.org/genai"
)

type Translator interface {
	StartChat(ctx context.Context, systemInstruction string) error
	Send(ctx context.Context, input string) (Result, error)
	SendWithRetry(ctx context.Context, input string, pass func(result Result) bool, attempts int) (Result, error)
}

type Result interface {
	Usage() interface{}
	Text() string
}

type Gemini struct {
	Chat *genai.Chat
}

type GeminiResponse struct {
	Response *genai.GenerateContentResponse
}

func (g *GeminiResponse) Usage() interface{} {
	if g.Response == nil {
		return nil
	}
	return g.Response.UsageMetadata
}

func (g *GeminiResponse) Text() string {
	if g.Response == nil || len(g.Response.Candidates) == 0 || len(g.Response.Candidates[0].Content.Parts) == 0 {
		return ""
	}
	return g.Response.Candidates[0].Content.Parts[0].Text
}

func (g Gemini) SendWithRetry(ctx context.Context, input string, pass func(result Result) bool, attempts int) (Result, error) {
	var err error
	for i := 1; i < attempts+1; i++ {
		discord.Infof("Attempt: %d", i)
		result, err := g.Send(ctx, input)
		if err != nil {
			discord.Errorf("Error on attempt %d: %v", i, err)
		}
		if err == nil && pass(result) {
			return result, nil
		}
	}
	return nil, fmt.Errorf("failed after %d attempts | %v", attempts, err)
}

func (g Gemini) Send(ctx context.Context, input string) (Result, error) {
	if g.Chat == nil {
		return nil, fmt.Errorf("chat not started, call StartChat first")
	}
	resp, err := g.Chat.SendMessage(ctx, genai.Part{Text: input})
	if resp == nil || len(resp.Candidates) == 0 || len(resp.Candidates[0].Content.Parts) == 0 {
		return nil, fmt.Errorf("no candidates found in response")
	}
	return &GeminiResponse{Response: resp}, err
}

func (g Gemini) StartChat(ctx context.Context, systemInstruction string) error {
	chat, err := GeminiCli.Chats.Create(ctx, config.TheConfig.GeminiModel, &genai.GenerateContentConfig{
		SystemInstruction: genai.NewContentFromText(systemInstruction, genai.RoleUser)},
		[]*genai.Content{})
	g.Chat = chat
	return err
}
