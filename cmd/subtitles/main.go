package main

import (
	"Sparkle/ai"
	"Sparkle/cleanup"
	"Sparkle/config"
	"Sparkle/discord"
	"Sparkle/job"
	"Sparkle/target"
	"Sparkle/translation"
	"Sparkle/utils"
	"fmt"
	"github.com/go-co-op/gocron"
	log "github.com/sirupsen/logrus"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"
)

func process() {
	err := os.RemoveAll(config.TheConfig.Output)
	if err != nil {
		discord.Errorf("error removing: %v", err)
	}

	target.SMMutex.Lock()
	defer target.SMMutex.Unlock()
	if len(target.Shows) == 0 && len(target.Movies) == 0 {
		return
	}
	shows := make([]target.Show, 0)
	movies := make([]target.Movie, 0)

	for _, keyword := range target.Shows {
		show := target.StringToShow(keyword)
		discord.Infof(utils.AsJsonNoFormat(show))
		shows = append(shows, show)
	}
	for _, keyword := range target.Movies {
		movie := target.Movie{Name: keyword}
		discord.Infof(utils.AsJsonNoFormat(movie))
		movies = append(movies, movie)
	}

	for _, root := range config.TheConfig.ShowDirs {
		target.LoopShows(root, shows, processFile)
	}
	for _, root := range config.TheConfig.MovieDirs {
		target.LoopMovies(root, movies, processFile)
	}

	err = os.RemoveAll(config.TheConfig.Output)
	if err != nil {
		discord.Errorf("error removing: %v", err)
	}
}

func skip(j job.Job) bool {
	for _, subtitleType := range config.TheConfig.TranslationSubtitleTypes {
		for _, languageWithCode := range config.TheConfig.TranslationLanguages {
			ss := strings.Split(languageWithCode, ";")
			languageCode := ss[1]
			dest := j.InputJoin(strings.ReplaceAll(j.Input, ".mkv",
				fmt.Sprintf(".%s.%s", languageCode, subtitleType)))
			stat, err := os.Stat(dest)
			if err != nil {
				return false
			}
			statInput, err := os.Stat(j.InputJoin(j.Input))
			if err != nil {
				discord.Errorf("Error getting stat for input: %v", err)
				return false
			}
			if statInput.ModTime().After(stat.ModTime()) {
				// If the input file is newer than the subtitle file, we need to process it again
				return false
			}
		}
	}
	return true
}

func pipeline(j job.Job) error {
	if skip(j) {
		log.Debugf("Skipping: %s", j.Input)
		return nil
	}
	err := os.MkdirAll(j.OutputJoin(), 0755)
	if err != nil {
		return err
	}
	source := j.InputJoin(j.Input)
	translatable, err := job.ContainsTranslatableSubtitles(source)
	if err != nil {
		return err
	}
	if !translatable {
		return fmt.Errorf("%s doesn't contain translatable subtitle", source)
	}
	discord.Infof("Extracting subtitles: %s", source)
	err = j.ExtractStreams(source, job.SubtitlesType)
	if err != nil {
		return err
	}

	for _, subtitleType := range config.TheConfig.TranslationSubtitleTypes {
		for _, languageWithCode := range config.TheConfig.TranslationLanguages {
			languageCode := strings.Split(languageWithCode, ";")[1]
			dest := j.InputJoin(strings.ReplaceAll(j.Input, ".mkv",
				fmt.Sprintf(".%s.%s", languageCode, subtitleType)))

			err = translation.Translate(j.Input, j.OutputJoin(), source,
				dest, languageWithCode, subtitleType, false)
			if err != nil {
				discord.Errorf("Error translating: %v", err)
				return err
			}

			discord.Infof("Translated: %s", dest)
		}
	}

	return nil
}

func processFile(file os.DirEntry, parent string, _ target.ToEncode) bool {
	ext := filepath.Ext(file.Name())
	if slices.Contains(job.ValidExtensions, ext[1:]) {
		j := job.Job{
			Id:          target.NewRandomString(5),
			InputParent: parent,
			Input:       file.Name(),
		}
		err := pipeline(j)
		if err != nil {
			discord.Errorf("Failed: %v", err)
			return false
		}
		return true
	}
	return false
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
