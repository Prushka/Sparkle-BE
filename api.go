package main

import (
	"Sparkle/cleanup"
	"encoding/json"
	"github.com/go-co-op/gocron"
	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
	log "github.com/sirupsen/logrus"
	"golang.org/x/net/websocket"
	"net/http"
	"os"
	"sort"
	"time"
)

var scheduler = gocron.NewScheduler(time.Now().Location())

var wss = make(map[string]map[string]*Player)

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

func REST() {
	scheduler.Every(2).Second().Do(
		func() {
			for _, players := range wss {
				playersStatusListSorted := make([]PlayerState, 0)
				for _, player := range players {
					if player.state.Name == "" {
						continue
					}
					playersStatusListSorted = append(playersStatusListSorted, *player.state)
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
	e := echo.New()
	e.Use(middleware.CORSWithConfig(middleware.CORSConfig{
		AllowOrigins:     []string{"*"},
		AllowCredentials: true,
	}), middleware.GzipWithConfig(middleware.DefaultGzipConfig), middleware.Logger(), middleware.Recover())
	e.Static("/static", OUTPUT)
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
			log.Infof("job: %s", s)
			err = json.Unmarshal([]byte(s), &job)
			if err != nil {
				log.Errorf("error getting job: %v", err)
				return c.String(http.StatusInternalServerError, err.Error())
			}
			if job.State == "complete" {
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

	e.GET("/sync/:room", func(c echo.Context) error {
		room := c.Param("room")
		id := RandomString(36)
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
				rdb.Do(c.Request().Context(), rdb.B().Publish().Channel(room).Message(msg).Build())
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
				PrintAsJson(currentPlayer.state)

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
				log.Infof("minTime: %f, maxTime: %f, diff: %f, existsPlaying: %t, existsPaused: %t", minTime, maxTime, diff, existsPlaying, existsPaused)
				if currentPlayer.state.Paused == nil { // new player
					Sync(&maxTime, &p, currentPlayer, "player is new")
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
	cleanup.AddOnStopFunc(cleanup.Echo, func(_ os.Signal) {
		err := e.Close()
		if err != nil {
			return
		}
	})
	e.Logger.Fatal(e.Start(":1323"))
}
