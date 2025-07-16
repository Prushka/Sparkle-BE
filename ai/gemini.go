package ai

import (
	"Sparkle/config"
	"Sparkle/utils"
	"context"
	"fmt"
	"google.golang.org/genai"
)

type gemini struct {
	Chat *genai.Chat
}

type geminiResponse struct {
	Response *genai.GenerateContentResponse
}

func NewGemini() Translator {
	return &gemini{}
}

func (g *geminiResponse) Usage() interface{} {
	if g.Response == nil {
		return nil
	}
	return g.Response.UsageMetadata
}

func (g *geminiResponse) Text() string {
	if g.Response == nil || len(g.Response.Candidates) == 0 || len(g.Response.Candidates[0].Content.Parts) == 0 {
		return ""
	}
	return g.Response.Candidates[0].Content.Parts[0].Text
}

func (g *gemini) StartChat(ctx context.Context, systemInstruction string) error {
	chat, err := GeminiCli.Chats.Create(ctx, config.TheConfig.GeminiModel, &genai.GenerateContentConfig{
		SystemInstruction: genai.NewContentFromText(systemInstruction, genai.RoleUser)},
		[]*genai.Content{})
	g.Chat = chat
	return err
}

func (g *gemini) Send(ctx context.Context, input string) (Result, error) {
	if g.Chat == nil {
		return nil, fmt.Errorf("chat not started, call StartChat first")
	}
	resp, err := g.Chat.SendMessage(ctx, genai.Part{Text: input})
	if resp == nil || len(resp.Candidates) == 0 || len(resp.Candidates[0].Content.Parts) == 0 {
		return nil, fmt.Errorf("no candidates found in response")
	}
	result := &geminiResponse{Response: resp}
	fmt.Printf("%v\n", utils.AsJson(result.Usage()))
	return result, err
}
