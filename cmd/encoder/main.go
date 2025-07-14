package main

import (
	"Sparkle/cleanup"
	"Sparkle/config"
	"Sparkle/discord"
	"Sparkle/job"
	"Sparkle/utils"
	"encoding/json"
	"fmt"
	mapset "github.com/deckarep/golang-set/v2"
	"github.com/go-co-op/gocron"
	log "github.com/sirupsen/logrus"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"sync"
	"time"
)

func processFile(file os.DirEntry, parent string, te ToEncode) bool {
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
					(j.Fast == te.Fast) {
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
			Id:          newRandomString(5),
			InputParent: parent,
			Input:       file.Name(),
			OriSize:     stats.Size(),
			OriModTime:  stats.ModTime().Unix(),
			Fast:        te.Fast,
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
		if j.State == job.Complete && config.TheConfig.RemoveOnSuccess {
			err = os.Remove(j.InputJoin(j.InputAfterRename()))
			if err != nil {
				discord.Errorf("error removing file: %v", err)
			}
			return true
		} else if config.TheConfig.EnableRename {
			err = os.Rename(j.InputJoin(j.InputAfterRename()), j.InputJoin(j.Input))
			if err != nil {
				discord.Errorf("error renaming file: %v", err)
			}
			return false
		}
	}
	return false
}

func encode(matches func(s string) bool, te ToEncode) error {
	files, err := os.ReadDir(config.TheConfig.Input)
	if err != nil {
		return err
	}
	for _, file := range files {
		if file.IsDir() {
			fs, err := os.ReadDir(utils.InputJoin(file.Name()))
			if err != nil {
				return err
			}
			for _, f := range fs {
				if matches == nil || matches(f.Name()) {
					if processFile(f, file.Name(), te) && config.TheConfig.RemoveOnSuccess {
						err = os.RemoveAll(utils.InputJoin(file.Name()))
					}
				}
			}
		} else {
			if matches == nil || matches(file.Name()) {
				processFile(file, "", te)
			}
		}
	}
	return nil
}

func newRandomString(n int) string {
	for {
		s := utils.RandomString(n)
		if !sessionIds.Contains(s) {
			sessionIds.Add(s)
			return s
		}
	}
}

func stringToShow(keyword string) Show {
	s := strings.Split(keyword, ",")
	showName := s[0]
	seasons := make(map[string]Season)
	if len(s) > 1 {
		for i := 1; i < len(s); i++ {
			ss := strings.Split(s[i], "|")
			var startEpisode *int
			seasonName := s[i]
			if len(ss) > 1 {
				se, _ := strconv.Atoi(ss[1])
				startEpisode = &se
				seasonName = ss[0]
			}
			if strings.ToLower(s[i]) == "specials" {
				seasons["Specials"] = Season{Name: "Specials", StartEpisode: startEpisode}
			} else {
				name := fmt.Sprintf("Season %s", seasonName)
				seasons[name] = Season{Name: name, StartEpisode: startEpisode}
			}
		}
	}
	return Show{Name: showName, Seasons: seasons}
}

func encodeShows(root string, shows []Show) {
	files, err := os.ReadDir(root)
	if err != nil {
		discord.Errorf("error reading directory: %v", err)
		return
	}
	for _, file := range files {
		if file.IsDir() {
			for _, show := range shows {
				if strings.Contains(strings.ToLower(file.Name()), strings.ToLower(show.Name)) {
					fs, err := os.ReadDir(filepath.Join(root, file.Name()))
					if err != nil {
						discord.Errorf("error reading directory: %v", err)
						return
					}
					for _, f := range fs {
						p := func(matches func(s string) bool) {
							root := filepath.Join(root, file.Name(), f.Name())
							discord.Infof("Scanning %s", root)
							config.TheConfig.Input = root
							err := encode(matches, show.ToEncode)
							if err != nil {
								discord.Errorf("error: %v", err)
							}
						}
						if f.IsDir() && (re.MatchString(f.Name()) || f.Name() == "Specials") {
							if len(show.Seasons) > 0 {
								if season, ok := show.Seasons[f.Name()]; ok {
									if season.StartEpisode == nil {
										p(nil)
									} else {
										p(func(s string) bool {
											match := episodeRe.FindStringSubmatch(s)
											if match != nil && len(match) > 1 {
												currentEpisode, err := strconv.Atoi(match[1])
												if err != nil {
													return false
												}
												if currentEpisode >= *season.StartEpisode {
													return true
												}
											} else {
												discord.Infof("No episode number found")
											}
											return false
										})
									}
								}
							} else {
								p(nil)
							}
						}
					}
				}
			}
		}
	}
}

