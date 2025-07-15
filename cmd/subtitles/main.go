package main

import (
	"Sparkle/cleanup"
	"Sparkle/config"
	"Sparkle/discord"
	"Sparkle/genai"
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

const outputVTT = "output.zh.vtt"

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

func translate(media, inputDir string) error {
	files, err := os.ReadDir(inputDir)
	if err != nil {
		return err
	}
	langLengths := make(map[string]int)
	langs := make(map[string]string)
	for _, file := range files {
		if strings.HasSuffix(file.Name(), ".vtt") {
			discord.Infof(file.Name())
		}
		if len(file.Name()) >= 7 {
			lang := file.Name()[len(file.Name())-7 : len(file.Name())-4]
			fBytes, err := os.ReadFile(filepath.Join(inputDir, file.Name()))
			if err != nil {
				discord.Errorf("Error reading file: %v", err)
			}
			fLines := strings.Split(string(fBytes), "\n")
			if prev, ok := langLengths[lang]; !ok || prev < len(fLines) {
				langLengths[lang] = len(fLines)
				langs[lang] = string(fBytes)
			}
		}
	}
	discord.Infof("%v", langLengths)
	if len(langs) == 0 {
		return fmt.Errorf("unable to find any webvtt")
	}
	assembled := fmt.Sprintf("Media: %s\n", media)
	count := 0
	if eng, ok := langs["eng"]; ok {
		discord.Infof("eng")
		assembled += fmt.Sprintf("Language: %s\n%s\n", "eng", eng)
		count++
	}
	for key, value := range langs {
		if count > 0 {
			break
		}
		discord.Infof(key)
		assembled += fmt.Sprintf("Language: %s\n%s\n", key, value)
		count++
	}
	translated, err := genai.TranslateSubtitles(assembled)
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(inputDir, outputVTT), []byte(translated), 0755)
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

	source := j.OutputJoin(outputVTT)
	dest := j.InputJoin(strings.ReplaceAll(j.Input, ".mkv", ".zh.vtt"))
	_, err = utils.CopyFile(source, dest)
	if err != nil {
		discord.Errorf("error copying file: %s->%s %v", source, dest, err)
		return err
	}

	discord.Infof("Done: %s", strings.ReplaceAll(j.Input, ".mkv", ".zh.vtt"))

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
	genai.InitOpenAI()
	blocking := make(chan bool, 1)
	cleanup.InitSignalCallback(blocking)
	target.UpdateEncoderList()
	process()
}
