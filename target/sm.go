package target

import (
	"Sparkle/config"
	"Sparkle/discord"
	"Sparkle/utils"
	"encoding/json"
	"fmt"
	mapset "github.com/deckarep/golang-set/v2"
	"os"
	"regexp"
	"strconv"
	"strings"
	"sync"
)

func StringToShow(keyword string) Show {
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

type EncodeList struct {
	Shows  []string `json:"shows"`
	Movies []string `json:"movies"`
}

var ShowSet = mapset.NewSet[string]()
var MovieSet = mapset.NewSet[string]()
var SMMutex sync.Mutex

var SeasonRe = regexp.MustCompile(`Season\s+\d+`)
var SeasonEpisodeRe = regexp.MustCompile(`S\d+E(\d+)`)

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

var SessionIds = mapset.NewSet[string]()

func NewRandomString(n int) string {
	for {
		s := utils.RandomString(n)
		if !SessionIds.Contains(s) {
			SessionIds.Add(s)
			return s
		}
	}
}

func UpdateEncoderList() bool {
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
	SMMutex.Lock()
	currShows := mapset.NewSet[string](encodeList.Shows...)
	currMovies := mapset.NewSet[string](encodeList.Movies...)
	changed := false
	if !ShowSet.Equal(currShows) {
		ShowSet = currShows
		changed = true
	}
	if !MovieSet.Equal(currMovies) {
		MovieSet = currMovies
		changed = true
	}
	SMMutex.Unlock()
	if changed {
		discord.Infof("List updated: %v", encodeList)
	}
	return changed
}
