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
	log "github.com/sirupsen/logrus"
	"os"
	"path/filepath"
	"slices"
	"strings"
)

func process() {
	err := os.RemoveAll(config.TheConfig.Output)
	if err != nil {
		discord.Errorf("error removing: %v", err)
	}

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
			if _, err := os.Stat(dest); err != nil {
				return false
			}
		}
	}
	return true
}

func pipeline(j job.Job) error {
	if skip(j) {
		discord.Infof("Skipping run as all languages are processed: %s", j.Input)
		return nil
	}
	err := os.MkdirAll(j.OutputJoin(), 0755)
	if err != nil {
		return err
	}
	discord.Infof("Extracting subtitles: %s", j.Input)
	_ = j.ExtractStreams(j.InputJoin(j.Input), job.SubtitlesType)

	for _, subtitleType := range config.TheConfig.TranslationSubtitleTypes {
		for _, languageWithCode := range config.TheConfig.TranslationLanguages {
			ss := strings.Split(languageWithCode, ";")
			language := ss[0]
			languageCode := ss[1]
			dest := j.InputJoin(strings.ReplaceAll(j.Input, ".mkv",
				fmt.Sprintf(".%s.%s", languageCode, subtitleType)))

			err = translation.Translate(j.Input, j.OutputJoin(), dest, language, subtitleType)
			if err != nil {
				discord.Errorf("Error translating: %v", err)
				return err
			}

			discord.Infof("Done: %s", dest)
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
	target.UpdateEncoderList()
	process()
}
