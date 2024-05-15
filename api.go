package main

import (
	"Sparkle/cleanup"
	"encoding/json"
	"github.com/go-co-op/gocron"
	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
	log "github.com/sirupsen/logrus"
	"golang.org/x/net/websocket"
	"io"
	"math"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
)

var scheduler = gocron.NewScheduler(time.Now().Location())
var wss = make(map[string]*Room)
var e *echo.Echo
var wssMutex sync.RWMutex

const (
	NewPlayer         = "new player"
	NameSync          = "name"
	TimeSync          = "time"
	PauseSync         = "pause"
	ChatSync          = "chat"
	FullSync          = "full"
	PlayersStatusSync = "players"
	PfpSync           = "pfp"
)

type Room struct {
	Players map[string]*Player
	mutex   sync.RWMutex
	id      string
	Chats   []*Chat `json:"chats"`
	VideoState
}

type Player struct {
	ws    *websocket.Conn
	Name  string `json:"name"`
	Id    string `json:"id"`
	mutex sync.RWMutex
	VideoState
}

type VideoState struct {
	Time   float64 `json:"time"`
	Paused bool    `json:"paused"`
}

type PlayerPayload struct {
	Type   string   `json:"type"`
	Time   *float64 `json:"time"`
	Name   string   `json:"name"`
	Paused *bool    `json:"paused"`
	Chat   string   `json:"chat"`
}

type SendPayload struct {
	Type    string   `json:"type"`
	Time    *float64 `json:"time"`
	Paused  *bool    `json:"paused"`
	FiredBy *Player  `json:"firedBy"`
	Chats   []*Chat  `json:"chats"`
	Players []Player `json:"players"`
}

func (room *Room) syncChatsToPlayerUnsafe(player *Player) {
	player.Send(SendPayload{Type: ChatSync, Chats: room.Chats})
}

func defaultVideoState() VideoState {
	return VideoState{Time: 0, Paused: true}
}

func (player *Player) Send(message interface{}) {
	messageStr := ""
	switch message.(type) {
	case string:
		messageStr = message.(string)
	default:
		messageBytes, err := json.Marshal(message)
		if err != nil {
			log.Error(err)
			return
		}
		messageStr = string(messageBytes)
	}
	err := websocket.Message.Send(player.ws, messageStr)
	if err != nil {
		log.Error(err)
		return
	}
}

func (player *Player) Sync(time *float64, paused *bool, firedBy *Player) {
	if time != nil && *time != player.Time {
		//if player.Name == "" {
		//	return
		//}
		player.Send(SendPayload{Type: TimeSync, Time: time, FiredBy: firedBy})
	}
	if paused != nil && *paused != player.Paused {
		player.Send(SendPayload{Type: PauseSync, Paused: paused, FiredBy: firedBy})
	}
}

func REST() {
	scheduler.Every(1).Second().Do(
		func() {
			wssMutex.RLock()
			defer wssMutex.RUnlock()
			for _, room := range wss {
				playersStatusListSorted := make([]Player, 0)
				room.mutex.RLock()
				for _, player := range room.Players {
					if player.Name == "" {
						continue
					}
					player.mutex.RLock()
					playersStatusListSorted = append(playersStatusListSorted, Player{Name: player.Name, Id: player.Id, VideoState: player.VideoState})
					player.mutex.RUnlock()
				}
				room.mutex.RUnlock()
				sort.Slice(playersStatusListSorted, func(i, j int) bool {
					if playersStatusListSorted[i].Name == playersStatusListSorted[j].Name {
						return playersStatusListSorted[i].Id < playersStatusListSorted[j].Id
					}
					return playersStatusListSorted[i].Name < playersStatusListSorted[j].Name
				})
				playersStatusListSortedStr, err := json.Marshal(SendPayload{Type: PlayersStatusSync, Players: playersStatusListSorted})
				if err != nil {
					log.Error(err)
					return
				}
				room.mutex.RLock()
				for _, player := range room.Players {
					player.Send(string(playersStatusListSortedStr))
				}
				room.mutex.RUnlock()
			}
		})
	scheduler.StartAsync()
	e = echo.New()
	e.Use(middleware.CORSWithConfig(middleware.CORSConfig{
		AllowOrigins:     []string{"*"},
		AllowCredentials: true,
	}), middleware.GzipWithConfig(middleware.DefaultGzipConfig), middleware.Logger(), middleware.Recover())
	routes()
	cleanup.AddOnStopFunc(cleanup.Echo, func(_ os.Signal) {
		err := e.Close()
		if err != nil {
			return
		}
	})
	e.Logger.Fatal(e.Start(":1323"))
}

