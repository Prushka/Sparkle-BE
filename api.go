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
	"net/http"
	"os"
	"sort"
	"sync"
	"time"
)

var scheduler = gocron.NewScheduler(time.Now().Location())
var wss = make(map[string]*Room)
var e *echo.Echo
var wssMutex sync.RWMutex

const (
	NewPlayer = "new player"
)

type Room struct {
	Players map[string]*Player
	mutex   sync.RWMutex
	id      string
	Chats   []Chat `json:"chats"`
	RoomState
}

type RoomState struct {
	Time   float64
	Paused bool
}

func (room *Room) addChat(chat string, state PlayerState, id string) {
	safeTime := 0.0
	if state.Time != nil {
		safeTime = *state.Time
	}
	room.mutex.Lock()
	room.Chats = append(room.Chats, Chat{Username: state.Name, Message: chat,
		Uid:       id,
		Timestamp: time.Now().Unix(), MediaSec: safeTime})
	room.mutex.Unlock()
	room.syncChats()
	go func() { DiscordWebhook(FormatSecondsToTime(*state.Time)+": "+chat, state.Name, id) }()
}

func (room *Room) syncChats() {
	room.mutex.RLock()
	defer room.mutex.RUnlock()
	for _, player := range room.Players {
		chatsStr, err := json.Marshal(room.Chats)
		if err != nil {
			log.Error(err)
			return
		}
		player.Send(string(chatsStr))
	}
}

func (room *Room) syncChatsToPlayer(player *Player) {
	room.mutex.RLock()
	defer room.mutex.RUnlock()
	chatsStr, err := json.Marshal(room.Chats)
	if err != nil {
		log.Error(err)
		return
	}
	player.Send(string(chatsStr))
}

func defaultRoomState() RoomState {
	return RoomState{Time: 0, Paused: true}
}

func newRoom(id string) *Room {
	return &Room{Players: make(map[string]*Player), id: id,
		RoomState: defaultRoomState(), Chats: make([]Chat, 0)}
}

func (room *Room) DeletePlayer(id string) {
	room.mutex.Lock()
	defer room.mutex.Unlock()
	delete(room.Players, id)
	if len(room.Players) == 0 {
		room.RoomState = defaultRoomState()
	}
}

func (room *Room) getState() RoomState {
	room.mutex.RLock()
	defer room.mutex.RUnlock()
	return room.RoomState
}

func (room *Room) UpdatePlayer(state PlayerState, sync bool) {
	room.mutex.Lock()
	defer room.mutex.Unlock()
	if sync {
		syncTime := false
		if state.Time != nil {
			if *state.Time-room.Time > 5 || room.Time-*state.Time > 5 {
				syncTime = true
			}
			room.Time = *state.Time
		}
		syncPaused := false
		if state.Paused != nil && *state.Paused != room.Paused {
			log.Debugf("[%v] player paused: %v, room paused: %v", state.Name, *state.Paused, room.Paused)
			room.Paused = *state.Paused
			syncPaused = true
		}

		if syncTime && state.Time != nil {
			for _, p := range room.Players {
				if state.id == p.state.id {
					continue
				}
				p.Sync(&room.Time, &room.Paused, "latest player update has more than 5s difference")
			}
		} else if syncPaused {
			for _, p := range room.Players {
				if state.id == p.state.id {
					continue
				}
				p.Sync(state.Time, &room.Paused, "player has different pause state")
			}
		}
	}
}

type Player struct {
	ws    *websocket.Conn
	state *PlayerState
	mutex sync.RWMutex
}

func (player *Player) Send(message string) {
	err := websocket.Message.Send(player.ws, message)
	if err != nil {
		log.Error(err)
	}
}

type PlayerState struct {
	Time   *float64 `json:"time,omitempty"`
	Paused *bool    `json:"paused,omitempty"`
	Name   string   `json:"name,omitempty"`
	Reason string   `json:"reason,omitempty"`
	Chat   string   `json:"chat,omitempty"`
	id     string
}