func encodeMovies(root string, movies []Movie) {
	files, err := os.ReadDir(root)
	if err != nil {
		discord.Errorf("error reading directory: %v", err)
		return
	}
	for _, file := range files {
		if file.IsDir() {
			for _, movie := range movies {
				if strings.Contains(strings.ToLower(file.Name()), strings.ToLower(movie.Name)) {
					root := filepath.Join(root, file.Name())
					discord.Infof("Processing %s", root)
					config.TheConfig.Input = root
					err = encode(nil, movie.ToEncode)
					if err != nil {
						discord.Errorf("error: %v", err)
					}
				}
			}
		}
	}
}

type EncodeList struct {
	Shows  []string `json:"shows"`
	Movies []string `json:"movies"`
}

var showSet = mapset.NewSet[string]()
var movieSet = mapset.NewSet[string]()
var smMutex sync.Mutex

var re = regexp.MustCompile(`Season\s+\d+`)
var episodeRe = regexp.MustCompile(`S\d+E(\d+)`)

type ToEncode struct {
	Fast bool
}

type Movie struct {
	Name string
	ToEncode
}

type Show struct {
	Name    string
	Seasons map[string]Season
	ToEncode
}

type Season struct {
	Name         string
	StartEpisode *int
}

var sessionIds = mapset.NewSet[string]()

func isFast(keyword string) (bool, string) {
	if strings.HasPrefix(keyword, "f:") {
		return true, keyword[2:]
	}
	return false, keyword
}

var totalProcessed = 0

func process() {
	totalProcessed = 0
	smMutex.Lock()
	defer smMutex.Unlock()
	if showSet.Cardinality() == 0 && movieSet.Cardinality() == 0 {
		return
	}
	shows := make([]Show, 0)
	movies := make([]Movie, 0)
	sessionIds.Clear()
	jobs, err := job.JobsCache.Get(true)
	if err != nil {
		discord.Errorf("error getting all jobs: %v", err)
		return
	}
	for _, j := range jobs {
		sessionIds.Add(j.Id)
	}
	for _, keyword := range showSet.ToSlice() {
		isFast, keyword := isFast(keyword)
		show := stringToShow(keyword)
		show.Fast = isFast
		discord.Infof(utils.AsJson(show))
		shows = append(shows, show)
	}
	for _, keyword := range movieSet.ToSlice() {
		isFast, keyword := isFast(keyword)
		movie := Movie{Name: keyword}
		movie.Fast = isFast
		discord.Infof(utils.AsJson(movie))
		movies = append(movies, movie)
	}
	for _, root := range config.TheConfig.ShowDirs {
		encodeShows(root, shows)
	}
	for _, root := range config.TheConfig.MovieDirs {
		encodeMovies(root, movies)
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
	blocking := make(chan bool, 1)
	cleanup.InitSignalCallback(blocking)
	scheduler := gocron.NewScheduler(time.Now().Location())
	cleanup.AddOnStopFunc(func(_ os.Signal) {
		scheduler.Stop()
	})
	utils.PanicOnSec(scheduler.SingletonMode().Every(5).Minute().Do(func() {
		encodeList := EncodeList{}
		encodeListFile := config.TheConfig.EncodeListFile
		if _, err := os.Stat(encodeListFile); err == nil {
			content, err := os.ReadFile(encodeListFile)
			if err != nil {
				discord.Errorf("error reading file: %v", err)
			}
			err = json.Unmarshal(content, &encodeList)
			if err != nil {
				discord.Errorf("error unmarshalling file: %v", err)
			}
		}
		smMutex.Lock()
		currShows := mapset.NewSet[string](encodeList.Shows...)
		currMovies := mapset.NewSet[string](encodeList.Movies...)
		changed := false
		if !showSet.Equal(currShows) {
			showSet = currShows
			changed = true
		}
		if !movieSet.Equal(currMovies) {
			movieSet = currMovies
			changed = true
		}
		smMutex.Unlock()
		if changed {
			discord.Infof("List updated: %v", encodeList)
			process()
		}
	}))

	utils.PanicOnSec(scheduler.SingletonMode().Every(2).Hours().Do(func() {
		process()
	}))
	scheduler.StartAsync()
	<-blocking
}
