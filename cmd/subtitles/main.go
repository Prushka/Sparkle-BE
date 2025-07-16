package main

import (
	"Sparkle/ai"
	"Sparkle/cleanup"
	"Sparkle/config"
	"Sparkle/discord"
	"Sparkle/job"
	"Sparkle/target"
	"Sparkle/utils"
	"fmt"
	log "github.com/sirupsen/logrus"
	"os"
	"path/filepath"
	"slices"
	"strings"
)

const outputVTT = "output.%s.vtt"

func getOutputVTT() string {
	return fmt.Sprintf(outputVTT, config.TheConfig.TranslationLanguageCode)
}

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

func pipeline(j job.Job) error {
	err := os.MkdirAll(j.OutputJoin(), 0755)
	if err != nil {
		return err
	}
	discord.Infof("Extracting subtitles: %s", j.Input)
	_ = j.ExtractStreams(j.InputJoin(j.Input), job.SubtitlesType)
	err = translate(j.Input, j.OutputJoin())
	if err != nil {
		discord.Errorf("Error translating: %v", err)
		return err
	}

	destExt := fmt.Sprintf(".%s.vtt", config.TheConfig.TranslationLanguageCode)

	source := j.OutputJoin(getOutputVTT())
	dest := j.InputJoin(strings.ReplaceAll(j.Input, ".mkv", destExt))
	_, err = utils.CopyFile(source, dest)
	if err != nil {
		discord.Errorf("error copying file: %s->%s %v", source, dest, err)
		return err
	}

	discord.Infof("Done: %s", strings.ReplaceAll(j.Input, ".mkv", destExt))

	return nil
}

func processFile(file os.DirEntry, parent string, te target.ToEncode) bool {
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
