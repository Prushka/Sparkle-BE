package main

import (
	"Sparkle/config"
	"Sparkle/discord"
	"github.com/gtuk/discordwebhook"
	"os"
)

type Chat struct {
	Message   string  `json:"message"`
	Timestamp int64   `json:"timestamp"`
	MediaSec  float64 `json:"mediaSec"`
	Uid       string  `json:"uid"`
}

func DiscordWebhook(chat string, name string, id string) {
	avatarUrl := config.TheConfig.Host + "/static/pfp/" + id + ".png"
	_, err := os.Stat(config.TheConfig.Output + "/pfp/" + id + ".png")
	message := discordwebhook.Message{
		Username: &name,
		Content:  &chat,
	}
	if err == nil {
		message.AvatarUrl = &avatarUrl
	}
	err = discordwebhook.SendMessage(config.TheConfig.DiscordWebhookChat, message)
	if err != nil {
		discord.Errorf("error sending message to discord: %v", err)
	}
}
