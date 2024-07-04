package main

import (
	"github.com/gtuk/discordwebhook"
	log "github.com/sirupsen/logrus"
	"os"
)

type Chat struct {
	Message   string  `json:"message"`
	Timestamp int64   `json:"timestamp"`
	MediaSec  float64 `json:"mediaSec"`
	Uid       string  `json:"uid"`
	UserId    string  `json:"userId"`
}

func DiscordWebhook(chat string, name string, id string) {
	avatarUrl := TheConfig.Host + "/static/pfp/" + id + ".png"
	_, err := os.Stat(TheConfig.Output + "/pfp/" + id + ".png")
	message := discordwebhook.Message{
		Username: &name,
		Content:  &chat,
	}
	if err == nil {
		message.AvatarUrl = &avatarUrl
	}
	err = discordwebhook.SendMessage(TheConfig.DiscordWebhook, message)
	if err != nil {
		log.Errorf("error sending message to discord: %v", err)
	}
}
