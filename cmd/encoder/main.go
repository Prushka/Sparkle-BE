package main

import (
	"Sparkle/ai"
	"Sparkle/cleanup"
	"Sparkle/config"
	"Sparkle/discord"
	"Sparkle/job"
	"Sparkle/target"
	"Sparkle/utils"
	"github.com/go-co-op/gocron"
	log "github.com/sirupsen/logrus"
	"net/http"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"
)

func processFile(file os.DirEntry, parent string, te target.ToEncode) bool {
	ext := filepath.Ext(file.Name())
	if slices.Contains(job.ValidExtensions, ext[1:]) {
		jobs, err := job.JobsCache.Get(false)
		if err != nil {
			discord.Errorf("error getting all jobs: %v", err)
			return false
		}
		stats, err := file.Info()
		if err != nil {
			discord.Errorf("error getting file info: %v", err)
			return false
		}
		currId := utils.GetTitleId(file.Name())
		log.Debugf("Current ID: %s", currId)
		for _, j := range jobs {
			prevId := utils.GetTitleId(j.Input)
			if currId == prevId {
				log.Debugf("File exists: %s", file.Name())
				if j.State == job.Complete && len(j.EncodedCodecs) > 0 &&
					(j.OriSize == 0 || j.OriSize == stats.Size()) &&
					(j.Fast == te.Fast) && (j.Translate == te.Translate) {
					return false
				} else {
					discord.Infof("File modified or prev encoding incomplete: %s, remove old", file.Name())
					err := os.RemoveAll(utils.OutputJoin(j.Id))
					if err != nil {
						discord.Errorf("error removing file: %v", err)
					}
				}
			}
		}
		j := job.Job{
			Id:          target.NewRandomString(5),
			InputParent: parent,
			Input:       file.Name(),
			OriSize:     stats.Size(),
			OriModTime:  stats.ModTime().Unix(),
			Fast:        te.Fast,
			Translate:   te.Translate,
		}
		startTime := time.Now()
		discord.Infof("Processing file: %s", file.Name())
		err = j.Pipeline()
		if err != nil {
			discord.Errorf("error processing file: %v", err)
		} else {
			totalProcessed++
		}
		discord.Infof("Processed %s, time cost: %s", file.Name(), time.Since(startTime))
		if j.State == job.Complete {
			return true
		}
	}
	return false
}

func parseExtraParams(keyword string) (target.ToEncode, string) {
	te := target.ToEncode{
		Fast:      false,
		Translate: false,
	}
	if strings.HasPrefix(keyword, "f:") {
		keyword = keyword[2:]
		te.Fast = true
	}
	if strings.HasSuffix(keyword, ":t") {
		keyword = keyword[:len(keyword)-2]
		te.Translate = true
	}
	return te, keyword
}

var totalProcessed = 0

func process() {
	totalProcessed = 0
	target.SMMutex.Lock()
	defer target.SMMutex.Unlock()
	if len(target.Shows) == 0 && len(target.Movies) == 0 {
		return
	}
	shows := make([]target.Show, 0)
	movies := make([]target.Movie, 0)
	target.SessionIds.Clear()
	jobs, err := job.JobsCache.Get(true)
	if err != nil {
		discord.Errorf("error getting all jobs: %v", err)
		return
	}
	for _, j := range jobs {
		target.SessionIds.Add(j.Id)
	}
	for _, keyword := range target.Shows {
		te, keyword := parseExtraParams(keyword)
		show := target.StringToShow(keyword)
		show.ToEncode = te
		discord.Infof(utils.AsJsonNoFormat(show))
		shows = append(shows, show)
	}
	for _, keyword := range target.Movies {
		te, keyword := parseExtraParams(keyword)
		movie := target.Movie{Name: keyword}
		movie.ToEncode = te
		discord.Infof(utils.AsJsonNoFormat(movie))
		movies = append(movies, movie)
	}
	for _, root := range config.TheConfig.ShowDirs {
		target.LoopShows(root, shows, processFile)
	}
	for _, root := range config.TheConfig.MovieDirs {
		target.LoopMovies(root, movies, processFile)
	}
	discord.Infof("Total processed: %d", totalProcessed)
	totalDeleted := 0
	if config.TheConfig.EnableCleanup {
		discord.Infof("Cleaning up old files")
		jobs, err := job.JobsCache.Get(false)
		if err != nil {
			discord.Errorf("error getting all jobs: %v", err)
			return
		}
		for _, j := range jobs {
			markedForRemoval := true
			for _, show := range shows {
				if strings.Contains(strings.ToLower(j.Input), strings.ToLower(show.Name)) {
					markedForRemoval = false
				}
			}
			for _, movie := range movies {
				if strings.Contains(strings.ToLower(j.Input), strings.ToLower(movie.Name)) {
					markedForRemoval = false
				}
			}
			if j.State != job.Complete {
				markedForRemoval = true
			}
			if markedForRemoval {
				discord.Infof("File: %s, remove old, %s", utils.OutputJoin(j.Id), j.Input)
				err := os.RemoveAll(utils.OutputJoin(j.Id))
				if err != nil {
					discord.Errorf("error removing file: %v", err)
				} else {
					totalDeleted++
				}
			}
		}
		discord.Infof("Total deleted: %d", totalDeleted)
	}

	if (totalProcessed > 0 || totalDeleted > 0) && len(config.TheConfig.PurgeCacheUrl) > 0 {
		_, err := http.Get(config.TheConfig.PurgeCacheUrl)
		if err != nil {
			discord.Errorf("error purging cache: %v", err)
		}
	}
}

func main() {
	log.SetLevel(log.InfoLevel)
	config.Configure()
	discord.Init()
	ai.Init()
	blocking := make(chan bool, 1)
	cleanup.InitSignalCallback(blocking)
	scheduler := gocron.NewScheduler(time.Now().Location())
	cleanup.AddOnStopFunc(func(_ os.Signal) {
		scheduler.Stop()
	})
	utils.PanicOnSec(scheduler.SingletonMode().Every(config.TheConfig.ScanConfigInterval).Do(func() {
		changed := target.UpdateEncoderList()
		if changed {
			process()
		}
	}))
	utils.PanicOnSec(scheduler.SingletonMode().Every(config.TheConfig.ScanInputInterval).Do(func() {
		process()
	}))
	scheduler.StartAsync()
	<-blocking
}