func (player *Player) Sync(time *float64, paused *bool, reason string) {
	if time == player.state.Time && paused == player.state.Paused {
		return
	}
	if player.state.Name == "" {
		return
	}
	syncTo := &PlayerState{Time: time, Paused: paused, Reason: reason}
	syncToStr, err := json.Marshal(syncTo)
	if err != nil {
		log.Error(err)
		return
	}
	player.Send(string(syncToStr))
}

func REST() {
	scheduler.Every(1).Second().Do(
		func() {
			wssMutex.RLock()
			defer wssMutex.RUnlock()
			for _, room := range wss {
				playersStatusListSorted := make([]PlayerState, 0)
				room.mutex.RLock()
				for _, player := range room.Players {
					playersStatusListSorted = append(playersStatusListSorted, *player.state)
				}
				room.mutex.RUnlock()
				sort.Slice(playersStatusListSorted, func(i, j int) bool {
					return playersStatusListSorted[i].Name < playersStatusListSorted[j].Name
				})
				playersStatusListSortedStr, err := json.Marshal(playersStatusListSorted)
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
	e.Static("/static", TheConfig.Output)
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
		ctx := c.Request().Context()
		keys, err := rdb.Do(ctx, rdb.B().Keys().Pattern("job:*").Build()).ToAny()
		if err != nil {
			log.Errorf("error getting keys: %v", err)
			return c.String(http.StatusInternalServerError, err.Error())
		}
		existingJobs := make([]Job, 0)
		for _, key := range keys.([]interface{}) {
			job := Job{}
			s, err := rdb.Do(ctx, rdb.B().JsonGet().Key(key.(string)).Build()).ToString()
			if err != nil {
				log.Errorf("error getting job: %v", err)
				return c.String(http.StatusInternalServerError, err.Error())
			}
			err = json.Unmarshal([]byte(s), &job)
			if err != nil {
				log.Errorf("error getting job: %v", err)
				return c.String(http.StatusInternalServerError, err.Error())
			}
			if job.State == Complete {
				existingJobs = append(existingJobs, job)
			}
		}
		return c.JSON(http.StatusOK, existingJobs)
	})
	e.GET("/job", func(c echo.Context) error {
		name := c.QueryParam("name")
		ctx := c.Request().Context()
		err := rdb.Do(ctx, rdb.B().JsonGet().Key(name).Build()).Error()
		if err != nil {
			log.Errorf("error getting job: %v", err)
			if err.Error() == "redis nil message" {
				return c.String(http.StatusNotFound, "Job not found")
			}
			return c.String(http.StatusInternalServerError, err.Error())
		}
		return c.String(http.StatusOK, "Job: "+name)
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
					room.DeletePlayer(id)
				}
				wssMutex.Unlock()
			}(ws)
			currentPlayer := &Player{ws: ws, state: &PlayerState{
				id: id,
			}}
			wssMutex.Lock()
			if wss[room] == nil {
				wss[room] = newRoom(room)
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
				state := &PlayerState{}
				err = json.Unmarshal([]byte(msg), state)
				if err != nil {
					log.Error(err)
					return
				}
				currentPlayer.mutex.Lock()
				if state.Time != nil {
					currentPlayer.state.Time = state.Time
				}
				if state.Paused != nil {
					currentPlayer.state.Paused = state.Paused
				}
				if state.Name != "" {
					currentPlayer.state.Name = state.Name
				}
				if currentPlayer.state.Name == "" {
					currentPlayer.mutex.Unlock()
					continue
				}
				var playerState PlayerState
				j, _ := json.Marshal(currentPlayer.state)
				_ = json.Unmarshal(j, &playerState)
				currentPlayer.mutex.Unlock()

				if state.Chat != "" {
					room.addChat(state.Chat, playerState, id)
					continue
				}
				if state.Reason == NewPlayer {
					roomState := room.getState()
					currentPlayer.Sync(&roomState.Time, &roomState.Paused, "player is new")
					room.syncChatsToPlayer(currentPlayer)
				} else {
					room.UpdatePlayer(playerState, true)
				}
			}
		}).ServeHTTP(c.Response(), c.Request())
		return nil
	})
}
