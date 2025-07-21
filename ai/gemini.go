package ai

import (
	"Sparkle/config"
	"Sparkle/discord"
	"Sparkle/utils"
	"context"
	"fmt"
	"google.golang.org/genai"
	"strings"
	"time"
)

type gemini struct {
	Chat *genai.Chat
}

type geminiResponse struct {
	response *genai.GenerateContentResponse
}

func NewGemini() AI {
	return &gemini{}
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

func (g *gemini) StartChat(ctx context.Context, systemInstruction string) error {
	chat, err := GeminiCli.Chats.Create(ctx, config.TheConfig.GeminiModel, &genai.GenerateContentConfig{
		SystemInstruction: genai.NewContentFromText(systemInstruction, genai.RoleUser)},
		[]*genai.Content{})
	g.Chat = chat
	return err
}

func (g *gemini) Send(ctx context.Context, input string) (Result, error) {
	discord.Infof("Sending to Gemini %s", config.TheConfig.GeminiModel)

	if g.Chat == nil {
		return nil, fmt.Errorf("chat not started, call StartChat first")
	}
	resp, err := g.Chat.SendMessage(ctx, genai.Part{Text: input})
	result := &geminiResponse{response: resp}
	if err != nil {
		if strings.Contains(err.Error(), "RESOURCE_EXHAUSTED") || strings.Contains(err.Error(), "try again later") {
			discord.Errorf("Exceeded quota/rate limit, sleeping...")
			time.Sleep(15 * time.Minute)
		}
		return result, err
	}
	if resp == nil || len(resp.Candidates) == 0 || len(resp.Candidates[0].Content.Parts) == 0 {
		return result, fmt.Errorf("no candidates found in response")
	}
	fmt.Printf("%v\n", utils.AsJson(result.Usage()))
	return result, err
}
