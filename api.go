package main

import (
	"Sparkle/cleanup"
	"encoding/json"
	"github.com/go-co-op/gocron"
	"github.com/gtuk/discordwebhook"
	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
	log "github.com/sirupsen/logrus"
	"golang.org/x/net/websocket"
	"io"
	"net/http"
	"os"
	"sort"
	"time"
)

var scheduler = gocron.NewScheduler(time.Now().Location())
var wss = make(map[string]map[string]*Player)
var chats = make(map[string][]Chat)
var e *echo.Echo

const (
	NewPlayer = "new player"
)

type Chat struct {
	Username  string  `json:"username"`
	Message   string  `json:"message"`
	Timestamp int64   `json:"timestamp"`
	MediaSec  float64 `json:"mediaSec"`
}

type Player struct {
	ws    *websocket.Conn
	state *PlayerState
	id    string
}

type PlayerState struct {
	Time   *float64 `json:"time,omitempty"`
	Paused *bool    `json:"paused,omitempty"`
	Name   string   `json:"name,omitempty"`
	Reason string   `json:"reason,omitempty"`
	Chat   string   `json:"chat,omitempty"`
}

func Sync(maxTime *float64, paused *bool, player *Player, reason string) {
	if maxTime == player.state.Time && paused == player.state.Paused {
		return
	}
	if player.state.Name == "" {
		return
	}
	syncTo := &PlayerState{Time: maxTime, Paused: paused, Reason: reason}
	syncToStr, err := json.Marshal(syncTo)
	if err != nil {
		log.Error(err)
		return
	}
	err = websocket.Message.Send(player.ws, string(syncToStr))
	if err != nil {
		log.Error(err)
		return
	}
}

func SyncChats(room string) {
	for _, player := range wss[room] {
		if chats[room] == nil {
			return
		}
		c := make([]Chat, 0)
		for _, chat := range chats[room] {
			//if chat.Timestamp > time.Now().Add(-3*time.Hour).Unix() {
			c = append(c, chat)
			//}
		}
		chatsStr, err := json.Marshal(c)
		if err != nil {
			log.Error(err)
			return
		}
		err = websocket.Message.Send(player.ws, string(chatsStr))
		if err != nil {
			log.Error(err)
			return
		}
	}
}

func REST() {
	scheduler.Every(1).Second().Do(
		func() {
			for _, players := range wss {
				playersStatusListSorted := make([]PlayerState, 0)
				for _, player := range players {
					if player.state.Name == "" {
						continue
					}
					playersStatusListSorted = append(playersStatusListSorted, *player.state)
				}
				if len(playersStatusListSorted) == 0 {
					continue
				}
				sort.Slice(playersStatusListSorted, func(i, j int) bool {
					return playersStatusListSorted[i].Name < playersStatusListSorted[j].Name
				})
				playersStatusListSortedStr, err := json.Marshal(playersStatusListSorted)
				if err != nil {
					log.Error(err)
					return
				}
				for _, player := range players {
					err = websocket.Message.Send(player.ws, string(playersStatusListSortedStr))
					if err != nil {
						log.Error(err)
						return
					}
				}
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
				delete(wss[room], id)
			}(ws)
			if wss[room] == nil {
				wss[room] = make(map[string]*Player)
			}
			currentPlayer := &Player{ws: ws, state: &PlayerState{}, id: id}
			wss[room][id] = currentPlayer
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
				if state.Time != nil {
					currentPlayer.state.Time = state.Time
				}
				if state.Paused != nil {
					currentPlayer.state.Paused = state.Paused
				}
				if state.Name != "" {
					currentPlayer.state.Name = state.Name
				}
				//PrintAsJson(currentPlayer.state)
				safeTime := 0.0
				if currentPlayer.state.Time != nil {
					safeTime = *currentPlayer.state.Time
				}

				if state.Chat != "" {
					if chats[room] == nil {
						chats[room] = make([]Chat, 0)
					}
					chats[room] = append(chats[room], Chat{Username: currentPlayer.state.Name, Message: state.Chat,
						Timestamp: time.Now().Unix(), MediaSec: safeTime})
					SyncChats(room)
					content := FormatSecondsToTime(*currentPlayer.state.Time) + ": " + state.Chat
					avatarUrl := TheConfig.Host + "/static/pfp/" + id + ".png"
					message := discordwebhook.Message{
						Username:  &currentPlayer.state.Name,
						Content:   &content,
						AvatarUrl: &avatarUrl,
					}
					err := discordwebhook.SendMessage(TheConfig.DiscordWebhook, message)
					if err != nil {
						log.Errorf("error sending message to discord: %v", err)
					}
					continue
				}
				if currentPlayer.state.Name == "" {
					continue
				}
				minTime := 999999999999.0
				maxTime := 0.0
				existsPlaying := false
				existsPaused := false
				for _, player := range wss[room] {
					if player.state.Time != nil {
						if *player.state.Time < minTime {
							minTime = *player.state.Time
						}
						if *player.state.Time > maxTime {
							maxTime = *player.state.Time
						}
					}
					if player.state.Paused != nil && !*player.state.Paused {
						existsPlaying = true
					}
					if player.state.Paused != nil && *player.state.Paused {
						existsPaused = true
					}
				}
				diff := maxTime - minTime
				p := !existsPlaying
				log.Debugf("minTime: %f, maxTime: %f, diff: %f, existsPlaying: %t, existsPaused: %t", minTime, maxTime, diff, existsPlaying, existsPaused)
				if state.Reason == NewPlayer {
					Sync(&maxTime, &p, currentPlayer, "player is new")
					SyncChats(room)
				} else if diff > 5 {
					for _, player := range wss[room] {
						if player.id == id {
							continue
						}
						Sync(currentPlayer.state.Time, &p, player, "latest player update has >5s difference")
					}
				} else if diff < 5 && !(existsPaused && existsPlaying) {

				} else {
					for _, player := range wss[room] {
						if player.id == id {
							continue
						}
						Sync(state.Time, state.Paused, player, "regular sync")
					}
				}
			}
		}).ServeHTTP(c.Response(), c.Request())
		return nil
	})
}