func routes() {
	e.GET("/all", func(c echo.Context) error {
		files, err := os.ReadDir(TheConfig.Output)
		if err != nil {
			return err
		}
		jobs := make([]*Job, 0)
		for _, file := range files {
			content, err := os.ReadFile(filepath.Join(TheConfig.Output, file.Name(), JobFile))
			if err != nil {
				continue
			}
			job := &Job{}
			err = json.Unmarshal(content, job)
			if err != nil {
				return err
			}
			if job.State == Complete {
				jobs = append(jobs, job)
			}
		}
		return c.JSON(http.StatusOK, jobs)
	})
	e.POST("/pfp/:id", func(c echo.Context) error {
		id := c.Param("id")
		file, err := c.FormFile("pfp")
		if err != nil {
			log.Errorf("error getting file: %v", err)
			return err
		}
		src, err := file.Open()
		if err != nil {
			log.Errorf("error opening file: %v", err)
			return err
		}
		defer func() {
			err := src.Close()
			if err != nil {
				log.Error(err)
			}
		}()
		err = os.MkdirAll(TheConfig.Output+"/pfp", 0755)
		if err != nil {
			return err
		}
		dst, err := os.Create(TheConfig.Output + "/pfp/" + id + ".png")
		if err != nil {
			return err
		}
		defer func(dst *os.File) {
			err := dst.Close()
			if err != nil {
				log.Error(err)
			}
		}(dst)
		if _, err = io.Copy(dst, src); err != nil {
			return err
		}
		wssMutex.RLock()
		for _, room := range wss {
			room.mutex.RLock()
			if firedBy, ok := room.Players[id]; ok {
				firedBy.mutex.RLock()
				payload := SendPayload{Type: PfpSync, FiredBy: firedBy}
				payloadStr, err := json.Marshal(payload)
				if err != nil {
					log.Error(err)
					return err
				}
				firedBy.mutex.RUnlock()
				for _, player := range room.Players {
					player.Send(payloadStr)
				}
			}
			room.mutex.RUnlock()
		}
		wssMutex.RUnlock()
		return c.String(http.StatusOK, "File uploaded")
	})

	e.GET("/sync/:room/:id", func(c echo.Context) error {
		room := c.Param("room")
		id := c.Param("id")
		websocket.Handler(func(ws *websocket.Conn) {
			defer func(ws *websocket.Conn) {
				err := ws.Close()
				if err != nil {
					c.Logger().Error(err)
				}
				wssMutex.Lock()
				room := wss[room]
				if room != nil {
					room.mutex.Lock()
					delete(room.Players, id)
					if len(room.Players) == 0 {
						room.VideoState = defaultVideoState()
					}
					room.mutex.Unlock()
				}
				wssMutex.Unlock()
			}(ws)
			currentPlayer := &Player{ws: ws, Id: id}
			wssMutex.Lock()
			if wss[room] == nil {
				wss[room] = &Room{Players: make(map[string]*Player), id: id,
					VideoState: defaultVideoState(), Chats: make([]*Chat, 0)}
			}
			room := wss[room]
			wssMutex.Unlock()
			room.mutex.Lock()
			room.Players[id] = currentPlayer
			room.mutex.Unlock()
			for {
				msg := ""
				err := websocket.Message.Receive(ws, &msg)
				if err != nil {
					log.Error(err)
					return
				}
				payload := &PlayerPayload{}
				err = json.Unmarshal([]byte(msg), payload)
				if err != nil {
					log.Error(err)
					return
				}
				currentPlayer.mutex.Lock()
				switch payload.Type {
				case NameSync:
					currentPlayer.Name = payload.Name
					room.mutex.Lock()
					for _, chat := range room.Chats {
						if chat.Uid == currentPlayer.Id {
							chat.Username = currentPlayer.Name
						}
					}
					for _, player := range room.Players {
						room.syncChatsToPlayerUnsafe(player)
					}
					room.mutex.Unlock()
				case ChatSync:
					if strings.TrimSpace(payload.Chat) == "" {
						continue
					}
					room.mutex.Lock()
					room.Chats = append(room.Chats, &Chat{Username: currentPlayer.Name, Message: payload.Chat,
						Uid:       currentPlayer.Id,
						Timestamp: time.Now().Unix(), MediaSec: currentPlayer.Time})
					for _, player := range room.Players {
						room.syncChatsToPlayerUnsafe(player)
					}
					room.mutex.Unlock()
					go func() {
						DiscordWebhook(FormatSecondsToTime(currentPlayer.Time)+": "+payload.Chat, currentPlayer.Name, currentPlayer.Id)
					}()
				case TimeSync:
					currentPlayer.Time = *payload.Time
					room.mutex.Lock()
					if math.Abs(room.Time-currentPlayer.Time) > 5 {
						log.Debugf("[%v] player time: %v, room time: %v", currentPlayer.Name, currentPlayer.Time, room.Time)
						room.Time = currentPlayer.Time
						for _, p := range room.Players {
							if currentPlayer.Id == p.Id {
								continue
							}
							p.Sync(&room.Time, nil, currentPlayer)
						}
					}
					room.mutex.Unlock()
				case PauseSync:
					currentPlayer.Paused = *payload.Paused
					room.mutex.Lock()
					if currentPlayer.Paused != room.Paused {
						log.Debugf("[%v] player paused: %v, room paused: %v", currentPlayer.Name, currentPlayer.Paused, room.Paused)
						room.Paused = currentPlayer.Paused
						for _, p := range room.Players {
							if currentPlayer.Id == p.Id {
								continue
							}
							p.Sync(nil, &room.Paused, currentPlayer)
						}
					}
					room.mutex.Unlock()
				case NewPlayer:
					room.mutex.Lock()
					realPlayers := 0
					for _, player := range room.Players {
						if player.Name != "" {
							realPlayers++
						}
					}
					if realPlayers > 1 {
						currentPlayer.Sync(&room.VideoState.Time, &room.VideoState.Paused, nil)
					} else {
						paused := false
						currentPlayer.Sync(&room.VideoState.Time, &paused, nil)
					}
					room.syncChatsToPlayerUnsafe(currentPlayer)
					room.mutex.Unlock()
				}
				currentPlayer.mutex.Unlock()
			}
		}).ServeHTTP(c.Response(), c.Request())
		return nil
	})
}
