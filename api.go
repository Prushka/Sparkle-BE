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
	StateSync         = "state"
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
	mutex sync.RWMutex
	VideoState
	PlayerState
}

type PlayerState struct {
	Name     string `json:"name"`
	Id       string `json:"id"`
	InBg     bool   `json:"inBg,omitempty"`
	LastSeen int64  `json:"lastSeen"`
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
	State  string   `json:"state"`
}

type SendPayload struct {
	Type      string   `json:"type"`
	Time      *float64 `json:"time,omitempty"`
	Paused    *bool    `json:"paused,omitempty"`
	FiredBy   *Player  `json:"firedBy,omitempty"`
	Chats     []*Chat  `json:"chats,omitempty"`
	Players   []Player `json:"players"`
	Timestamp int64    `json:"timestamp"`
}

func (room *Room) syncChatsToPlayerUnsafe(player *Player) {
	if len(room.Chats) > 0 {
		player.Send(SendPayload{Type: ChatSync, Chats: room.Chats, Timestamp: time.Now().UnixMilli()})
	}
}

func defaultVideoState() VideoState {
	return VideoState{Time: 0, Paused: true}
}

func (player *Player) Send(message interface{}) {
	messageStr := ""
	switch message.(type) {
	case string:
		messageStr = message.(string)
	case []byte:
		messageStr = string(message.([]byte))
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

func (player *Player) Sync(t *float64, paused *bool, firedBy *Player) {
	if firedBy != nil && firedBy.InBg {
		return
	}
	if t != nil {
		player.Send(SendPayload{Type: TimeSync, Time: t, FiredBy: firedBy, Timestamp: time.Now().UnixMilli()})
	}
	if paused != nil {
		player.Send(SendPayload{Type: PauseSync, Paused: paused, FiredBy: firedBy, Timestamp: time.Now().UnixMilli()})
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
					player.mutex.RLock()
					if player.Name == "" {
						player.mutex.RUnlock()
						continue
					}
					playersStatusListSorted = append(playersStatusListSorted, Player{PlayerState: player.PlayerState, VideoState: player.VideoState})
					player.mutex.RUnlock()
				}
				room.mutex.RUnlock()
				if len(playersStatusListSorted) == 0 {
					continue
				}
				sort.Slice(playersStatusListSorted, func(i, j int) bool {
					if playersStatusListSorted[i].Name == playersStatusListSorted[j].Name {
						return playersStatusListSorted[i].Id < playersStatusListSorted[j].Id
					}
					return playersStatusListSorted[i].Name < playersStatusListSorted[j].Name
				})
				playersStatusListSortedStr, err := json.Marshal(SendPayload{Type: PlayersStatusSync, Players: playersStatusListSorted, Timestamp: time.Now().UnixMilli()})
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

func populate(path string) interface{} {
	content, err := os.ReadFile(OutputJoin(path, JobFile))
	if err != nil {
		return nil
	}
	job := make(map[string]interface{})
	err = json.Unmarshal(content, &job)
	if err != nil {
		return nil
	}
	fileSizes := make(map[string]int64)
	if job["State"] == Complete {
		files, err := os.ReadDir(OutputJoin(path))
		if err != nil {
			return nil
		}
		for _, file := range files {
			stat, err := os.Stat(file.Name())
			if err == nil {
				fileSizes[file.Name()] = stat.Size()
			}
		}
		return job
	}
	return nil
}

func routes() {
	e.GET("/all", func(c echo.Context) error {
		files, err := os.ReadDir(TheConfig.Output)
		if err != nil {
			return err
		}
		jobs := make([]interface{}, 0)
		for _, file := range files {
			job := populate(file.Name())
			if job != nil {
				jobs = append(jobs, job)
			}
		}
		return c.JSON(http.StatusOK, jobs)
	})
	e.GET("/job/:id", func(c echo.Context) error {
		id := c.Param("id")
		job := populate(id)
		return c.JSON(http.StatusOK, job)
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
				payload := SendPayload{Type: PfpSync, FiredBy: firedBy, Timestamp: time.Now().UnixMilli()}
				payloadStr, err := json.Marshal(payload)
				if err != nil {
					log.Error(err)
					room.mutex.RUnlock()
					firedBy.mutex.RUnlock()
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
				log.Infof("[%v] disconnected", id)
			}(ws)
			currentPlayer := &Player{ws: ws,
				PlayerState: PlayerState{Id: id, LastSeen: time.Now().Unix()},
			}
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
			log.Infof("[%v] connected", id)
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
				func() {
					currentPlayer.mutex.Lock()
					room.mutex.Lock()
					defer currentPlayer.mutex.Unlock()
					defer room.mutex.Unlock()
					currentPlayer.LastSeen = time.Now().Unix()
					switch payload.Type {
					case StateSync:
						switch payload.State {
						case "bg":
							currentPlayer.InBg = true
						case "fg":
							currentPlayer.InBg = false
							room.syncChatsToPlayerUnsafe(currentPlayer)
						}
					case NameSync:
						currentPlayer.Name = payload.Name
						for _, chat := range room.Chats {
							if chat.Uid == currentPlayer.Id {
								chat.Username = currentPlayer.Name
							}
						}
						for _, player := range room.Players {
							room.syncChatsToPlayerUnsafe(player)
						}
					case ChatSync:
						if strings.TrimSpace(payload.Chat) == "" {
							return
						}
						room.Chats = append(room.Chats, &Chat{Username: currentPlayer.Name, Message: payload.Chat,
							Uid:       currentPlayer.Id,
							Timestamp: time.Now().UnixMilli(), MediaSec: currentPlayer.Time})
						for _, player := range room.Players {
							room.syncChatsToPlayerUnsafe(player)
						}
						go func() {
							DiscordWebhook(FormatSecondsToTime(currentPlayer.Time)+": "+payload.Chat, currentPlayer.Name, currentPlayer.Id)
						}()
					case TimeSync:
						currentPlayer.Time = *payload.Time
						if math.Abs(room.Time-currentPlayer.Time) > 5 {
							log.Debugf("[%v] player time: %v, room time: %v", currentPlayer.Name, currentPlayer.Time, room.Time)
							for _, p := range room.Players {
								if currentPlayer.Id == p.Id {
									continue
								}
								p.Sync(&currentPlayer.Time, nil, currentPlayer)
							}
						}
						room.Time = currentPlayer.Time
					case PauseSync:
						currentPlayer.Paused = *payload.Paused
						log.Debugf("[%v] player paused: %v, room paused: %v", currentPlayer.Name, currentPlayer.Paused, room.Paused)
						room.Paused = currentPlayer.Paused
						for _, p := range room.Players {
							if currentPlayer.Id == p.Id {
								continue
							}
							p.Sync(nil, &room.Paused, currentPlayer)
						}
					case NewPlayer:
						paused := false
						currentPlayer.Sync(&room.VideoState.Time, &paused, nil)
						for _, p := range room.Players {
							if currentPlayer.Id == p.Id {
								continue
							}
							p.Sync(nil, &paused, currentPlayer)
						}
						room.syncChatsToPlayerUnsafe(currentPlayer)
					}
				}()
			}
		}).ServeHTTP(c.Response(), c.Request())
		return nil
	})
}
