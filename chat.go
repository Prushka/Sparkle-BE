package main

import (
	"encoding/json"
	"github.com/gtuk/discordwebhook"
	log "github.com/sirupsen/logrus"
	"golang.org/x/net/websocket"
	"os"
	"sync"
	"time"
)

var chats = make(map[string]*Chats)
var chatMapMutex sync.RWMutex

type Chat struct {
	Username  string  `json:"username"`
	Message   string  `json:"message"`
	Timestamp int64   `json:"timestamp"`
	MediaSec  float64 `json:"mediaSec"`
	Uid       string  `json:"uid"`
}

type Chats struct {
	Chats []Chat `json:"chats"`
	Mutex sync.RWMutex
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

func AddChat(room string, chat string, player *Player) {
	var roomChats *Chats
	chatMapMutex.Lock()
	if chats[room] == nil {
		chats[room] = &Chats{Chats: make([]Chat, 0)}
	}
	roomChats = chats[room]
	chatMapMutex.Unlock()

	roomChats.Mutex.Lock()
	safeTime := 0.0
	if player.state.Time != nil {
		safeTime = *player.state.Time
	}
	roomChats.Chats = append(roomChats.Chats, Chat{Username: player.state.Name, Message: chat,
		Uid:       player.id,
		Timestamp: time.Now().Unix(), MediaSec: safeTime})
	roomChats.Mutex.Unlock()
	SyncChats(room)
	DiscordWebhook(FormatSecondsToTime(*player.state.Time)+": "+chat, player.state.Name, player.id)
}

func SyncChats(room string) {
	for _, player := range wss[room] {
		var roomChats *Chats
		chatMapMutex.Lock()
		if chats[room] == nil {
			chats[room] = &Chats{Chats: make([]Chat, 0)}
		}
		roomChats = chats[room]
		chatMapMutex.Unlock()
		roomChats.Mutex.RLock()
		chatsStr, err := json.Marshal(roomChats.Chats)
		if err != nil {
			log.Error(err)
			return
		}
		roomChats.Mutex.RUnlock()
		err = websocket.Message.Send(player.ws, string(chatsStr))
		if err != nil {
			log.Error(err)
			return
		}
	}
}
