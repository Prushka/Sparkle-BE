package main

import (
	"Sparkle/cleanup"
	"Sparkle/config"
	"Sparkle/discord"
	"Sparkle/target"
	"Sparkle/utils"
	"github.com/go-co-op/gocron"
	log "github.com/sirupsen/logrus"
	"os"
	"time"
)

func process() {
	target.SMMutex.Lock()
	defer target.SMMutex.Unlock()
	if target.ShowSet.Cardinality() == 0 && target.MovieSet.Cardinality() == 0 {
		return
	}
	shows := make([]target.Show, 0)
	movies := make([]target.Movie, 0)

	for _, keyword := range target.ShowSet.ToSlice() {
		show := target.StringToShow(keyword)
		discord.Infof(utils.AsJson(show))
		shows = append(shows, show)
	}
	for _, keyword := range target.MovieSet.ToSlice() {
		movie := target.Movie{Name: keyword}
		discord.Infof(utils.AsJson(movie))
		movies = append(movies, movie)
	}

	for _, root := range config.TheConfig.ShowDirs {
		target.LoopShows(root, shows, processFile)
	}
	for _, root := range config.TheConfig.MovieDirs {
		target.LoopMovies(root, movies, processFile)
	}
}

func processFile(file os.DirEntry, parent string, te target.ToEncode) bool {
	log.Infof("File: %v, Parent: %v, te: %v", file.Name(), parent, utils.AsJson(te))
	return true
}

func main() {
	log.SetLevel(log.InfoLevel)
	config.Configure()
	discord.Init()
	blocking := make(chan bool, 1)
	cleanup.InitSignalCallback(blocking)
	scheduler := gocron.NewScheduler(time.Now().Location())
	cleanup.AddOnStopFunc(func(_ os.Signal) {
		scheduler.Stop()
	})

	utils.PanicOnSec(scheduler.SingletonMode().Every(5).Minute().Do(func() {
		changed := target.UpdateEncoderList()
		if changed {
			process()
		}
	}))

	utils.PanicOnSec(scheduler.SingletonMode().Every(2).Hours().Do(func() {
		process()
	}))
	scheduler.StartAsync()
	<-blocking
}
