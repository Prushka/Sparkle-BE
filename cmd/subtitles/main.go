package main

import (
	"Sparkle/cleanup"
	"Sparkle/config"
	"Sparkle/discord"
	"Sparkle/job"
	"Sparkle/target"
	"Sparkle/utils"
	"context"
	"fmt"
	"github.com/go-co-op/gocron"
	"github.com/openai/openai-go"
	"github.com/openai/openai-go/option"
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
		discord.Errorf("error removing file: %v", err)
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
			log.Infof(file.Name())
		}
		if len(file.Name()) >= 7 {
			lang := file.Name()[len(file.Name())-7 : len(file.Name())-4]
			fBytes, err := os.ReadFile(filepath.Join(inputDir, file.Name()))
			if err != nil {
				log.Errorf("Error reading file: %v", err)
			}
			fLines := strings.Split(string(fBytes), "\n")
			if prev, ok := langLengths[lang]; !ok || prev < len(fLines) {
				langLengths[lang] = len(fLines)
				langs[lang] = string(fBytes)
			}
		}
	}
	log.Infof("%v", langLengths)
	if len(langs) == 0 {
		return fmt.Errorf("unable to find any webvtt")
	}
	assembled := fmt.Sprintf("Media: %s\n", media)
	count := 0
	for key, value := range langs {
		log.Infof(key)
		assembled += fmt.Sprintf("Language: %s\n%s\n", key, value)
		count++
		if count > 1 {
			break
		}
	}

	log.Infof("Sending to ChatGPT: %s", assembled)

	ctx := context.Background()
	msgs := []openai.ChatCompletionMessageParamUnion{
		openai.SystemMessage(`Role: You are an intelligent WEBVTT subtitle translator.
Input: WEBVTT(s) containing subtitles in one or two non‑Chinese languages.
Task:
1. Preserve every original timing cue exactly.
2. Replace each subtitle line with a context‑aware Chinese translation.
Output: A single, valid WEBVTT and nothing else, formatted identically to the input except that all subtitle text is now in Chinese.`),
		openai.UserMessage("WEBVTT\n\n00:02.044 --> 00:05.089\n(pompöse Orchestermusik)"),
	}
	resp, err := openaiCli.Chat.Completions.New(ctx, openai.ChatCompletionNewParams{
		Model:    "o3-mini",
		Messages: msgs,
	})
	if err != nil {
		return err
	}

	log.Infof("Response: %s", resp.Choices[0].Message.Content)
	fmt.Printf("%v\n", utils.AsJson(resp))
	return nil
}

func pipeline(j job.Job) error {
	err := os.MkdirAll(j.OutputJoin(), 0755)
	if err != nil {
		return err
	}
	log.Infof("Extracting subtitles: %s", j.Input)
	j.ExtractStreams(j.InputJoin(j.Input), job.SubtitlesType)

	return translate(j.Input, j.OutputJoin())
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
			log.Errorf("Failed: %v", err)
			return false
		}
		return true
	}
	return false
}

var openaiCli openai.Client

func initOpenAI() {
	openaiCli = openai.NewClient(
		option.WithAPIKey(config.TheConfig.OpenAI),
	)
}

func main() {
	log.SetLevel(log.InfoLevel)
	config.Configure()
	discord.Init()
	initOpenAI()
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
